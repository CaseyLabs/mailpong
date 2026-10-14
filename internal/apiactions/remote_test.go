package apiactions

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caseylabs/mailven/internal/credentials"
	"github.com/caseylabs/mailven/internal/gmailapi"
	"github.com/caseylabs/mailven/internal/message"
)

type tokens struct{}

func (tokens) Token(context.Context) (string, error)           { return "t", nil }
func (tokens) Refresh(context.Context, string) (string, error) { return "t", nil }

func TestGmailRemoteRequests(t *testing.T) {
	cases := []struct {
		name string
		a    message.APIAction
		want []string
	}{
		{"mark read removes UNREAD", message.APIAction{Op: message.APIActionRead, RemoteID: "m",
			Payload: message.APIActionPayload{Value: true}}, []string{`/messages/m/modify {"addLabelIds":[],"removeLabelIds":["UNREAD"]}`}},
		{"unstar removes STARRED", message.APIAction{Op: message.APIActionStar, RemoteID: "m"},
			[]string{`/messages/m/modify {"addLabelIds":[],"removeLabelIds":["STARRED"]}`}},
		{"trash uses the trash verb", message.APIAction{Op: message.APIActionMove, RemoteID: "m",
			Payload: message.APIActionPayload{Trash: true}}, []string{`/messages/m/trash `}},
		{"restore untrashes then labels", message.APIAction{Op: message.APIActionMove, RemoteID: "m",
			Payload: message.APIActionPayload{FromTrash: true, Source: "TRASH", Dest: "INBOX"}},
			[]string{`/messages/m/untrash `, `/messages/m/modify {"addLabelIds":["INBOX"],"removeLabelIds":[]}`}},
		{"archive removes INBOX", message.APIAction{Op: message.APIActionMove, RemoteID: "m",
			Payload: message.APIActionPayload{Source: "INBOX"}}, []string{`/messages/m/modify {"addLabelIds":[],"removeLabelIds":["INBOX"]}`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b, _ := io.ReadAll(r.Body)
				got = append(got, strings.TrimSpace(r.URL.Path+" "+string(b)))
				_, _ = w.Write([]byte(`{"id":"m"}`))
			}))
			defer srv.Close()
			g := &Gmail{Writer: &gmailapi.Writer{Tokens: tokens{}, Base: srv.URL, Grant: &credentials.OAuthTokens{
				Provider: "google", ScopeResource: "https://www.googleapis.com", ScopesKnown: true,
				Scopes: []string{"https://www.googleapis.com/auth/gmail.modify"}}}}
			if _, err := g.Apply(context.Background(), &tc.a); err != nil {
				t.Fatal(err)
			}
			var want []string
			for _, w := range tc.want {
				want = append(want, strings.TrimSpace(w))
			}
			if strings.Join(got, "|") != strings.Join(want, "|") {
				t.Fatalf("requests = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGmailMoveVerifyRequiresSourceLabelRemoval(t *testing.T) {
	labels := []string{"DEST", "SOURCE"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "m", "labelIds": labels, "historyId": "1"})
	}))
	defer srv.Close()
	g := &Gmail{Reader: &gmailapi.Reader{Tokens: tokens{}, Base: srv.URL}}
	a := &message.APIAction{Op: message.APIActionMove, RemoteID: "m", Payload: message.APIActionPayload{Dest: "DEST", Source: "SOURCE"}}
	got, err := g.Verify(context.Background(), a)
	if err != nil || got != Differs {
		t.Fatalf("Verify with both labels = %v, %v; want Differs", got, err)
	}
	labels = []string{"DEST"}
	got, err = g.Verify(context.Background(), a)
	if err != nil || got != Matches {
		t.Fatalf("Verify after source removal = %v, %v; want Matches", got, err)
	}
}
