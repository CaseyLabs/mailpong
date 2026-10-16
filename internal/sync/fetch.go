package sync

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/caseylabs/mailven/internal/folder"
	imapPkg "github.com/caseylabs/mailven/internal/imap"
	"github.com/caseylabs/mailven/internal/message"
	"github.com/caseylabs/mailven/internal/smime"
	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// ProcessedBody holds the parsed body content and attachments for a message
type ProcessedBody struct {
	MessageID      string
	BodyHTML       string
	BodyText       string
	Snippet        string
	HasAttachments bool
	Attachments    []*message.Attachment  // Extracted during parsing (no re-parse needed)
	RawBytes       []byte                 // For on-demand attachment content fetch
	SMIMEResult    *smime.SignatureResult // S/MIME verification result
	SMIMERawBody   []byte                 // Raw S/MIME body for on-view processing
	SMIMEEncrypted bool                   // Whether the message is encrypted
	PGPRawBody     []byte                 // Raw PGP body for on-view processing
	PGPEncrypted   bool                   // Whether the message is PGP encrypted

	// Size signals used by shouldChargeFailure to distinguish "true empty
	// body" from "server-side truncation". ReportedSize comes from the
	// IMAP RFC822.SIZE response item; ReceivedBytes is what we actually
	// read from the BODY[] literal. A meaningful shortfall between the
	// two (and below maxMessageSize, to exclude Mailven's own cap) means
	// the FETCH was likely truncated and we should NOT persist the
	// failure — the next sync may succeed.
	ReportedSize  int64
	ReceivedBytes int64

	// Incomplete means the server returned the message but no complete
	// body (no body section, an empty one, or a read that failed partway).
	// Only MessageID and the size signals are set; nothing is stored, and
	// the message is neither treated as fetched nor deleted.
	Incomplete bool
}

// FetchMessageBody fetches the body for a single message on-demand.
// Uses streaming fetch internally to avoid blocking on .Collect().
func (e *Engine) FetchMessageBody(ctx context.Context, accountID, messageID string) (*message.Message, error) {
	// Get message from store to get UID and folder
	uid, folderID, err := e.messageStore.GetMessageUIDAndFolder(messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get message info: %w", err)
	}

	// Get folder to get path
	f, err := e.folderStore.Get(folderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get folder: %w", err)
	}

	e.log.Debug().
		Str("messageID", messageID).
		Uint32("uid", uid).
		Str("folder", f.Path).
		Msg("Fetching message body on-demand")

	// Get a connection from the pool
	conn, err := e.pool.GetConnection(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	var streamErr error
	defer func() {
		if errors.Is(streamErr, errStreamBroken) {
			e.pool.Discard(conn)
			return
		}
		e.pool.Release(conn)
	}()

	// Select the mailbox
	_, err = conn.Client().SelectMailbox(ctx, f.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to select mailbox: %w", err)
	}

	// Use fetchMessageBodiesBatch for streaming fetch (avoids .Collect() blocking)
	uidToMessageID := map[uint32]string{uid: messageID}
	results, err := e.fetchMessageBodiesBatch(ctx, conn.Client().RawClient(), uidToMessageID)
	streamErr = err
	if err != nil {
		return nil, fmt.Errorf("fetch body failed: %w", err)
	}

	result, ok := results[uid]
	if ok && result.Incomplete {
		return nil, fmt.Errorf("server returned an incomplete message body")
	}
	if !ok || result == nil {
		// Message no longer exists on server — clean up the ghost
		e.log.Warn().Str("messageID", messageID).Uint32("uid", uid).Msg("Message not found on server, deleting ghost")
		if delErr := e.messageStore.Delete(messageID); delErr != nil {
			e.log.Debug().Err(delErr).Str("messageID", messageID).Msg("Failed to delete ghost message")
		} else {
			e.changeSeq.bump(folderID)
		}
		return nil, fmt.Errorf("message not found on server")
	}

	// Update message in store
	if err := e.messageStore.UpdateBody(messageID, result.BodyHTML, result.BodyText, result.Snippet, result.HasAttachments); err != nil {
		return nil, fmt.Errorf("failed to update message body: %w", err)
	}
	e.changeSeq.bump(folderID)

	// Replace attachments so a re-fetch doesn't list them twice
	if e.attachmentStore != nil {
		if err := e.attachmentStore.ReplaceForMessages([]string{messageID}, result.Attachments); err != nil {
			e.log.Debug().Err(err).Str("messageID", messageID).Msg("Failed to save attachment metadata")
		}
	}

	// Return updated message
	return e.messageStore.Get(messageID)
}

// replaceConn discards a dead connection and returns a fresh one with the
// mailbox at path selected.
func (e *Engine) replaceConn(ctx context.Context, old *imapPkg.PooledConnection, accountID, path string) (*imapPkg.PooledConnection, error) {
	e.pool.Discard(old)
	conn, err := e.pool.GetConnection(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get new connection: %w", err)
	}
	if _, err := conn.Client().SelectMailbox(ctx, path); err != nil {
		e.pool.Release(conn)
		return nil, fmt.Errorf("failed to select mailbox on new connection: %w", err)
	}
	return conn, nil
}

// errStreamBroken reports that a FETCH literal ended early and the client was
// closed. The results returned alongside it are still valid.
var errStreamBroken = errors.New("FETCH literal ended early, connection closed")

// readLiteral reads a FETCH literal, up to limit bytes. A dropped connection
// can end the literal early without an error, so a read shorter than the
// literal's declared size is reported as io.ErrUnexpectedEOF.
//
// After an error, the caller must not call Next or Close on the message or
// the fetch command. go-imap v2's fetchLiteralReader has already handed its
// buffered reader back to the read goroutine, and discarding the literal's
// unread bytes would race with it. Close the client instead; the pool then
// drops the connection.
func readLiteral(lit imap.LiteralReader, limit int64) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(lit, limit))
	if want := min(lit.Size(), limit); err == nil && int64(len(raw)) < want {
		err = fmt.Errorf("read %d of %d literal bytes: %w", len(raw), want, io.ErrUnexpectedEOF)
	}
	return raw, err
}

// fetchMessageBodiesBatch fetches bodies for multiple messages in a single IMAP command
// The mailbox must already be selected by the caller.
// Returns a map of UID -> ProcessedBody for successfully fetched messages.
//
// Uses streaming (Next() loop) instead of Collect() to:
// - Avoid indefinite blocking if connection hangs
// - Allow context cancellation between messages
// - Return partial results if connection dies mid-batch
func (e *Engine) fetchMessageBodiesBatch(ctx context.Context, client *imapclient.Client, uidToMessageID map[uint32]string) (map[uint32]*ProcessedBody, error) {
	if len(uidToMessageID) == 0 {
		return make(map[uint32]*ProcessedBody), nil
	}

	// Check context
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Build UID set for batch fetch
	uidSet := imap.UIDSet{}
	for uid := range uidToMessageID {
		uidSet.AddNum(imap.UID(uid))
	}

	e.log.Debug().
		Int("count", len(uidToMessageID)).
		Msg("Fetching message bodies in batch")

	fetchOptions := &imap.FetchOptions{
		UID: true,
		BodySection: []*imap.FetchItemBodySection{
			{
				Specifier: imap.PartSpecifierNone, // Full message
				Peek:      true,                   // Don't mark as read
			},
		},
		RFC822Size: true,
	}

	fetchCmd := client.Fetch(uidSet, fetchOptions)
	results := make(map[uint32]*ProcessedBody)

	// Stream messages one at a time instead of blocking on Collect()
	// This allows cancellation between messages and returns partial results on error
	streamBroken := false
	for !streamBroken {
		// Check for cancellation between messages
		if ctx.Err() != nil {
			fetchCmd.Close()
			e.log.Debug().
				Int("fetched", len(results)).
				Int("requested", len(uidToMessageID)).
				Msg("Fetch cancelled, returning partial results")
			return results, ctx.Err()
		}

		msg := fetchCmd.Next()
		if msg == nil {
			break
		}

		// Extract UID and body section from streamed message
		var fetchedUID imap.UID
		var rawBytes []byte
		var gotBodySection bool
		var readErr error
		var reportedSize int64 // RFC822.SIZE; 0 if server didn't return it

		for readErr == nil {
			item := msg.Next()
			if item == nil {
				break
			}

			switch data := item.(type) {
			case imapclient.FetchItemDataUID:
				fetchedUID = data.UID
			case imapclient.FetchItemDataRFC822Size:
				// Captured so shouldChargeFailure can compare against bytes
				// actually read and detect server-side truncation before
				// persisting body_failed = 1.
				reportedSize = int64(data.Size)
			case imapclient.FetchItemDataBodySection:
				gotBodySection = true
				// Read body from literal reader with size limit to prevent memory exhaustion
				if data.Literal != nil {
					rawBytes, readErr = readLiteral(data.Literal, maxMessageSize)
					if readErr != nil {
						if err := ctx.Err(); err != nil {
							client.Close()
							e.log.Debug().Err(err).Msg("Body fetch cancelled during literal read")
							return results, err
						}
						e.log.Warn().
							Err(readErr).
							Uint32("uid", uint32(fetchedUID)).
							Msg("Failed to read body literal")
					}
					// Log if we hit the size limit
					if int64(len(rawBytes)) == maxMessageSize {
						e.log.Warn().
							Uint32("uid", uint32(fetchedUID)).
							Int64("maxSize", maxMessageSize).
							Msg("Message body truncated at size limit")
					}
				} else {
					e.log.Warn().
						Uint32("uid", uint32(fetchedUID)).
						Msg("Body section has nil Literal reader")
				}
			}
		}

		// After a failed read, stop consuming the command (see readLiteral).
		streamBroken = readErr != nil

		// Log if we didn't receive a body section at all
		if !gotBodySection && fetchedUID != 0 {
			e.log.Warn().
				Uint32("uid", uint32(fetchedUID)).
				Msg("No body section in IMAP response for message")
		}

		uid := uint32(fetchedUID)
		if uid == 0 {
			e.log.Warn().Msg("Received message without UID in batch response")
			continue
		}

		messageID, ok := uidToMessageID[uid]
		if !ok {
			e.log.Warn().Uint32("uid", uid).Msg("Received unexpected UID in batch response")
			continue
		}

		// The server listed this UID, so the message exists: a missing,
		// empty or partly read body is a fetch failure, not a deletion.
		// Keep the size signals so markUnresolvedAsFailed can decide
		// whether to retry it next sync.
		if !gotBodySection || readErr != nil || len(rawBytes) == 0 {
			e.log.Warn().
				Uint32("uid", uid).
				Str("messageID", messageID).
				Int("bodySize", len(rawBytes)).
				Msg("Incomplete message body, not storing")
			results[uid] = &ProcessedBody{
				MessageID:     messageID,
				ReportedSize:  reportedSize,
				ReceivedBytes: int64(len(rawBytes)),
				Incomplete:    true,
			}
			continue
		}

		e.log.Debug().
			Uint32("uid", uid).
			Int("bodySize", len(rawBytes)).
			Msg("Processing message body")

		// Parse body content with timeout, extracting attachments in the same pass
		parsed := e.parseMessageBodyFull(rawBytes, messageID, 30*time.Second)

		// Generate snippet
		var snippet string
		if parsed.BodyText != "" {
			snippet = generateSnippet(parsed.BodyText, 200)
		} else if parsed.BodyHTML != "" {
			snippet = generateSnippet(stripHTMLTags(parsed.BodyHTML), 200)
		}

		results[uid] = &ProcessedBody{
			MessageID:      messageID,
			BodyHTML:       parsed.BodyHTML, // sanitized by the parser
			BodyText:       parsed.BodyText,
			Snippet:        snippet,
			HasAttachments: parsed.HasAttachments,
			Attachments:    parsed.Attachments,
			RawBytes:       rawBytes,
			SMIMEResult:    parsed.SMIMEResult,
			SMIMERawBody:   parsed.SMIMERawBody,
			SMIMEEncrypted: parsed.SMIMEEncrypted,
			PGPRawBody:     parsed.PGPRawBody,
			PGPEncrypted:   parsed.PGPEncrypted,
			ReportedSize:   reportedSize,
			ReceivedBytes:  int64(len(rawBytes)),
		}
	}

	if err := ctx.Err(); err != nil {
		client.Close()
		e.log.Debug().Err(err).Msg("Body fetch cancelled")
		return results, err
	}

	if streamBroken {
		e.log.Warn().
			Int("fetched", len(results)).
			Int("requested", len(uidToMessageID)).
			Msg("Body literal ended early, closing connection and returning partial results")
		client.Close()
		if err := ctx.Err(); err != nil {
			return results, err
		}
		return results, errStreamBroken
	}

	if err := fetchCmd.Close(); err != nil {
		if cancelErr := ctx.Err(); cancelErr != nil {
			e.log.Debug().Err(cancelErr).Msg("Body fetch cancelled while closing command")
			return results, cancelErr
		}
		e.log.Warn().Err(err).
			Int("fetched", len(results)).
			Int("requested", len(uidToMessageID)).
			Msg("Fetch close error, returning partial results")
		// Return what we have, don't fail completely
		// Partial content is better than no content
	}

	e.log.Debug().
		Int("fetched", len(results)).
		Int("requested", len(uidToMessageID)).
		Msg("Batch fetch complete")

	return results, nil
}

// bodyTruncationThreshold is the fraction of the IMAP-reported size below
// which a received body is considered suspiciously short — likely truncated
// in transit rather than legitimately empty. Tuned for the typical case of
// a multipart message: small IMAP framing variations stay above this floor,
// but a real loss of bytes (a clearly-cut-off response) falls under it.
const bodyTruncationThreshold = 0.8

// shouldChargeFailure decides whether a message that came back with no
// usable body should be persisted as body_failed=1. Returns false when the
// signals suggest the failure was a transient truncation, in which case
// the next sync may succeed — so we want to retry rather than permanently
// skip the message.
//
// Decision table (all comparisons in bytes):
//
//	reportedSize == 0          → charge   (no signal to defer on; treat as definitive)
//	received    >= maxMsgSize  → charge   (Mailven's own cap, not server truncation; next fetch hits same wall)
//	received    <  reported*T  → DON'T    (clear shortfall; likely server-side truncation)
//	otherwise                  → charge   (received is close enough to expected; the empty body is real)
//
// Kept as a pure function (no Engine receiver) so it can be unit-tested
// against synthetic inputs without standing up a full sync engine.
func shouldChargeFailure(receivedBytes, reportedSize int64) bool {
	if reportedSize <= 0 {
		return true
	}
	if receivedBytes >= maxMessageSize {
		return true
	}
	threshold := int64(float64(reportedSize) * bodyTruncationThreshold)
	return receivedBytes >= threshold
}

// markUnresolvedAsFailed persists messages.body_failed=1 for any requested ID
// whose body parsed to empty AND wasn't encrypted, OR whose UID the server
// didn't return at all. Encrypted messages (which legitimately carry empty
// plaintext until view-time decryption) are treated as resolved. The flag
// excludes the message from future body-fetch queries via the body_failed=0
// guard in GetMessagesWithoutBody/AndSize/Count (see migration v39).
//
// sizes carries the per-message (received, reported) byte counts captured
// during FETCH so shouldChargeFailure can defer marking when the response
// was likely server-side truncation. IDs absent from sizes — typically
// requested-but-not-returned — fall through to the "no signal" branch and
// are charged (server returned nothing despite RFC822.SIZE being requested:
// either a server bug or a persistent permission issue, both worth bounding).
//
// Without this persistence, the previous in-memory failedParseAttempts map
// reset every sync cycle and re-fetched the same unparseable IDs forever.
// fetchedSize records what the IMAP server said the message size was
// (RFC822.SIZE) vs what we actually read from the BODY[] literal. Used by
// shouldChargeFailure to decide whether to defer marking a message failed.
type fetchedSize struct {
	received int64
	reported int64
}

func (e *Engine) markUnresolvedAsFailed(requestedIDs []string, updates []message.BodyUpdate, sizes map[string]fetchedSize) {
	if len(requestedIDs) == 0 {
		return
	}
	resolved := make(map[string]bool, len(updates))
	for _, u := range updates {
		if u.BodyHTML != "" || u.BodyText != "" || u.SMIMEEncrypted || u.PGPEncrypted {
			resolved[u.MessageID] = true
		}
	}
	var failedIDs []string
	var deferredCount int
	for _, id := range requestedIDs {
		if resolved[id] {
			continue
		}
		// Defer marking when the response looks truncated — retrying next
		// sync may succeed. shouldChargeFailure returns true when no size
		// signal exists (id not in sizes map → zero values → reported=0).
		s := sizes[id]
		if !shouldChargeFailure(s.received, s.reported) {
			deferredCount++
			continue
		}
		failedIDs = append(failedIDs, id)
	}
	if deferredCount > 0 {
		e.log.Info().Int("count", deferredCount).Msg("Deferred marking bodies as parse-failed; FETCH looked truncated, will retry next sync")
	}
	if len(failedIDs) == 0 {
		return
	}
	if err := e.messageStore.MarkBodyFailed(failedIDs); err != nil {
		e.log.Warn().Err(err).Int("count", len(failedIDs)).Msg("Failed to persist body-parse-failed flag")
		return
	}
	e.log.Info().Int("count", len(failedIDs)).Msg("Marked message bodies as parse-failed; excluded from future sync")
}

// FetchBodiesInBackground fetches bodies for messages that don't have them yet.
// This is called after headers sync to fetch bodies in the background.
// syncPeriodDays limits body fetching to messages within the sync period (0 = all messages).
//
// OPTIMIZED: Uses batch IMAP FETCH to fetch multiple message bodies in a single command,
// reducing network round-trips significantly. Uses hybrid byte+count batching (like Geary)
// for memory safety and efficiency:
//   - Max 512KB per batch (memory bounded)
//   - Max 50 messages per batch (even if small)
//   - Min 1 message per batch (handles oversized emails)
//
// Pipeline design for maximum throughput:
//  1. Wait for previous batch's goroutine (if any)
//  2. Apply DB updates from previous batch
//  3. Query candidates and build byte-aware batch
//  4. Fetch bodies via IMAP
//  5. Launch goroutine to parse/sanitize (DB update happens in step 2 of next iteration)
//  6. Repeat
//
// This allows IMAP fetch (network-bound) to run in parallel with parsing (CPU-bound).
// DB updates are synchronous relative to the next DB query to prevent race conditions.
//
// Uses a single IMAP connection for efficiency (reuses connection for all body fetches).
// Includes error recovery: on connection errors, discards dead connection and gets a new one.
// Returns error only if connection recovery fails - individual message failures are logged and skipped.
func (e *Engine) FetchBodiesInBackground(ctx context.Context, accountID, folderID string, syncPeriodDays int) error {
	// Check context at start
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Serialize with other body fetches on this folder: two concurrent runs
	// can both see body_fetched=0 and double-insert attachment rows (plain
	// INSERT, no unique constraint). Separate key namespace from the header
	// lock so header reconciles never queue behind body downloads.
	unlock, err := e.folderLocks.lock(ctx, "body:"+folderID)
	if err != nil {
		return err
	}
	defer unlock()

	// Get folder to get path
	f, err := e.folderStore.Get(folderID)
	if err != nil {
		return fmt.Errorf("failed to get folder: %w", err)
	}

	// Calculate sync date cutoff
	var sinceDate time.Time
	if syncPeriodDays > 0 {
		sinceDate = time.Now().AddDate(0, 0, -syncPeriodDays)
	}

	e.log.Debug().
		Str("account", accountID).
		Str("folder", f.Path).
		Int("syncPeriodDays", syncPeriodDays).
		Msg("Fetching message bodies in background (hybrid batch mode)")

	// Get a SINGLE connection from the pool - reused for all body fetches
	conn, err := e.pool.GetConnection(ctx, accountID)
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}
	// Note: We manage connection lifecycle manually due to recovery logic
	// Don't use defer e.pool.Release(conn) - we handle it explicitly

	// Select the mailbox ONCE
	_, err = conn.Client().SelectMailbox(ctx, f.Path)
	if err != nil {
		e.pool.Release(conn)
		return fmt.Errorf("failed to select mailbox: %w", err)
	}

	// Classify inbox mail synced before bulk detection existed. Shares this
	// connection and the body lock; failures only delay classification.
	if f.Type == folder.TypeInbox {
		if err := e.backfillBulkFlags(ctx, conn.Client().RawClient(), folderID); err != nil {
			e.log.Warn().Err(err).Str("folder", f.Path).Msg("Bulk classification backfill failed")
		}
	}

	// Get total count of messages without body (respecting sync period)
	totalWithoutBody, err := e.messageStore.CountMessagesWithoutBody(folderID, sinceDate)
	if err != nil {
		e.pool.Release(conn)
		return fmt.Errorf("failed to count messages without body: %w", err)
	}

	if totalWithoutBody == 0 {
		e.log.Debug().Msg("All messages have bodies, nothing to fetch")
		// Emit 1/1 so frontend shows 100% complete for bodies phase
		e.emitProgress(accountID, folderID, 1, 1, "bodies")
		e.pool.Release(conn)
		return nil
	}

	e.log.Info().Int("count", totalWithoutBody).Msg("Fetching message bodies (hybrid batch mode)")

	// Emit initial progress so frontend knows body fetch has started
	e.emitProgress(accountID, folderID, 0, totalWithoutBody, "bodies")

	// Tracking for error recovery and progress
	failedBatches := 0      // consecutive batch failures
	connectionFailures := 0 // total connection recovery attempts
	fetched := 0
	failed := 0

	// Note: parse-failure tracking is persisted in messages.body_failed (v39).
	// Once a fetch returns nothing usable for a message, MarkBodyFailed flags
	// it and GetMessagesWithoutBodyAndSize excludes it from future queries —
	// so no in-memory dedup is needed and the cap survives across sessions.

	// Processing result from goroutine - contains parsed data ready for DB.
	// requestedIDs is every message the batch asked the server for, so we can
	// tell which IDs came back unresolved (parsed empty AND not encrypted, or
	// not returned at all) and persist their failure via MarkBodyFailed.
	// sizes maps messageID → (received bytes, IMAP-reported size) so
	// markUnresolvedAsFailed can defer flagging when the FETCH looks truncated.
	type processingResult struct {
		requestedIDs []string
		bodyUpdates  []message.BodyUpdate
		attachments  []*message.Attachment
		sizes        map[string]fetchedSize
		fetchedCount int
	}

	// Channel and pending state for pipelined processing
	var pendingResultChan chan processingResult

	// Start heartbeat logging for long operations - shows sync is alive during long fetches
	heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
	defer cancelHeartbeat()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				e.log.Info().
					Int("fetched", fetched).
					Int("total", totalWithoutBody).
					Int("failed", failed).
					Str("folder", f.Path).
					Msg("Body fetch in progress (heartbeat)")
			case <-heartbeatCtx.Done():
				return
			}
		}
	}()

	for {
		// Step 1: Wait for previous batch's goroutine (if any)
		// Step 2: Apply DB updates from previous batch
		if pendingResultChan != nil {
			e.log.Debug().Msg("Waiting for previous batch goroutine to complete")
			result := <-pendingResultChan
			e.log.Debug().
				Int("bodyUpdates", len(result.bodyUpdates)).
				Int("attachments", len(result.attachments)).
				Int("fetchedCount", result.fetchedCount).
				Msg("Received result from processing goroutine")

			// Persist parse failures via messages.body_failed so we never
			// re-fetch these from IMAP again. A message is "resolved" if its
			// parsed body is non-empty OR it's encrypted (encrypted bodies are
			// intentionally empty until decrypted at view time). Everything
			// else — including IDs the server didn't return — gets flagged.
			e.markUnresolvedAsFailed(result.requestedIDs, result.bodyUpdates, result.sizes)

			// Apply database updates - MUST complete before querying next batch
			if len(result.bodyUpdates) > 0 {
				e.log.Debug().Int("count", len(result.bodyUpdates)).Msg("Applying batch DB update")
				if err := e.messageStore.UpdateBodiesBatch(result.bodyUpdates); err != nil {
					e.log.Warn().Err(err).Msg("Failed to batch update bodies")
					failed += result.fetchedCount
				} else {
					fetched += result.fetchedCount
					e.changeSeq.bump(folderID)
					e.log.Debug().Int("fetched", fetched).Int("total", totalWithoutBody).Msg("DB update successful")
				}
			} else {
				e.log.Warn().Int("fetchedCount", result.fetchedCount).Msg("No body updates in result - bodies may be lost!")
			}
			if len(result.bodyUpdates) > 0 {
				if err := e.attachmentStore.ReplaceForMessages(bodyUpdateIDs(result.bodyUpdates), result.attachments); err != nil {
					e.log.Warn().Err(err).Msg("Failed to batch create attachments")
					// Attachments failed but bodies were saved, don't count as failed
				}
			}

			// Emit progress after DB update completes
			e.log.Debug().Int("fetched", fetched).Int("total", totalWithoutBody).Msg("Emitting progress")
			e.emitProgress(accountID, folderID, fetched, totalWithoutBody, "bodies")
			pendingResultChan = nil
		}

		// Check context before starting new batch
		if ctx.Err() != nil {
			e.log.Debug().Msg("Body fetch cancelled")
			e.pool.Release(conn)
			return ctx.Err()
		}

		// Step 3: Query candidates and build byte-aware batch
		// Get more candidates than we'll use to allow for byte-based selection
		candidates, err := e.messageStore.GetMessagesWithoutBodyAndSize(folderID, bodyBatchQueryLimit, sinceDate)
		if err != nil {
			e.pool.Release(conn)
			return fmt.Errorf("failed to get messages without body: %w", err)
		}

		e.log.Debug().
			Int("candidates", len(candidates)).
			Int("fetched", fetched).
			Int("failed", failed).
			Msg("Queried candidates for next batch")

		if len(candidates) == 0 {
			e.log.Debug().Msg("No more candidates, body sync complete")
			break // All done
		}

		// Note: candidates already excludes any message with body_failed=1
		// (enforced by GetMessagesWithoutBodyAndSize since v39), so the
		// in-memory filter that used to live here is gone.

		// Adaptive batch sizing: use smaller batches for large mailboxes
		// This provides faster recovery if one batch fails and more frequent progress updates
		batchMaxMessages := bodyBatchMaxMessages
		batchMaxBytes := int64(bodyBatchMaxBytes)

		if totalWithoutBody > 1000 {
			batchMaxMessages = 25
			batchMaxBytes = 256 * 1024 // 256KB
			// Log only once (when we first enter the large mailbox mode)
			if fetched == 0 && failed == 0 {
				e.log.Info().
					Int("totalMessages", totalWithoutBody).
					Int("batchMaxMessages", batchMaxMessages).
					Int64("batchMaxBytes", batchMaxBytes).
					Msg("Using smaller batches for large mailbox")
			}
		}

		// Build batch using hybrid byte + count limits
		var batchIDs []string
		var batchBytes int64

		for _, msg := range candidates {
			msgSize := int64(msg.Size)
			if msgSize <= 0 {
				msgSize = 10 * 1024 // Assume 10KB for messages with unknown size
			}

			// Check if adding this message would exceed limits
			wouldExceedBytes := batchBytes+msgSize > batchMaxBytes && len(batchIDs) >= bodyBatchMinMessages
			wouldExceedCount := len(batchIDs) >= batchMaxMessages

			if wouldExceedBytes || wouldExceedCount {
				break // Batch is full
			}

			batchIDs = append(batchIDs, msg.ID)
			batchBytes += msgSize
		}

		if len(batchIDs) == 0 {
			e.log.Warn().Msg("No messages selected for batch")
			break
		}

		e.log.Debug().
			Int("batchSize", len(batchIDs)).
			Int64("batchBytes", batchBytes).
			Msg("Processing batch")

		// Get UIDs for all messages in batch (single DB query)
		uidInfos, err := e.messageStore.GetMessageUIDsAndFolder(batchIDs)
		if err != nil {
			e.log.Warn().Err(err).Msg("Failed to get UIDs for batch, skipping")
			failedBatches++
			if failedBatches > maxMessageRetries {
				e.log.Error().Int("failedBatches", failedBatches).Msg("Too many consecutive batch failures")
				break
			}
			continue
		}

		// Build UID -> messageID map for batch fetch
		uidToMessageID := make(map[uint32]string)
		for msgID, info := range uidInfos {
			uidToMessageID[info.UID] = msgID
		}

		if len(uidToMessageID) == 0 {
			e.log.Warn().Int("requested", len(batchIDs)).Msg("No valid UIDs found for batch")
			continue
		}

		// Step 4: Fetch bodies via IMAP - single round-trip for all messages in batch
		bodies, fetchErr := e.fetchMessageBodiesBatch(ctx, conn.Client().RawClient(), uidToMessageID)
		if err := ctx.Err(); err != nil {
			e.log.Debug().Err(err).Msg("Batch body fetch cancelled")
			e.pool.Discard(conn)
			return err
		}
		if errors.Is(fetchErr, errStreamBroken) {
			// Keep the partial results (retrying would refetch the cut
			// message) and continue on a fresh connection.
			if conn, err = e.replaceConn(ctx, conn, accountID, f.Path); err != nil {
				return fmt.Errorf("after broken stream: %w", err)
			}
			fetchErr = nil
		}
		if fetchErr != nil {
			// Check if this is a connection error
			if imapPkg.IsConnectionError(fetchErr) {
				connectionFailures++

				// Check if we've exhausted connection recovery attempts
				if connectionFailures > maxConnectionRetries {
					e.log.Error().
						Int("connectionFailures", connectionFailures).
						Msg("Body fetch aborted - connection recovery failed")
					e.pool.Discard(conn)
					return fmt.Errorf("connection recovery failed after %d attempts", connectionFailures)
				}

				e.log.Debug().
					Err(fetchErr).
					Int("attempt", connectionFailures).
					Msg("Connection error during batch fetch, attempting recovery")

				if conn, err = e.replaceConn(ctx, conn, accountID, f.Path); err != nil {
					return fmt.Errorf("after connection error: %w", err)
				}

				e.log.Debug().Msg("Connection recovered successfully, retrying batch")
				continue // Retry same batch
			}

			// Non-connection error
			e.log.Warn().Err(fetchErr).Msg("Batch fetch failed with non-connection error")
			failedBatches++
			if failedBatches > maxMessageRetries {
				e.log.Error().Int("failedBatches", failedBatches).Msg("Too many consecutive batch failures")
				break
			}
			continue
		}

		// Reset failure counters on success
		failedBatches = 0

		// If we got no bodies back, persist the failure for every requested
		// ID so they are excluded from future syncs forever — otherwise the
		// next cycle would query and FETCH the same UIDs again.
		if len(bodies) == 0 {
			e.log.Warn().Int("requested", len(uidToMessageID)).Msg("IMAP returned no bodies for batch")
			// No size signals here — server returned nothing, so every ID
			// falls through to the "no signal" branch of shouldChargeFailure
			// and gets charged. Pass nil to keep the call sites uniform.
			e.markUnresolvedAsFailed(batchIDs, nil, nil)
			failed += len(uidToMessageID)
			continue
		}

		// Step 5: Launch goroutine to build body updates
		// DB update will happen in step 2 of the NEXT iteration
		// Attachments were already extracted during parsing - no re-parse needed!
		resultChan := make(chan processingResult, 1)
		currentBodies := bodies // capture for goroutine

		go func() {
			startTime := time.Now()
			var bodyUpdates []message.BodyUpdate
			var allAttachments []*message.Attachment
			sizes := make(map[string]fetchedSize, len(currentBodies))

			fetchedCount := 0
			for _, pb := range currentBodies {
				sizes[pb.MessageID] = fetchedSize{received: pb.ReceivedBytes, reported: pb.ReportedSize}
				if pb.Incomplete {
					continue
				}
				fetchedCount++

				// Build body update
				bu := message.BodyUpdate{
					MessageID:      pb.MessageID,
					BodyHTML:       pb.BodyHTML,
					BodyText:       pb.BodyText,
					Snippet:        pb.Snippet,
					HasAttachments: pb.HasAttachments,
					SMIMERawBody:   pb.SMIMERawBody,
					SMIMEEncrypted: pb.SMIMEEncrypted,
					PGPRawBody:     pb.PGPRawBody,
					PGPEncrypted:   pb.PGPEncrypted,
				}
				// Don't cache S/MIME or PGP verification status — computed fresh on each view
				bodyUpdates = append(bodyUpdates, bu)

				// Use pre-extracted attachments (no re-parsing!)
				if len(pb.Attachments) > 0 {
					allAttachments = append(allAttachments, pb.Attachments...)
				}
			}

			e.log.Debug().
				Int("bodyUpdates", len(bodyUpdates)).
				Int("attachments", len(allAttachments)).
				Dur("elapsed", time.Since(startTime)).
				Msg("Built body updates and attachments for batch")

			resultChan <- processingResult{
				requestedIDs: batchIDs,
				bodyUpdates:  bodyUpdates,
				attachments:  allAttachments,
				sizes:        sizes,
				fetchedCount: fetchedCount,
			}
		}()

		// Mark that we have pending work - will be processed in step 1-2 of next iteration
		pendingResultChan = resultChan
	}

	// Handle final batch if there's pending work
	if pendingResultChan != nil {
		result := <-pendingResultChan

		// Persist parse failures for the final batch via the same path as
		// every other batch — see markUnresolvedAsFailed.
		e.markUnresolvedAsFailed(result.requestedIDs, result.bodyUpdates, result.sizes)

		if len(result.bodyUpdates) > 0 {
			if err := e.messageStore.UpdateBodiesBatch(result.bodyUpdates); err != nil {
				e.log.Warn().Err(err).Msg("Failed to batch update bodies (final)")
				failed += result.fetchedCount
			} else {
				fetched += result.fetchedCount
				e.changeSeq.bump(folderID)
			}
		}
		if len(result.bodyUpdates) > 0 {
			if err := e.attachmentStore.ReplaceForMessages(bodyUpdateIDs(result.bodyUpdates), result.attachments); err != nil {
				e.log.Warn().Err(err).Msg("Failed to batch create attachments (final)")
			}
		}

		e.emitProgress(accountID, folderID, fetched, totalWithoutBody, "bodies")
	}

	// Release connection when done
	e.pool.Release(conn)

	// Log summary
	if failed > 0 {
		e.log.Info().
			Int("fetched", fetched).
			Int("failed", failed).
			Int("total", totalWithoutBody).
			Msg("Body fetch complete with failures (hybrid batch mode)")
	} else {
		e.log.Info().
			Int("fetched", fetched).
			Int("total", totalWithoutBody).
			Msg("Body fetch complete (hybrid batch mode)")
	}

	return nil
}

// FetchRawMessage fetches the raw RFC822 content of a message from the IMAP server.
// Uses streaming fetch to avoid blocking on .Collect().
func (e *Engine) FetchRawMessage(ctx context.Context, accountID, folderID string, uid uint32) ([]byte, error) {
	// Get folder path
	f, err := e.folderStore.Get(folderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get folder: %w", err)
	}
	if f == nil {
		return nil, fmt.Errorf("folder not found: %s", folderID)
	}

	// Get a connection from the pool
	conn, err := e.pool.GetConnection(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer e.pool.Release(conn)

	// Select the mailbox
	_, err = conn.Client().SelectMailbox(ctx, f.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to select mailbox: %w", err)
	}

	// Fetch the raw message
	uidSet := imap.UIDSet{}
	uidSet.AddNum(imap.UID(uid))

	fetchOptions := &imap.FetchOptions{
		BodySection: []*imap.FetchItemBodySection{
			{
				Specifier: imap.PartSpecifierNone,
				Peek:      true,
			},
		},
	}

	fetchCmd := conn.Client().RawClient().Fetch(uidSet, fetchOptions)

	// Stream the single message instead of blocking on Collect()
	var rawBytes []byte

	msg := fetchCmd.Next()
	if msg == nil {
		fetchCmd.Close()
		return nil, fmt.Errorf("message not found: UID %d", uid)
	}

	// Extract body section from streamed message
	for {
		item := msg.Next()
		if item == nil {
			break
		}

		if data, ok := item.(imapclient.FetchItemDataBodySection); ok {
			if data.Literal != nil {
				rawBytes, err = readLiteral(data.Literal, maxMessageSize)
				if err != nil {
					// Don't consume the command further (see readLiteral).
					conn.Client().RawClient().Close()
					return nil, fmt.Errorf("failed to read message body: %w", err)
				}
				break
			}
		}
	}

	fetchCmd.Close()

	if len(rawBytes) == 0 {
		return nil, fmt.Errorf("message body not found: UID %d", uid)
	}

	return rawBytes, nil
}

// bodyUpdateIDs returns the message IDs of a batch of body updates.
func bodyUpdateIDs(updates []message.BodyUpdate) []string {
	ids := make([]string, len(updates))
	for i, u := range updates {
		ids[i] = u.MessageID
	}
	return ids
}
