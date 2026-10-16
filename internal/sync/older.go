package sync

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/caseylabs/mailven/internal/folder"
)

// searchableFolders lists the folders a server search or Load older reads:
// the account's subscribed folders except Trash and Spam, whose mail the chat
// list never shows.
func (e *Engine) searchableFolders(accountID string) ([]*folder.Folder, error) {
	all, err := e.folderStore.ListSubscribed(accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to list folders: %w", err)
	}
	return slices.DeleteFunc(all, func(f *folder.Folder) bool {
		return f.Type == folder.TypeTrash || f.Type == folder.TypeSpam
	}), nil
}

// buildSearchCriteria ORs the query across sender, subject, recipients and
// body. Many servers (Gmail in particular) implement TEXT poorly, so the
// fields are named explicitly: OR FROM q OR SUBJECT q OR TO q OR CC q BODY q.
func buildSearchCriteria(query string) *imap.SearchCriteria {
	header := func(key string) imap.SearchCriteria {
		return imap.SearchCriteria{Header: []imap.SearchCriteriaHeaderField{{Key: key, Value: query}}}
	}
	or := func(l, r imap.SearchCriteria) imap.SearchCriteria {
		return imap.SearchCriteria{Or: [][2]imap.SearchCriteria{{l, r}}}
	}
	body := imap.SearchCriteria{Body: []string{query}}
	c := or(header("FROM"), or(header("SUBJECT"), or(header("TO"), or(header("CC"), body))))
	return &c
}

// searchUIDs runs a UID search on the selected mailbox.
func searchUIDs(ctx context.Context, client *imapclient.Client, criteria *imap.SearchCriteria) ([]uint32, error) {
	data, err := waitSearch(ctx, client.UIDSearch(criteria, nil))
	if err != nil {
		return nil, fmt.Errorf("IMAP search failed: %w", err)
	}
	var uids []uint32
	for _, uid := range data.AllUIDs() {
		uids = append(uids, uint32(uid))
	}
	return uids, nil
}

// storePinned fetches headers for the UIDs not yet local and pins what was
// stored, so the window's deletions leave it alone. It reports how many were
// stored.
func (e *Engine) storePinned(ctx context.Context, client *imapclient.Client, accountID, folderID string, uids []uint32) (int, error) {
	stored := 0
	for chunk := range slices.Chunk(uids, headerBatchSize) {
		if err := e.fetchMessageHeaders(ctx, client, accountID, folderID, chunk); err != nil {
			return stored, err
		}
		if err := e.messageStore.PinByUIDs(folderID, chunk); err != nil {
			return stored, err
		}
		e.changeSeq.bump(folderID)
		stored += len(chunk)
	}
	return stored, nil
}

// notLocal returns the UIDs of uids that the folder does not hold yet.
func (e *Engine) notLocal(folderID string, uids []uint32) ([]uint32, error) {
	local, err := e.messageStore.GetAllUIDs(folderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get local UIDs: %w", err)
	}
	have := make(map[uint32]bool, len(local))
	for _, uid := range local {
		have[uid] = true
	}
	return slices.DeleteFunc(slices.Clone(uids), func(uid uint32) bool { return have[uid] }), nil
}

// withFolder selects a folder on a pooled connection, serialized with the
// folder's reconciles, and runs fn.
func (e *Engine) withFolder(ctx context.Context, accountID string, f *folder.Folder, fn func(*imapclient.Client) error) error {
	unlock, err := e.folderLocks.lock(ctx, f.ID)
	if err != nil {
		return err
	}
	defer unlock()
	conn, err := e.pool.GetConnection(ctx, accountID)
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}
	defer func() { e.pool.Release(conn) }()
	mailbox, err := conn.Client().SelectMailbox(ctx, f.Path)
	if err != nil {
		return fmt.Errorf("failed to select mailbox: %w", err)
	}
	// A recreated mailbox is the scheduled sync's to reset; its UIDs are not
	// the ones stored here.
	if f.UIDValidity != 0 && f.UIDValidity != mailbox.UIDValidity {
		return fmt.Errorf("folder %s changed on the server; sync it first", f.Name)
	}
	return fn(conn.Client().RawClient())
}

// SearchServer searches the account's folders on the server and stores the
// matches not yet local (pinned, so the sync window keeps them), up to limit
// across the account. found counts the matches the server reported.
func (e *Engine) SearchServer(ctx context.Context, accountID, query string, limit int) (found, added int, err error) {
	folders, err := e.searchableFolders(accountID)
	if err != nil {
		return 0, 0, err
	}
	for _, f := range folders {
		if limit > 0 && added >= limit {
			break
		}
		err = e.withFolder(ctx, accountID, f, func(client *imapclient.Client) error {
			uids, err := searchUIDs(ctx, client, buildSearchCriteria(query))
			if err != nil {
				return err
			}
			found += len(uids)
			missing, err := e.notLocal(f.ID, uids)
			if err != nil {
				return err
			}
			// Newest first: the highest UIDs arrived last.
			sort.Slice(missing, func(i, j int) bool { return missing[i] > missing[j] })
			if limit > 0 && len(missing) > limit-added {
				missing = missing[:limit-added]
			}
			n, err := e.storePinned(ctx, client, accountID, f.ID, missing)
			added += n
			return err
		})
		if err != nil {
			return found, added, err
		}
	}
	return found, added, nil
}

// LoadOlder stores the next page of mail from before the sync window in each
// of the account's folders, newest first, pinned so the window keeps it.
// done reports that no folder has older mail left; with no sync period the
// account is already complete.
func (e *Engine) LoadOlder(ctx context.Context, accountID string, syncPeriodDays, pageSize int) (found, added int, done bool, err error) {
	if syncPeriodDays <= 0 {
		return 0, 0, true, nil
	}
	folders, err := e.searchableFolders(accountID)
	if err != nil {
		return 0, 0, false, err
	}
	before := time.Now().AddDate(0, 0, -syncPeriodDays)
	done = true
	for _, f := range folders {
		err = e.withFolder(ctx, accountID, f, func(client *imapclient.Client) error {
			uids, err := searchUIDs(ctx, client, &imap.SearchCriteria{
				Before:  before,
				NotFlag: []imap.Flag{imap.FlagDeleted},
			})
			if err != nil {
				return err
			}
			missing, err := e.notLocal(f.ID, uids)
			if err != nil {
				return err
			}
			found += len(missing)
			sort.Slice(missing, func(i, j int) bool { return missing[i] > missing[j] })
			if len(missing) > pageSize {
				missing = missing[:pageSize]
				done = false
			}
			n, err := e.storePinned(ctx, client, accountID, f.ID, missing)
			added += n
			return err
		})
		if err != nil {
			return found, added, false, err
		}
	}
	return found, added, done, nil
}
