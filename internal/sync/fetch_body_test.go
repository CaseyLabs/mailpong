package sync

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/caseylabs/mailven/internal/email"
	"github.com/caseylabs/mailven/internal/logging"
)

// scriptedFetchClient returns a client whose server answers the first
// command with the given untagged FETCH responses, then closes the
// connection without a tagged reply, as a dropped connection would.
func scriptedFetchClient(t *testing.T, responses string) *imapclient.Client {
	t.Helper()
	clientConn, serverConn := net.Pipe()
	go func() {
		defer serverConn.Close()
		fmt.Fprint(serverConn, "* OK [CAPABILITY IMAP4rev1] ready\r\n")
		if _, err := bufio.NewReader(serverConn).ReadString('\n'); err != nil {
			return
		}
		fmt.Fprint(serverConn, responses)
	}()
	client := imapclient.New(clientConn, nil)
	t.Cleanup(func() { client.Close() })
	return client
}

func TestFetchMessageBodiesBatch_IncompleteBodies(t *testing.T) {
	full := "From: a@example.com\r\nSubject: hi\r\nContent-Type: text/plain\r\n\r\nhello\r\n"
	responses := strings.Join([]string{
		"* 1 FETCH (UID 1 RFC822.SIZE 100)\r\n", // no body section
		"* 2 FETCH (UID 2 RFC822.SIZE 100 BODY[] {0}\r\n)\r\n",
		fmt.Sprintf("* 3 FETCH (UID 3 RFC822.SIZE %d BODY[] {%d}\r\n%s)\r\n", len(full), len(full), full),
		"* 4 FETCH (UID 4 RFC822.SIZE 100 BODY[] {100}\r\nFrom: a@exa", // cut off mid-literal
	}, "")
	client := scriptedFetchClient(t, responses)
	e := &Engine{sanitizer: email.NewSanitizer(), attachExtractor: email.NewAttachmentExtractor(), log: logging.WithComponent("sync-test")}

	results, err := e.fetchMessageBodiesBatch(context.Background(), client, map[uint32]string{1: "none", 2: "empty", 3: "full", 4: "cut"})
	if !errors.Is(err, errStreamBroken) {
		t.Fatalf("fetchMessageBodiesBatch error = %v, want errStreamBroken", err)
	}

	tests := []struct {
		uid            uint32
		wantIncomplete bool
	}{
		{uid: 1, wantIncomplete: true},
		{uid: 2, wantIncomplete: true},
		{uid: 3, wantIncomplete: false},
		{uid: 4, wantIncomplete: true},
	}
	for _, tc := range tests {
		r, ok := results[tc.uid]
		if !ok {
			t.Errorf("uid %d missing from results", tc.uid)
			continue
		}
		if r.Incomplete != tc.wantIncomplete {
			t.Errorf("uid %d Incomplete = %v, want %v", tc.uid, r.Incomplete, tc.wantIncomplete)
		}
		if tc.wantIncomplete && (r.BodyText != "" || r.BodyHTML != "") {
			t.Errorf("uid %d incomplete result carries a body", tc.uid)
		}
		if !tc.wantIncomplete && !strings.Contains(r.BodyText, "hello") {
			t.Errorf("uid %d body text = %q, want the message body", tc.uid, r.BodyText)
		}
	}
}
