package graphapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/credentials"
	"github.com/caseylabs/mailven/internal/smtp"
	"github.com/caseylabs/mailven/internal/submission"
)

type staticToken struct{}

func (staticToken) Token(context.Context) (string, error)           { return "tok", nil }
func (staticToken) Refresh(context.Context, string) (string, error) { return "tok2", nil }

func capture(t *testing.T, status int) (*[]byte, *string) {
	t.Helper()
	var got []byte
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got, _ = base64.StdEncoding.DecodeString(string(b))
		path = r.URL.EscapedPath()
		if r.Header.Get("Content-Type") != "text/plain" || r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("headers %v", r.Header)
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	old := graphBase
	graphBase = srv.URL
	t.Cleanup(func() { graphBase = old })
	return &got, &path
}

func send(req submission.Request) (smtp.SubmissionResult, error) {
	return (&Sender{Tokens: staticToken{}, Own: "me@contoso.com"}).Send(context.Background(), req, func(smtp.SubmissionResult) error { return nil })
}

func TestSendMIMEPassthroughAndPath(t *testing.T) {
	got, path := capture(t, 202)
	raw := []byte("From: me@contoso.com\r\nTo: zoé@例え.jp\r\nContent-Type: application/pkcs7-mime; smime-type=enveloped-data\r\n\r\n\xff\x00\r\n")
	result, err := send(submission.Request{From: "ME@contoso.com", Recipients: []string{"zoé@例え.jp"}, Raw: raw})
	if err != nil || result.Outcome != smtp.OutcomeAccepted || !bytes.Equal(*got, raw) || *path != "/me/sendMail" {
		t.Fatalf("%v %v path %s equal %v", result, err, *path, bytes.Equal(*got, raw))
	}
}

func TestSendSharedMailboxPath(t *testing.T) {
	_, path := capture(t, 202)
	_, err := send(submission.Request{From: "shared@contoso.com", Recipients: []string{"b@example.com"}, Raw: []byte("To: b@example.com\r\n\r\n")})
	if err != nil || *path != "/users/shared@contoso.com/sendMail" {
		t.Fatalf("%v %s", err, *path)
	}
}

func TestSendHiddenBccOnly(t *testing.T) {
	got, _ := capture(t, 202)
	raw := []byte("To: b@example.com\r\n\r\nx\r\n")
	if _, err := send(submission.Request{From: "me@contoso.com", Recipients: []string{"b@example.com", "bcc@example.com"}, Raw: raw}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(*got), "Bcc: bcc@example.com\r\nTo: b@example.com") || strings.Count(string(*got), "bcc@example.com") != 1 {
		t.Fatalf("%q", *got)
	}
}

func TestSendOutcomes(t *testing.T) {
	for status, want := range map[int]smtp.Outcome{400: smtp.OutcomeRejectedPermanent, 403: smtp.OutcomeRejectedPermanent, 429: smtp.OutcomeUnknown, 504: smtp.OutcomeUnknown} {
		capture(t, status)
		result, err := send(submission.Request{From: "me@contoso.com", Recipients: []string{"b@example.com"}, Raw: []byte("To: b@example.com\r\n\r\n")})
		if err == nil || result.Outcome != want {
			t.Errorf("%d: %v %v", status, result, err)
		}
	}
}

func tokens(scopes ...string) *credentials.OAuthTokens {
	return &credentials.OAuthTokens{Provider: "microsoft", ScopesKnown: true, ScopeResource: "https://graph.microsoft.com", Scopes: scopes}
}

func TestPreflight(t *testing.T) {
	route := func(shared bool) *account.MailAPIRoute {
		return &account.MailAPIRoute{Route: account.MailRouteGraphAPI, State: account.MailRouteStateStaged, Shared: shared}
	}
	own := tokens("https://graph.microsoft.com/Mail.Send")
	shared := tokens("https://graph.microsoft.com/Mail.Send.Shared")
	req := func(from string, n int) submission.Request {
		return submission.Request{From: from, Raw: make([]byte, n)}
	}
	tests := []struct {
		name   string
		route  *account.MailAPIRoute
		tokens *credentials.OAuthTokens
		req    submission.Request
		ok     bool
	}{
		{"own", route(false), own, req("ME@contoso.com", 1), true},
		{"shared needs shared grant", route(true), own, req("other@contoso.com", 1), false},
		{"shared needs shared route", route(false), shared, req("other@contoso.com", 1), false},
		{"shared ok", route(true), shared, req("other@contoso.com", 1), true},
		{"no send", route(false), tokens("https://graph.microsoft.com/Mail.Read"), req("me@contoso.com", 1), false},
		{"nil route", nil, own, req("me@contoso.com", 1), false},
		{"over limit", route(false), own, req("me@contoso.com", MaxRawBytes+1), false},
		{"at limit", route(false), own, req("me@contoso.com", MaxRawBytes), true},
	}
	for _, tt := range tests {
		if err := Preflight(tt.route, tt.tokens, "me@contoso.com", tt.req); (err == nil) != tt.ok {
			t.Errorf("%s: %v", tt.name, err)
		}
	}
}
