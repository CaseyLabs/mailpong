package sync

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emersion/go-imap/v2"

	"github.com/caseylabs/mailpong/internal/database"
	"github.com/caseylabs/mailpong/internal/logging"
	"github.com/caseylabs/mailpong/internal/message"
)

// newTestMessageStore opens a migrated database with account acct-1 and
// folder inbox-1, and returns a message store on it.
func newTestMessageStore(t *testing.T) *message.Store {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO accounts (id, name, email, imap_host, smtp_host, username)
		VALUES ('acct-1', 'Test', 'me@example.com', 'imap.example.com', 'smtp.example.com', 'me')`); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO folders (id, account_id, name, path, folder_type)
		VALUES ('inbox-1', 'acct-1', 'INBOX', 'INBOX', 'inbox')`); err != nil {
		t.Fatalf("seed folder: %v", err)
	}
	return message.NewStore(db)
}

func headerFetch(uid int, envelope, header string) string {
	return fmt.Sprintf("* %d FETCH (UID %d RFC822.SIZE 100 %sBODY[HEADER] {%d}\r\n%s)\r\n",
		uid, uid, envelope, len(header), header)
}

// TestHeaderFetchStopsAfterBrokenLiteral checks that the header readers
// return an error, rather than partial headers, when a literal is cut off.
func TestHeaderFetchStopsAfterBrokenLiteral(t *testing.T) {
	const header = "Subject: hi\r\nMessage-ID: <a@example.com>\r\n\r\n"
	const envelope = `ENVELOPE (NIL "hi" NIL NIL NIL NIL NIL NIL NIL "<a@example.com>") `
	cut := "* 2 FETCH (UID 2 RFC822.SIZE 100 " + envelope + "BODY[HEADER] {100}\r\nSubject: cu"

	tests := []struct {
		name      string
		responses string
		wantSaved []uint32
	}{
		{"cut off first", cut, nil},
		{"cut off after a full message", headerFetch(1, envelope, header) + cut, []uint32{1}},
	}
	for _, tt := range tests {
		t.Run("fetchHeaderFields/"+tt.name, func(t *testing.T) {
			client := scriptedFetchClient(t, tt.responses)
			var uids imap.UIDSet
			uids.AddRange(1, 2)
			out, err := fetchHeaderFields(context.Background(), client, uids, bulkHeaderFields)
			if err == nil || out != nil {
				t.Fatalf("fetchHeaderFields = %v, %v; want nil map and an error", out, err)
			}
		})
		t.Run("fetchMessageHeaders/"+tt.name, func(t *testing.T) {
			store := newTestMessageStore(t)
			e := &Engine{messageStore: store, log: logging.WithComponent("sync-test")}
			client := scriptedFetchClient(t, tt.responses)
			err := e.fetchMessageHeaders(context.Background(), client, "acct-1", "inbox-1", []uint32{1, 2})
			if !errors.Is(err, errStreamBroken) {
				t.Fatalf("fetchMessageHeaders error = %v, want errStreamBroken", err)
			}
			for _, uid := range []uint32{1, 2} {
				m, err := store.GetByUID("inbox-1", uid)
				if err != nil {
					t.Fatal(err)
				}
				want := len(tt.wantSaved) > 0 && tt.wantSaved[0] == uid
				if got := m != nil; got != want {
					t.Errorf("uid %d saved = %v, want %v", uid, got, want)
				}
			}
		})
	}
}

// fakeLiteral is a literal whose declared size can differ from its content.
type fakeLiteral struct {
	io.Reader
	size int64
}

func (l fakeLiteral) Size() int64 { return l.size }

func TestReadLiteral(t *testing.T) {
	tests := []struct {
		name    string
		content string
		size    int64
		limit   int64
		want    string
		wantErr bool
	}{
		{"complete", "hello", 5, 100, "hello", false},
		{"short", "hel", 5, 100, "hel", true},
		{"capped at limit", "hello", 5, 3, "hel", false},
		{"short below limit", "he", 5, 3, "he", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readLiteral(fakeLiteral{strings.NewReader(tt.content), tt.size}, tt.limit)
			if string(got) != tt.want || (err != nil) != tt.wantErr {
				t.Fatalf("readLiteral = %q, %v; want %q, error %v", got, err, tt.want, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Errorf("error %v does not wrap io.ErrUnexpectedEOF", err)
			}
		})
	}
}
