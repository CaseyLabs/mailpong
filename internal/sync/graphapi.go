package sync

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/caseylabs/mailven/internal/credentials"
	"github.com/caseylabs/mailven/internal/graphapi"
	"github.com/caseylabs/mailven/internal/message"
	"github.com/caseylabs/mailven/internal/oauth2"
)

// Graph receive constants. Sources are recorded in PLAN.md Surprises.
const (
	graphCursorKind   = "graph-delta"
	graphFoldersScope = "folders"
	graphFoldersKind  = "graph-folders"
	graphBatchSize    = 50 // messages fetched and committed per page
	graphProvider     = message.APIProviderGraph
)

func graphFolderScope(folderID string) string { return "folder:" + folderID }

// GraphSync mirrors one Microsoft 365 mailbox over Graph into the local store,
// one delta cursor per folder. It is inert: nothing schedules it, and it only
// runs against staged routes. A shared mailbox is read when Reader.Shared is
// set, which also requires the Shared grant.
type GraphSync struct {
	Engine      *Engine
	Reader      *graphapi.Reader
	AccountID   string
	Tokens      func() (*credentials.OAuthTokens, error)
	Concurrency int // 0 uses graphapi.MaxConcurrency
	// Sleep waits between rate-limit retries; nil sleeps for real.
	Sleep func(ctx context.Context, d time.Duration) error
	// OnFailure receives authorization failures (the F4b reauthorization
	// event). Rate limits never reach it and credentials are never cleared.
	OnFailure func(oauth2.MailAPIFailure, error)

	folderCache *apiFolders
}

// GraphResult summarizes one run.
type GraphResult struct {
	Reconciled int // folders that ran a full reconcile (first sync or recovery)
	Added      int
	Updated    int
	Removed    int
}

// Run syncs every mapped folder once: incrementally from its stored delta
// link, or with a full reconcile when it has none, it is invalid, or Graph
// answered 410. Renamed folders reconcile; vanished folders are cleaned up.
func (g *GraphSync) Run(ctx context.Context) (res GraphResult, err error) {
	defer func() {
		var re *graphapi.ReadError
		if err != nil && errors.As(err, &re) && re.Failure != oauth2.MailAPIFailureNone && g.OnFailure != nil {
			g.OnFailure(re.Failure, err)
		}
	}()
	tokens, err := g.Tokens()
	if err != nil {
		return res, &graphapi.ReadError{Failure: oauth2.ClassifyMailAPIError(err), Err: err}
	}
	if err = tokens.RequireMailAPICapability("read", g.Reader.Shared != ""); err != nil {
		return res, &graphapi.ReadError{Failure: oauth2.MailAPIFailurePartialConsent, Err: err}
	}
	if g.folderCache == nil {
		g.folderCache = &apiFolders{engine: g.Engine, accountID: g.AccountID}
	}
	store := g.Engine.messageStore
	v, err := g.backoff(ctx, func() (any, error) {
		root, fs, e := g.Reader.Folders(ctx)
		return folderListing{root, fs}, e
	})
	if err != nil {
		return res, err
	}
	listing := v.(folderListing)
	var gf []message.GraphFolder
	for _, f := range listing.folders {
		gf = append(gf, message.GraphFolder{ID: f.ID, Name: f.Name, ParentID: f.ParentID, WellKnown: f.WellKnown})
	}
	fm := message.MapGraphFolders(listing.root, gf)
	// The folder listing can contain empty folders; materialize it independently
	// of delta message ingestion so actions can still target those destinations.
	for _, f := range fm.Folders {
		if _, err := g.folderCache.ensure(f); err != nil {
			return res, err
		}
	}

	fcur, err := store.LoadAPICursor(g.AccountID, graphFoldersScope)
	if err != nil {
		return res, err
	}
	old := map[string]string{} // folder ID -> path at the last complete run
	if fcur.Cursor != "" {
		if err := json.Unmarshal([]byte(fcur.Cursor), &old); err != nil {
			old = map[string]string{}
		}
	}
	ids := make([]string, 0, len(fm.Folders))
	for id := range fm.Folders {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		if prev, ok := old[id]; ok && prev != fm.Folders[id].Path {
			// A rename moves every message to a new local folder: reconcile it.
			if err := store.InvalidateAPICursor(g.AccountID, graphFolderScope(id)); err != nil {
				return res, err
			}
		}
	}
	current := map[string]string{}
	for _, id := range ids {
		err := g.syncFolder(ctx, id, fm, &res)
		if errors.Is(err, graphapi.ErrFolderGone) {
			continue // deleted since the listing; dropFolder cleans it up below
		}
		if err != nil {
			return res, err
		}
		current[id] = fm.Folders[id].Path
	}
	for id := range old {
		if _, ok := current[id]; !ok {
			if err := g.dropFolder(ctx, id, fm, &res); err != nil {
				return res, err
			}
		}
	}
	g.removeStaleFolders(old, current)
	b, err := json.Marshal(current)
	if err != nil {
		return res, err
	}
	return res, store.CommitAPIPage(message.APIPage{AccountID: g.AccountID, Provider: graphProvider, Scope: graphFoldersScope,
		Generation: fcur.Generation, Cursor: string(b), CursorKind: graphFoldersKind, ClearInvalid: true})
}

type folderListing struct {
	root    string
	folders []graphapi.Folder
}

// syncFolder runs one folder's delta, falling back to a reconcile on 410.
func (g *GraphSync) syncFolder(ctx context.Context, folderID string, fm message.GraphFolderMap, res *GraphResult) error {
	store := g.Engine.messageStore
	scope := graphFolderScope(folderID)
	cur, err := store.LoadAPICursor(g.AccountID, scope)
	if err != nil {
		return err
	}
	if cur.Kind == graphCursorKind && cur.Cursor != "" && !cur.Invalid {
		err = g.deltaRun(ctx, folderID, scope, cur, cur.Cursor, fm, res)
		if !errors.Is(err, graphapi.ErrDeltaExpired) {
			return err
		}
		if err = store.InvalidateAPICursor(g.AccountID, scope); err != nil {
			return err
		}
		if cur, err = store.LoadAPICursor(g.AccountID, scope); err != nil {
			return err
		}
	}
	res.Reconciled++
	return g.deltaRun(ctx, folderID, scope, cur, "", fm, res)
}

// deltaRun walks a folder's delta from start ("" = a fresh enumeration, which
// is a reconcile). Each page commits with the old cursor, so an interrupted
// run repeats idempotent work; only the final commit stores the new delta
// link. A reconcile also re-checks local messages the enumeration did not list:
// they are removed unless Graph still has them elsewhere (a move).
func (g *GraphSync) deltaRun(ctx context.Context, folderID, scope string, cur message.APICursor, start string,
	fm message.GraphFolderMap, res *GraphResult) error {
	reconcile := start == ""
	seen := map[string]bool{}
	link := start
	var final string
	for final == "" {
		v, err := g.backoff(ctx, func() (any, error) { return g.Reader.Delta(ctx, folderID, link) })
		if err != nil {
			return err
		}
		page := v.(*graphapi.DeltaPage)
		items := page.Items
		for _, it := range items {
			if !it.Removed {
				seen[it.ID] = true
			}
		}
		if err := g.commitBatches(ctx, scope, cur, cur.Cursor, cur.Kind, items, fm, res); err != nil {
			return err
		}
		if final = page.DeltaLink; final == "" {
			if link = page.NextLink; link == "" {
				return errors.New("graph delta ended without a next or delta link")
			}
		}
	}
	if reconcile {
		local, err := g.Engine.messageStore.ListAPIRemoteIDsInContainer(g.AccountID, graphProvider, folderID)
		if err != nil {
			return err
		}
		var missing []graphapi.Meta
		for _, id := range local {
			if !seen[id] {
				missing = append(missing, graphapi.Meta{ID: id, Removed: true})
			}
		}
		if err := g.commitBatches(ctx, scope, cur, cur.Cursor, cur.Kind, missing, fm, res); err != nil {
			return err
		}
	}
	return g.Engine.messageStore.CommitAPIPage(message.APIPage{AccountID: g.AccountID, Provider: graphProvider, Scope: scope,
		Generation: cur.Generation, Cursor: final, CursorKind: graphCursorKind, ClearInvalid: true})
}

// commitBatches processes items in pages of graphBatchSize, one transaction each.
func (g *GraphSync) commitBatches(ctx context.Context, scope string, cur message.APICursor, cursor, kind string,
	items []graphapi.Meta, fm message.GraphFolderMap, res *GraphResult) error {
	workers := g.Concurrency
	if workers < 1 {
		workers = graphapi.MaxConcurrency
	}
	for chunk := range slices.Chunk(items, graphBatchSize) {
		type outcome struct {
			up       *message.APIUpsert
			gone     bool
			existing bool
		}
		outs, err := apiFanOut(ctx, workers, chunk, func(ctx context.Context, m graphapi.Meta) (outcome, error) {
			up, gone, existing, e := g.process(ctx, m, fm)
			return outcome{up, gone, existing}, e
		})
		if err != nil {
			return err
		}
		page := message.APIPage{AccountID: g.AccountID, Provider: graphProvider, Scope: scope,
			Generation: cur.Generation, Cursor: cursor, CursorKind: kind}
		for i, o := range outs {
			switch {
			case o.gone:
				page.Removals = append(page.Removals, chunk[i].ID)
				res.Removed++
			case o.up != nil:
				page.Upserts = append(page.Upserts, *o.up)
				if o.existing {
					res.Updated++
				} else {
					res.Added++
				}
			}
		}
		if err := g.Engine.messageStore.CommitAPIPage(page); err != nil {
			return err
		}
	}
	return nil
}

// process builds the upsert for one delta entry: nothing when it is unchanged,
// a flags-only refresh when only the change key moved, and otherwise a full
// ingest from the raw message. A removed entry is verified first, since a
// message moved to another folder leaves this folder's delta as removed.
func (g *GraphSync) process(ctx context.Context, meta graphapi.Meta, fm message.GraphFolderMap) (up *message.APIUpsert, gone, existing bool, err error) {
	store := g.Engine.messageStore
	ref, rows, err := store.GetAPIRef(g.AccountID, graphProvider, meta.ID)
	if err != nil {
		return nil, false, false, err
	}
	existing = ref != nil
	if meta.Removed {
		v, err := g.backoff(ctx, func() (any, error) { return g.Reader.GetMeta(ctx, meta.ID) })
		if errors.Is(err, graphapi.ErrMessageGone) {
			return nil, existing, existing, nil
		}
		if err != nil {
			return nil, false, existing, err
		}
		meta = *v.(*graphapi.Meta)
	}
	mapping := fm.MapGraphMessage(meta.ParentFolderID)
	newRef := message.APIRef{RemoteID: meta.ID, RemoteThreadID: meta.ConversationID, Containers: []string{meta.ParentFolderID},
		Unmapped: mapping.Unmapped, Version: meta.ChangeKey}
	folderIDs := make([]string, 0, len(mapping.Folders))
	for _, f := range mapping.Folders {
		id, err := g.folderCache.ensure(f)
		if err != nil {
			return nil, false, existing, err
		}
		folderIDs = append(folderIDs, id)
	}
	if existing && len(rows) == len(folderIDs) && !slices.ContainsFunc(folderIDs, func(id string) bool { _, ok := rows[id]; return !ok }) {
		if ref.Version == meta.ChangeKey && slices.Equal(ref.Containers, newRef.Containers) {
			return nil, false, true, nil
		}
		if !meta.IsDraft { // only drafts change content in place; everything else is flags/categories
			up := &message.APIUpsert{Ref: newRef}
			for _, id := range folderIDs {
				up.Rows = append(up.Rows, message.APIRow{FlagsOnly: true, Message: message.Message{
					FolderID: id, IsRead: meta.IsRead, IsStarred: meta.Flagged, IsDraft: meta.IsDraft}})
			}
			return up, false, true, nil
		}
	}
	if len(folderIDs) == 0 { // moved into a folder with no mapping: keep the identity, drop the rows
		return &message.APIUpsert{Ref: newRef}, false, existing, nil
	}
	v, err := g.backoff(ctx, func() (any, error) { return g.Reader.GetRaw(ctx, meta.ID) })
	if errors.Is(err, graphapi.ErrMessageGone) {
		return nil, existing, existing, nil
	}
	if err != nil {
		return nil, false, existing, err
	}
	m, extras := g.Engine.parseAPIRaw(g.AccountID, meta.ID, "graph.invalid", v.([]byte))
	m.IsRead, m.IsStarred, m.IsDraft = meta.IsRead, meta.Flagged, meta.IsDraft
	out := &message.APIUpsert{Ref: newRef}
	for _, id := range folderIDs {
		row := m
		row.FolderID = id
		out.Rows = append(out.Rows, message.APIRow{Message: row, Extras: extras})
	}
	return out, false, existing, nil
}

// dropFolder reconciles a folder that no longer exists: each message it held
// is re-checked, so one moved elsewhere keeps its identity and the rest are
// removed. Local-only messages without an API ref are never touched.
func (g *GraphSync) dropFolder(ctx context.Context, folderID string, fm message.GraphFolderMap, res *GraphResult) error {
	store := g.Engine.messageStore
	scope := graphFolderScope(folderID)
	cur, err := store.LoadAPICursor(g.AccountID, scope)
	if err != nil {
		return err
	}
	local, err := store.ListAPIRemoteIDsInContainer(g.AccountID, graphProvider, folderID)
	if err != nil {
		return err
	}
	var items []graphapi.Meta
	for _, id := range local {
		items = append(items, graphapi.Meta{ID: id, Removed: true})
	}
	if err := g.commitBatches(ctx, scope, cur, "", "", items, fm, res); err != nil {
		return err
	}
	return store.CommitAPIPage(message.APIPage{AccountID: g.AccountID, Provider: graphProvider, Scope: scope,
		Generation: cur.Generation, ClearInvalid: true})
}

// removeStaleFolders deletes local folders whose remote folder was renamed or
// deleted, once they hold no messages; a folder with local-only mail stays.
func (g *GraphSync) removeStaleFolders(old, current map[string]string) {
	live := map[string]bool{}
	for _, p := range current {
		live[p] = true
	}
	for _, path := range old {
		if live[path] {
			continue
		}
		f, err := g.Engine.folderStore.GetByPath(g.AccountID, path)
		if err != nil || f == nil {
			continue
		}
		if n, err := g.Engine.messageStore.CountFolderMessages(f.ID); err != nil || n > 0 {
			continue
		}
		if err := g.Engine.folderStore.Delete(f.ID); err != nil {
			g.Engine.log.Debug().Err(err).Str("path", path).Msg("could not remove stale Graph folder")
			continue
		}
		g.folderCache.forget(path)
	}
}

// backoff retries Graph rate limits (see apiBackoff).
func (g *GraphSync) backoff(ctx context.Context, do func() (any, error)) (any, error) {
	return apiBackoff(ctx, g.Sleep, func(err error) (bool, time.Duration) {
		var re *graphapi.ReadError
		if errors.As(err, &re) && re.RateLimited {
			return true, re.RetryAfter
		}
		return false, 0
	}, do)
}
