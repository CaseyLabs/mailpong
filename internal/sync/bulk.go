package sync

import (
	"bufio"
	"bytes"
	"context"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/textproto"

	"github.com/caseylabs/mailven/internal/message"
)

// bulkHeaderFields are the headers classifyBulk reads; the backfill fetches
// only these.
var bulkHeaderFields = []string{"List-Unsubscribe", "List-Id", "Precedence", "Auto-Submitted"}

// bulkBackfillBatch is how many messages one backfill FETCH covers.
const bulkBackfillBatch = 500

// classifyBulk reports whether a message is newsletter, list, or automated
// mail rather than a person writing: list headers, a bulk Precedence, an
// Auto-Submitted value other than "no", or a no-reply style sender.
func classifyBulk(h textproto.Header, fromEmail string) bool {
	if strings.TrimSpace(h.Get("List-Unsubscribe")) != "" || strings.TrimSpace(h.Get("List-Id")) != "" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(h.Get("Precedence"))) {
	case "bulk", "list", "junk":
		return true
	}
	auto, _, _ := strings.Cut(h.Get("Auto-Submitted"), ";")
	if auto = strings.ToLower(strings.TrimSpace(auto)); auto != "" && auto != "no" {
		return true
	}
	return isNoReplySender(fromEmail)
}

// isNoReplySender matches noreply/no-reply/do-not-reply style local parts
// (with any separators and suffixes) and notification@ senders.
func isNoReplySender(email string) bool {
	local, _, found := strings.Cut(strings.ToLower(strings.TrimSpace(email)), "@")
	if !found {
		return false
	}
	switch local {
	case "notification", "notifications":
		return true
	}
	squashed := strings.NewReplacer("-", "", "_", "", ".", "").Replace(local)
	return strings.HasPrefix(squashed, "noreply") || strings.HasPrefix(squashed, "donotreply")
}

// readRawHeader parses raw header bytes; false when there are no headers to
// read.
func readRawHeader(rawHeader []byte) (textproto.Header, bool) {
	if len(rawHeader) == 0 {
		return textproto.Header{}, false
	}
	h, err := textproto.ReadHeader(bufio.NewReader(bytes.NewReader(rawHeader)))
	if err != nil && h.Len() == 0 {
		return textproto.Header{}, false
	}
	return h, true
}

// bulkFlag classifies raw header bytes; nil when there are no headers to read.
func bulkFlag(rawHeader []byte, fromEmail string) *bool {
	h, ok := readRawHeader(rawHeader)
	if !ok {
		return nil
	}
	bulk := classifyBulk(h, fromEmail)
	return &bulk
}

// applyHeaderFlags sets the header-derived IsBulk and DeliveredTo of m,
// parsing rawHeader once; both stay nil when there are no headers to read.
func applyHeaderFlags(m *message.Message, rawHeader []byte) {
	m.IsBulk, m.DeliveredTo = nil, nil
	h, ok := readRawHeader(rawHeader)
	if !ok {
		return
	}
	bulk := classifyBulk(h, m.FromEmail)
	m.IsBulk = &bulk
	delivered := encodeAddrs(headerForwardingAddresses(h))
	m.DeliveredTo = &delivered
}

// backfillBulkFlags classifies messages in the selected mailbox whose is_bulk
// is still NULL (synced before classification existed), fetching only the
// list/auto headers in batches. Runs under the caller's folder lock and stops
// on cancellation; unfinished rows stay NULL for the next run.
func (e *Engine) backfillBulkFlags(ctx context.Context, client *imapclient.Client, folderID string) error {
	var afterUID uint32
	classified := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		pending, err := e.messageStore.ListUnclassifiedBulk(folderID, afterUID, bulkBackfillBatch)
		if err != nil {
			return err
		}
		if len(pending) == 0 {
			break
		}
		afterUID = pending[len(pending)-1].UID

		fromByUID := make(map[uint32]string, len(pending))
		idByUID := make(map[uint32]string, len(pending))
		var uidSet imap.UIDSet
		for _, m := range pending {
			fromByUID[m.UID] = m.FromEmail
			idByUID[m.UID] = m.ID
			uidSet.AddNum(imap.UID(m.UID))
		}

		headers, err := fetchHeaderFields(ctx, client, uidSet, bulkHeaderFields)
		if err != nil {
			return err
		}
		// Messages the server didn't return (expunged, or UIDs from a
		// local move) are classified by sender alone so they aren't
		// refetched every sync.
		flags := make(map[string]bool, len(pending))
		for uid, id := range idByUID {
			if b := bulkFlag(headers[uid], fromByUID[uid]); b != nil {
				flags[id] = *b
				continue
			}
			flags[id] = isNoReplySender(fromByUID[uid])
		}
		if err := e.messageStore.SetBulkFlags(flags); err != nil {
			return err
		}
		classified += len(flags)
	}
	if classified > 0 {
		e.log.Info().Str("folderID", folderID).Int("count", classified).Msg("Backfilled bulk-mail classification")
	}
	return nil
}

// fetchHeaderFields fetches BODY.PEEK[HEADER.FIELDS (...)] for uids, keyed by UID.
func fetchHeaderFields(ctx context.Context, client *imapclient.Client, uids imap.UIDSet, fields []string) (map[uint32][]byte, error) {
	cmd := client.Fetch(uids, &imap.FetchOptions{
		UID: true,
		BodySection: []*imap.FetchItemBodySection{{
			Specifier:    imap.PartSpecifierHeader,
			HeaderFields: fields,
			Peek:         true,
		}},
	})
	out := make(map[uint32][]byte)
	for {
		if ctx.Err() != nil {
			_ = cmd.Close()
			return nil, ctx.Err()
		}
		msg := cmd.Next()
		if msg == nil {
			break
		}
		var uid imap.UID
		var raw []byte
		for item := msg.Next(); item != nil; item = msg.Next() {
			switch data := item.(type) {
			case imapclient.FetchItemDataUID:
				uid = data.UID
			case imapclient.FetchItemDataBodySection:
				if data.Literal == nil {
					continue
				}
				// A short read would classify the message from partial
				// headers; fail the batch so its rows stay NULL, and don't
				// consume the command further (see readLiteral).
				var err error
				if raw, err = readLiteral(data.Literal, maxMessageSize); err != nil {
					client.Close()
					return nil, err
				}
			}
		}
		if uid != 0 {
			out[uint32(uid)] = raw
		}
	}
	if err := cmd.Close(); err != nil {
		return nil, err
	}
	return out, nil
}
