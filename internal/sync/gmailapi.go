package sync

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/caseylabs/mailven/internal/credentials"
	"github.com/caseylabs/mailven/internal/gmailapi"
	"github.com/caseylabs/mailven/internal/message"
	"github.com/caseylabs/mailven/internal/oauth2"
)

// Gmail receive constants. Source rationale is in docs/MAIL_APIS.md,
// "Provider limit source rationale".
const (
	gmailScope           = "mailbox"
	gmailCursorKind      = "gmail-history"
	gmailBatchSize       = 50 // messages fetched and committed per page
	gmailDefaultWorkers  = 4  // raw fetches in flight; Gmail's per-user limit is 250 quota units/s, a get costs 5
	gmailAPIProviderName = message.APIProviderGmail
)

// GmailSync mirrors one Gmail account over the API into the local store. It
// is scheduled by app.runAPISync for active routes. Callers must serialize Run
// calls per account; GmailSync has no internal mutex.
type GmailSync struct {
	Engine      *Engine
	Reader      *gmailapi.Reader
	AccountID   string
	Tokens      func() (*credentials.OAuthTokens, error)
	Concurrency int
	// Sleep waits between rate-limit retries; nil sleeps for real.
	Sleep func(ctx context.Context, d time.Duration) error
	// OnFailure receives authorization failures (the F4b reauthorization
	// event). Rate limits never reach it and credentials are never cleared.
	OnFailure func(oauth2.MailAPIFailure, error)

	folderCache *apiFolders
}

// GmailResult summarizes one run.
type GmailResult struct {
	Reconciled bool // a full reconcile ran (first sync or recovery)
	Added      int
	Updated    int
	Removed    int
}

// Run syncs once: incrementally from the stored history cursor, or with a
// full reconcile when there is none, it is invalid, or Gmail expired it.
func (g *GmailSync) Run(ctx context.Context) (res GmailResult, err error) {
	defer func() {
		var re *gmailapi.ReadError
		if err != nil && errors.As(err, &re) && re.Failure != oauth2.MailAPIFailureNone && g.OnFailure != nil {
			g.OnFailure(re.Failure, err)
		}
	}()
	tokens, err := g.Tokens()
	if err != nil {
		return res, &gmailapi.ReadError{Failure: oauth2.ClassifyMailAPIError(err), Err: err}
	}
	if err = tokens.RequireMailAPICapability("read", false); err != nil {
		return res, &gmailapi.ReadError{Failure: oauth2.MailAPIFailurePartialConsent, Err: err}
	}
	if g.folderCache == nil {
		g.folderCache = &apiFolders{engine: g.Engine, accountID: g.AccountID}
	}
	store := g.Engine.messageStore
	cur, err := store.LoadAPICursor(g.AccountID, gmailScope)
	if err != nil {
		return res, err
	}
	labels, err := g.labels(ctx)
	if err != nil {
		return res, err
	}
	// labels.list is the authoritative container listing. Create mapped folders
	// even when their current message set is empty.
	for _, label := range labels {
		for _, f := range message.MapGmailLabels([]string{label.ID}, labels).Folders {
			if _, err := g.ensureFolder(f); err != nil {
				return res, err
			}
		}
	}
	if cur.Kind == gmailCursorKind && cur.Cursor != "" && !cur.Invalid {
		err = g.incremental(ctx, cur, labels, &res)
		if !errors.Is(err, gmailapi.ErrHistoryExpired) {
			return res, err
		}
		if err = store.InvalidateAPICursor(g.AccountID, gmailScope); err != nil {
			return res, err
		}
		if cur, err = store.LoadAPICursor(g.AccountID, gmailScope); err != nil {
			return res, err
		}
	}
	res.Reconciled = true
	return res, g.reconcile(ctx, cur, labels, &res)
}

func (g *GmailSync) labels(ctx context.Context) ([]message.GmailLabel, error) {
	ls, err := g.withBackoff(ctx, func() (any, error) { return g.Reader.Labels(ctx) })
	if err != nil {
		return nil, err
	}
	var out []message.GmailLabel
	for _, l := range ls.([]gmailapi.Label) {
		out = append(out, message.GmailLabel{ID: l.ID, Name: l.Name, Type: l.Type})
	}
	return out, nil
}

// reconcile diffs the remote ID list against the local refs. The history ID is
// captured before listing, so anything that changes meanwhile is replayed by
// the first incremental run. Until the last page commits the old data stays
// and the cursor stays invalid; a failure never wipes anything.
func (g *GmailSync) reconcile(ctx context.Context, cur message.APICursor, labels []message.GmailLabel, res *GmailResult) error {
	store := g.Engine.messageStore
	sv, err := g.withBackoff(ctx, func() (any, error) { return g.Reader.Profile(ctx) })
	if err != nil {
		return err
	}
	start := sv.(string)
	var remote []string
	for token := ""; ; {
		var ids []string
		var next string
		if _, err = g.withBackoff(ctx, func() (any, error) {
			var e error
			ids, next, e = g.Reader.ListIDs(ctx, token)
			return nil, e
		}); err != nil {
			return err
		}
		remote = append(remote, ids...)
		if token = next; token == "" {
			break
		}
	}
	local, err := store.ListAPIRemoteIDs(g.AccountID, gmailAPIProviderName)
	if err != nil {
		return err
	}
	inRemote := make(map[string]bool, len(remote))
	for _, id := range remote {
		inRemote[id] = true
	}
	var removals []string
	for _, id := range local {
		if !inRemote[id] {
			removals = append(removals, id)
		}
	}
	for chunk := range slices.Chunk(remote, gmailBatchSize) {
		gone, err := g.commitBatch(ctx, chunk, labels, cur, "", "", res)
		if err != nil {
			return err
		}
		removals = append(removals, gone...)
	}
	res.Removed += len(removals)
	return store.CommitAPIPage(message.APIPage{AccountID: g.AccountID, Provider: gmailAPIProviderName, Scope: gmailScope,
		Generation: cur.Generation, Removals: removals, Cursor: start, CursorKind: gmailCursorKind, ClearInvalid: true})
}

// incremental replays history after the stored cursor. Net state per message
// wins over record order (a deleted message stays deleted); batches re-commit
// the old cursor, so an interrupted run resumes by repeating idempotent work.
func (g *GmailSync) incremental(ctx context.Context, cur message.APICursor, labels []message.GmailLabel, res *GmailResult) error {
	deleted := map[string]bool{}
	var touched []string
	seen := map[string]bool{}
	last := cur.Cursor
	for token := ""; ; {
		v, err := g.withBackoff(ctx, func() (any, error) { return g.Reader.History(ctx, cur.Cursor, token) })
		if err != nil {
			return err
		}
		page := v.(*gmailapi.HistoryPage)
		for _, rec := range page.Records {
			added, del, relabelled := rec.IDs()
			for _, id := range append(added, relabelled...) {
				if !seen[id] {
					seen[id] = true
					touched = append(touched, id)
				}
			}
			for _, id := range del {
				deleted[id] = true
			}
		}
		if page.HistoryID != "" {
			last = page.HistoryID
		}
		if token = page.NextPageToken; token == "" {
			break
		}
	}
	var ids, removals []string
	for _, id := range touched {
		if deleted[id] {
			continue
		}
		ids = append(ids, id)
	}
	for id := range deleted {
		removals = append(removals, id)
	}
	slices.Sort(removals)
	for chunk := range slices.Chunk(ids, gmailBatchSize) {
		gone, err := g.commitBatch(ctx, chunk, labels, cur, cur.Cursor, gmailCursorKind, res)
		if err != nil {
			return err
		}
		removals = append(removals, gone...)
	}
	res.Removed += len(removals)
	return g.Engine.messageStore.CommitAPIPage(message.APIPage{AccountID: g.AccountID, Provider: gmailAPIProviderName,
		Scope: gmailScope, Generation: cur.Generation, Removals: removals, Cursor: last, CursorKind: gmailCursorKind})
}

// commitBatch fetches and stores one batch of message IDs in one transaction.
// It returns the IDs Gmail no longer has so the caller removes them.
func (g *GmailSync) commitBatch(ctx context.Context, ids []string, labels []message.GmailLabel, cur message.APICursor,
	cursor, kind string, res *GmailResult) (gone []string, err error) {
	type outcome struct {
		up       *message.APIUpsert
		gone     bool
		existing bool
	}
	workers := g.Concurrency
	if workers < 1 {
		workers = gmailDefaultWorkers
	}
	outs, err := apiFanOut(ctx, workers, ids, func(ctx context.Context, id string) (outcome, error) {
		up, isGone, existing, e := g.process(ctx, id, labels)
		return outcome{up, isGone, existing}, e
	})
	if err != nil {
		return nil, err
	}
	page := message.APIPage{AccountID: g.AccountID, Provider: gmailAPIProviderName, Scope: gmailScope,
		Generation: cur.Generation, Cursor: cursor, CursorKind: kind}
	for i, o := range outs {
		switch {
		case o.gone:
			gone = append(gone, ids[i])
		case o.up != nil:
			page.Upserts = append(page.Upserts, *o.up)
			if o.existing {
				res.Updated++
			} else {
				res.Added++
			}
		}
	}
	return gone, g.Engine.messageStore.CommitAPIPage(page)
}

// process builds the upsert for one remote message: nothing when it is
// unchanged, a flags-only refresh for a label change that adds no folder, and
// otherwise a full ingest from the raw message.
func (g *GmailSync) process(ctx context.Context, id string, labels []message.GmailLabel) (up *message.APIUpsert, gone, existing bool, err error) {
	store := g.Engine.messageStore
	ref, rows, err := store.GetAPIRef(g.AccountID, gmailAPIProviderName, id)
	if err != nil {
		return nil, false, false, err
	}
	if ref != nil {
		v, err := g.withBackoff(ctx, func() (any, error) { return g.Reader.GetMinimal(ctx, id) })
		if errors.Is(err, gmailapi.ErrMessageGone) {
			return nil, true, true, nil
		}
		if err != nil {
			return nil, false, true, err
		}
		min := v.(*gmailapi.Message)
		mapping := message.MapGmailLabels(min.LabelIDs, labels)
		flagsOnly := true
		var upsert message.APIUpsert
		upsert.Ref = gmailRef(min, mapping)
		for _, f := range mapping.Folders {
			folderID, err := g.ensureFolder(f)
			if err != nil {
				return nil, false, true, err
			}
			if _, ok := rows[folderID]; !ok {
				flagsOnly = false
				break
			}
			upsert.Rows = append(upsert.Rows, message.APIRow{FlagsOnly: true, Message: message.Message{
				FolderID: folderID, IsRead: mapping.IsRead, IsStarred: mapping.IsStarred, IsDraft: mapping.IsDraft}})
		}
		// An unchanged version skips the write only while the rows still
		// match the mapped folders; a label rename maps to a new folder.
		if flagsOnly && min.HistoryID == ref.Version && len(upsert.Rows) == len(rows) {
			return nil, false, true, nil
		}
		if flagsOnly {
			return &upsert, false, true, nil
		}
	}
	v, err := g.withBackoff(ctx, func() (any, error) { return g.Reader.GetRaw(ctx, id) })
	if errors.Is(err, gmailapi.ErrMessageGone) {
		return nil, ref != nil, ref != nil, nil // an unknown message has nothing to remove
	}
	if err != nil {
		return nil, false, ref != nil, err
	}
	upsert, err := g.ingest(v.(*gmailapi.Message), labels)
	return upsert, false, ref != nil, err
}

func gmailRef(m *gmailapi.Message, mapping message.APIMapping) message.APIRef {
	return message.APIRef{RemoteID: m.ID, RemoteThreadID: m.ThreadID, Containers: slices.Sorted(slices.Values(m.LabelIDs)),
		Unmapped: mapping.Unmapped, Version: m.HistoryID}
}

// ingest parses a raw message with the IMAP pipeline's parser (parseAPIRaw).
// The Gmail thread ID is stored on the ref; the local thread ID derives from headers.
func (g *GmailSync) ingest(raw *gmailapi.Message, labels []message.GmailLabel) (*message.APIUpsert, error) {
	mapping := message.MapGmailLabels(raw.LabelIDs, labels)
	m, extras := g.Engine.parseAPIRaw(g.AccountID, raw.ID, "gmail.invalid", raw.Raw)
	m.IsRead, m.IsStarred, m.IsDraft = mapping.IsRead, mapping.IsStarred, mapping.IsDraft
	up := &message.APIUpsert{Ref: gmailRef(raw, mapping)}
	for _, f := range mapping.Folders {
		folderID, err := g.ensureFolder(f)
		if err != nil {
			return nil, err
		}
		row := m
		row.FolderID = folderID
		up.Rows = append(up.Rows, message.APIRow{Message: row, Extras: extras})
	}
	return up, nil
}

// ensureFolder returns the local folder for a mapped label, creating it once.
func (g *GmailSync) ensureFolder(f message.APIFolder) (string, error) {
	return g.folderCache.ensure(f)
}

// withBackoff retries Gmail rate limits (see apiBackoff).
func (g *GmailSync) withBackoff(ctx context.Context, do func() (any, error)) (any, error) {
	return apiBackoff(ctx, g.Sleep, func(err error) (bool, time.Duration) {
		var re *gmailapi.ReadError
		if errors.As(err, &re) && re.RateLimited {
			return true, re.RetryAfter
		}
		return false, 0
	}, do)
}
