package sync

import (
	"context"
	"testing"

	"github.com/caseylabs/mailpong/internal/logging"
)

func TestFolderChangeSeq(t *testing.T) {
	const header = "Subject: hi\r\nMessage-ID: <a@example.com>\r\n\r\n"
	const envelope = `ENVELOPE (NIL "hi" NIL NIL NIL NIL NIL NIL NIL "<a@example.com>") `
	tests := []struct {
		name      string
		responses string
		changed   bool
	}{
		{"no-op fetch", "", false},
		{"header stored", headerFetch(1, envelope, header), true},
		{"literal cut off", "* 1 FETCH (UID 1 RFC822.SIZE 100 " + envelope + "BODY[HEADER] {100}\r\nSubj", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Engine{messageStore: newTestMessageStore(t), log: logging.WithComponent("sync-test")}
			before := e.FolderChangeSeq("inbox-1")
			// The scripted server hangs up after its responses, so an
			// error is expected; only the stored headers matter here.
			_ = e.fetchMessageHeaders(context.Background(), scriptedFetchClient(t, tt.responses), "acct-1", "inbox-1", []uint32{1})
			if got := e.FolderChangeSeq("inbox-1") != before; got != tt.changed {
				t.Errorf("changed = %v, want %v", got, tt.changed)
			}
			if e.FolderChangeSeq("other") != 0 {
				t.Error("another folder's counter moved")
			}
		})
	}
}
