package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/mail"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-message/textproto"

	imapPkg "github.com/hkdb/aerion/internal/imap"
	"github.com/hkdb/aerion/internal/message"
)

// forwardingHeaderFields name the headers that show which address a message
// was originally sent to when it reached the mailbox through a forward, most
// specific first: Delivered-To is the final mailbox (the Gmail address itself
// after a Workspace forward), so it comes last.
var forwardingHeaderFields = []string{"X-Forwarded-For", "X-Original-To", "Delivered-To"}

// forwardingAddresses extracts the deduplicated, lowercased addresses of the
// forwarding headers from raw header bytes. Nil when there are no headers to
// read; an empty non-nil slice when the headers carry no addresses.
func forwardingAddresses(rawHeader []byte) []string {
	h, ok := readRawHeader(rawHeader)
	if !ok {
		return nil
	}
	return headerForwardingAddresses(h)
}

// headerForwardingAddresses extracts the forwarding addresses of a parsed
// header; never nil.
func headerForwardingAddresses(h textproto.Header) []string {
	addrs := []string{}
	seen := map[string]bool{}
	add := func(a string) {
		a = strings.ToLower(strings.Trim(strings.TrimSpace(a), "<>,;"))
		if a == "" || !strings.Contains(a, "@") || seen[a] {
			return
		}
		seen[a] = true
		addrs = append(addrs, a)
	}
	for _, name := range forwardingHeaderFields {
		for fields := h.FieldsByKey(name); fields.Next(); {
			v := fields.Value()
			if parsed, err := mail.ParseAddressList(v); err == nil {
				for _, p := range parsed {
					add(p.Address)
				}
				continue
			}
			// X-Forwarded-For is a space-separated list of bare addresses.
			for _, part := range strings.Fields(v) {
				add(part)
			}
		}
	}
	return addrs
}

// encodeAddrs encodes addresses as the stored delivered_to JSON list.
func encodeAddrs(addrs []string) string {
	if addrs == nil {
		addrs = []string{}
	}
	b, _ := json.Marshal(addrs)
	return string(b)
}

// ResolveForwardingAddresses returns the forwarding-header addresses of a
// message. A stored list is used as is; for messages synced before the column
// existed (NULL) the headers are fetched for that UID, stored, and returned.
func ResolveForwardingAddresses(ctx context.Context, store *message.Store, pool *imapPkg.Pool, folderPath string, m *message.Message) ([]string, error) {
	stored, err := store.GetDeliveredTo(m.ID)
	if err != nil {
		return nil, err
	}
	if stored != nil {
		var addrs []string
		if err := json.Unmarshal([]byte(*stored), &addrs); err != nil {
			return nil, fmt.Errorf("invalid delivered_to: %w", err)
		}
		return addrs, nil
	}

	raw, err := FetchForwardingHeaders(ctx, pool, m.AccountID, folderPath, m.UID)
	if err != nil {
		return nil, err
	}
	// Record "none" for a block without addresses rather than refetching on
	// every reply.
	addrs := forwardingAddresses(raw)
	if addrs == nil {
		addrs = []string{}
	}
	if err := store.SetDeliveredTo(m.ID, encodeAddrs(addrs)); err != nil {
		return nil, err
	}
	return addrs, nil
}

// FetchForwardingHeaders fetches only the forwarding header fields of one
// message from the server.
func FetchForwardingHeaders(ctx context.Context, pool *imapPkg.Pool, accountID, folderPath string, uid uint32) ([]byte, error) {
	conn, err := pool.GetConnection(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer pool.Release(conn)
	if _, err := conn.Client().SelectMailbox(ctx, folderPath); err != nil {
		return nil, fmt.Errorf("failed to select mailbox: %w", err)
	}

	var uids imap.UIDSet
	uids.AddNum(imap.UID(uid))
	headers, err := fetchHeaderFields(ctx, conn.Client().RawClient(), uids, forwardingHeaderFields)
	if err != nil {
		return nil, err
	}
	raw, ok := headers[uid]
	if !ok {
		return nil, fmt.Errorf("message not found: UID %d", uid)
	}
	return raw, nil
}
