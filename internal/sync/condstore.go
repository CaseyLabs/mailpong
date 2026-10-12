package sync

// CONDSTORE / CHANGEDSINCE incremental flag sync.
//
// RFC 7162 lets servers tag every flag change with a monotonic MODSEQ counter.
// When supported, the client can ask FETCH 1:* (FLAGS) (CHANGEDSINCE <prev>)
// and the server returns only UIDs whose flags changed after <prev> — typically
// 0-10 messages per sync instead of every UID in the mailbox.
//
// For Mailven users with 10k+ inboxes this turns flag sync from a multi-second
// pre-cycle stall into a single sub-100ms round-trip.
//
// Files split for review/test isolation:
//   - condstore.go      — pure decision helpers + the new IO method
//   - messages.go       — keeps the existing full-sync fallback verbatim
//   - condstore_test.go — unit tests for the pure helpers
//
// Correctness story lives in nextModSeq. Anywhere we advance the persisted
// HighestModSeq after a flag sync that didn't succeed, the next sync silently
// skips whatever changes we missed (it asks for "what changed since [the new
// modseq]" — which excludes the ones we never saw). So the test for nextModSeq
// nails that invariant: failure ⇒ pinned, success ⇒ advance.

import (
	"context"
	"fmt"

	"github.com/caseylabs/mailven/internal/message"
	"github.com/emersion/go-imap/v2"
	imapclient "github.com/emersion/go-imap/v2/imapclient"
)

// shouldUseCondStore returns true when the current sync cycle can use the
// incremental CHANGEDSINCE fetch path. All inputs come straight from the
// orchestrator; the function has no side effects so it's trivially testable.
//
// The four "no" branches:
//
//	uidValidityChanged   - the mailbox was recreated server-side, so the
//	                       MODSEQ we stored last time refers to a different
//	                       universe of UIDs. Must do a full resync.
//	prevModSeq == 0      - first-ever sync for this folder (or after a
//	                       rollback that cleared the column). Nothing to be
//	                       incremental against. Do full; next cycle uses
//	                       the modseq we captured this round.
//	mailboxModSeq == 0   - server didn't return HIGHESTMODSEQ in the SELECT
//	                       response despite advertising the capability. Skip
//	                       the incremental path and fall back; we can't
//	                       advance a baseline we don't have.
//	!supportsCondStore   - server lacks the capability outright. Always full.
func shouldUseCondStore(uidValidityChanged bool, prevModSeq, mailboxModSeq uint64, supportsCondStore bool) bool {
	if uidValidityChanged {
		return false
	}
	if prevModSeq == 0 {
		return false
	}
	if mailboxModSeq == 0 {
		return false
	}
	if !supportsCondStore {
		return false
	}
	return true
}

// nextModSeq returns the value to persist as the folder's new HighestModSeq.
// This is the single load-bearing safety invariant of the whole CONDSTORE
// fix: advancing the baseline after a flag sync that didn't succeed means
// the next cycle's CHANGEDSINCE filter skips whatever the failed cycle
// missed — silently. Forever, unless something else triggers a full resync.
//
// Rules:
//
//	flagSyncOK == false                         → pin to prevModSeq (retry next cycle)
//	mailboxModSeq == 0 (server didn't return)   → pin to prevModSeq (don't lose what we had)
//	otherwise                                   → advance to mailboxModSeq
func nextModSeq(flagSyncOK bool, mailboxModSeq, prevModSeq uint64) uint64 {
	if !flagSyncOK {
		return prevModSeq
	}
	if mailboxModSeq == 0 {
		return prevModSeq
	}
	return mailboxModSeq
}

const (
	// Below this many existing messages, a full FLAGS reconciliation runs every
	// sync — cheap, and correct regardless of a server's (possibly broken)
	// CONDSTORE. This is what fixes multi-client read/unread for iCloud, Yahoo,
	// Exchange etc. whose CHANGEDSINCE can't be trusted.
	flagFullReconcileThreshold = 2000
	// At/above the threshold, the CONDSTORE fast-path avoids an O(all-messages)
	// fetch every cycle — but we still force a FULL sweep every N cycles so any
	// gap left by a broken/partial CONDSTORE self-heals.
	flagFullSweepEvery = 10
)

// runFlagSync orchestrates one cycle's flag sync. Full reconciliation
// (syncMessageFlags over every existing UID) is the AUTHORITATIVE path and runs
// every sync for normal-sized mailboxes, so a provider whose CONDSTORE is
// broken/absent can never cause a permanent miss. The CONDSTORE incremental
// path is used only as a fast-path for large mailboxes, and even then a full
// sweep is forced every flagFullSweepEvery cycles.
//
// Returns flagSyncOK — true when the cycle's flag state can be trusted (so
// HighestModSeq may advance via nextModSeq); false when the flag sync failed
// and the persisted modseq must be pinned so the next cycle re-checks.
//
// preferIncremental forces the CONDSTORE fast-path whenever CONDSTORE is usable,
// bypassing the size threshold and periodic sweep. It's set by the IDLE
// flag-change path (SyncFolderFlags), which needs a near-instant reconcile and
// relies on the scheduled full reconciliation as the correctness net. The
// scheduled path passes false, keeping its full-for-small-mailboxes behavior.
//
// Guard-clause style throughout — no if/else, per project convention.
func (e *Engine) runFlagSync(
	ctx context.Context,
	rawClient *imapclient.Client,
	folderID string,
	existingUIDs []uint32,
	uidValidityChanged bool,
	prevModSeq, mailboxModSeq uint64,
	supportsCondStore bool,
	preferIncremental bool,
) bool {
	// CONDSTORE incremental: forced for the IDLE fast path (preferIncremental),
	// otherwise only for large mailboxes on non-sweep cycles. The short-circuit
	// keeps dueForFullFlagSweep (which mutates a counter) from running on the
	// IDLE path.
	useIncremental := shouldUseCondStore(uidValidityChanged, prevModSeq, mailboxModSeq, supportsCondStore) &&
		(preferIncremental ||
			(len(existingUIDs) >= flagFullReconcileThreshold && !e.dueForFullFlagSweep(folderID)))

	if !useIncremental {
		e.log.Debug().
			Str("folder", folderID).
			Int("existing", len(existingUIDs)).
			Bool("condstore", supportsCondStore).
			Msg("Flag sync: full reconciliation")
		if err := e.syncMessageFlags(ctx, rawClient, folderID, existingUIDs); err != nil {
			e.log.Warn().Err(err).Msg("Full flag reconciliation failed")
			return false
		}
		return true
	}

	// Fast-path: CONDSTORE incremental. Tiny payload.
	changed, err := e.syncMessageFlagsChangedSince(ctx, rawClient, folderID, prevModSeq)
	if err == nil {
		e.log.Debug().
			Str("folder", folderID).
			Int("changed", changed).
			Int("existing", len(existingUIDs)).
			Uint64("sinceModSeq", prevModSeq).
			Msg("Flag sync: incremental (CONDSTORE)")
		return true
	}

	// CONDSTORE errored → full reconciliation this cycle; pin modseq on failure.
	e.log.Warn().Err(err).Uint64("sinceModSeq", prevModSeq).
		Msg("Incremental (CONDSTORE) flag sync failed, falling back to full")
	if ferr := e.syncMessageFlags(ctx, rawClient, folderID, existingUIDs); ferr != nil {
		e.log.Warn().Err(ferr).Msg("Fallback full flag sync also failed")
		return false
	}
	return true
}

// dueForFullFlagSweep increments a per-folder counter and returns true every
// flagFullSweepEvery calls, forcing a periodic full flag reconciliation on
// large mailboxes even while the CONDSTORE fast-path is available — so a
// broken/partial CONDSTORE self-heals over time.
func (e *Engine) dueForFullFlagSweep(folderID string) bool {
	e.flagSweepMu.Lock()
	defer e.flagSweepMu.Unlock()
	e.flagSweepCounter[folderID]++
	return e.flagSweepCounter[folderID]%flagFullSweepEvery == 0
}

// syncMessageFlagsChangedSince issues a single FETCH 1:* (FLAGS) (CHANGEDSINCE
// sinceModSeq) against the server. Returns the number of flag updates applied,
// or an error. The caller MUST treat any non-nil return as "do not advance
// modseq" (use nextModSeq with flagSyncOK=false).
//
// Reuses the flag-mapping pattern from syncMessageFlags so the two paths
// produce identical FlagUpdate records — only the fetch criterion differs.
func (e *Engine) syncMessageFlagsChangedSince(ctx context.Context, client *imapclient.Client, folderID string, sinceModSeq uint64) (int, error) {
	if sinceModSeq == 0 {
		return 0, fmt.Errorf("invalid zero modseq baseline")
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	// UID range 1:* — emersion/go-imap/v2 encodes Stop=0 as "*" (see
	// numset.go AddNum doc: "The value 0 represents \"*\""). CHANGEDSINCE
	// filters the result down to only messages modified after sinceModSeq,
	// so the response is typically tiny regardless of mailbox size.
	uidSet := imap.UIDSet{}
	uidSet.AddRange(imap.UID(1), imap.UID(0))

	fetchOptions := &imap.FetchOptions{
		UID:          true,
		Flags:        true,
		ChangedSince: sinceModSeq,
	}

	fetchCmd := client.Fetch(uidSet, fetchOptions)

	var flagUpdates []message.FlagUpdate
	for {
		msg := fetchCmd.Next()
		if msg == nil {
			break
		}

		var fetchedUID uint32
		var isRead, isStarred, isAnswered, isForwarded, isDraft, isDeleted bool

		for {
			item := msg.Next()
			if item == nil {
				break
			}
			switch data := item.(type) {
			case imapclient.FetchItemDataUID:
				fetchedUID = uint32(data.UID)
			case imapclient.FetchItemDataFlags:
				for _, flag := range data.Flags {
					switch flag {
					case imap.FlagSeen:
						isRead = true
					case imap.FlagFlagged:
						isStarred = true
					case imap.FlagAnswered:
						isAnswered = true
					case imap.FlagDraft:
						isDraft = true
					case imap.FlagDeleted:
						isDeleted = true
					case "$Forwarded", "\\Forwarded":
						isForwarded = true
					}
				}
			}
		}

		if fetchedUID > 0 {
			flagUpdates = append(flagUpdates, message.FlagUpdate{
				UID:         fetchedUID,
				IsRead:      isRead,
				IsStarred:   isStarred,
				IsAnswered:  isAnswered,
				IsForwarded: isForwarded,
				IsDraft:     isDraft,
				IsDeleted:   isDeleted,
			})
		}
	}

	if err := fetchCmd.Close(); err != nil {
		return 0, fmt.Errorf("failed to fetch changed flags: %w", err)
	}

	if len(flagUpdates) > 0 {
		if err := e.messageStore.UpdateFlagsByUIDBatch(folderID, flagUpdates); err != nil {
			return 0, fmt.Errorf("failed to batch update changed flags: %w", err)
		}
	}

	return len(flagUpdates), nil
}
