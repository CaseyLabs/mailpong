package gmailapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/credentials"
	"github.com/caseylabs/mailven/internal/smtp"
	"github.com/caseylabs/mailven/internal/submission"
)

type staticToken struct{}

func (staticToken) Token(context.Context) (string, error)           { return "tok", nil }
func (staticToken) Refresh(context.Context, string) (string, error) { return "tok2", nil }

func capture(t *testing.T, status int) (*httptest.Server, *[]byte) {
	t.Helper()
	var got []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Raw string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		got, _ = base64.RawURLEncoding.DecodeString(body.Raw)
		if r.Header.Get("Authorization") != "Bearer tok" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("headers %v", r.Header)
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	old := sendURL
	sendURL = srv.URL
	t.Cleanup(func() { sendURL = old })
	return srv, &got
}

func send(t *testing.T, req submission.Request) (smtp.SubmissionResult, error) {
	t.Helper()
	return (&Sender{Tokens: staticToken{}}).Send(context.Background(), req, func(smtp.SubmissionResult) error { return nil })
}

func TestSendPassesMIMEThroughByteIdentical(t *testing.T) {
	_, got := capture(t, 200)
	// Signed/encrypted-looking and international content, binary-safe lines.
	raw := []byte("From: a@example.com\r\nTo: zoé@例え.jp\r\nSubject: =?utf-8?b?5pel5pys?=\r\nContent-Type: multipart/signed; protocol=\"application/pgp-signature\"; boundary=b\r\n\r\n--b\r\n\xff\x00 raw\r\n--b--\r\n")
	result, err := send(t, submission.Request{From: "a@example.com", Recipients: []string{"zoé@例え.jp"}, Raw: raw})
	if err != nil || result.Outcome != smtp.OutcomeAccepted {
		t.Fatalf("%v %v", result, err)
	}
	if !bytes.Equal(*got, raw) {
		t.Fatalf("sent MIME differs:\n%q\n%q", *got, raw)
	}
}

func TestSendAddsOnlyHiddenRecipientsAsBcc(t *testing.T) {
	_, got := capture(t, 200)
	raw := []byte("From: a@example.com\r\nTo: b@example.com\r\nCc: c@example.com\r\n\r\nhi\r\n")
	if _, err := send(t, submission.Request{From: "a@example.com", Recipients: []string{"b@example.com", "c@example.com", "hide1@example.com", "hide2@example.com"}, Raw: raw}); err != nil {
		t.Fatal(err)
	}
	head := strings.SplitN(string(*got), "\r\n", 2)
	if head[0] != "Bcc: hide1@example.com, hide2@example.com" || head[1] != string(raw) {
		t.Fatalf("got %q", *got)
	}
}

func TestSendRejectedAndUnknown(t *testing.T) {
	capture(t, 403)
	result, err := send(t, submission.Request{From: "a@example.com", Recipients: []string{"b@example.com"}, Raw: []byte("To: b@example.com\r\n\r\n")})
	if err == nil || result.Outcome != smtp.OutcomeRejectedPermanent {
		t.Fatalf("%v %v", result, err)
	}
	capture(t, 503)
	result, err = send(t, submission.Request{From: "a@example.com", Recipients: []string{"b@example.com"}, Raw: []byte("To: b@example.com\r\n\r\n")})
	if err == nil || result.Outcome != smtp.OutcomeUnknown {
		t.Fatalf("%v %v", result, err)
	}
}

func TestSendCancelledNothingSent(t *testing.T) {
	srv, _ := capture(t, 200)
	srv.Config.Handler = http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("sent") })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := (&Sender{Tokens: staticToken{}}).Send(ctx, submission.Request{From: "a@example.com", Raw: []byte("To: b@example.com\r\n\r\n"), Recipients: []string{"b@example.com"}}, func(smtp.SubmissionResult) error { return nil })
	if err == nil || result.Outcome != smtp.OutcomeNotSubmitted {
		t.Fatalf("%v %v", result, err)
	}
}

func goodTokens() *credentials.OAuthTokens {
	return &credentials.OAuthTokens{Provider: "google", ScopesKnown: true, ScopeResource: "https://www.googleapis.com",
		Scopes: []string{"https://www.googleapis.com/auth/gmail.modify"}}
}

func TestPreflight(t *testing.T) {
	route := &account.MailAPIRoute{Route: account.MailRouteGmailAPI, State: account.MailRouteStateStaged}
	now := time.Now()
	cache := Cached{FetchedAt: now, Entries: []SendAs{{Email: "me@gmail.com", IsPrimary: true}, {Email: "alias@example.com", VerificationStatus: "accepted"}, {Email: "pending@example.com", VerificationStatus: "pending"}}}
	req := func(from string, n int) submission.Request {
		return submission.Request{From: from, Raw: make([]byte, n)}
	}
	noSend := goodTokens()
	noSend.Scopes = []string{"openid"}
	failed := cache
	failed.LookupFailed = true
	tests := []struct {
		name   string
		route  *account.MailAPIRoute
		tokens *credentials.OAuthTokens
		cache  Cached
		req    submission.Request
		ok     bool
	}{
		{"primary", route, goodTokens(), cache, req("me@gmail.com", 10), true},
		{"verified alias", route, goodTokens(), cache, req("ALIAS@example.com", 10), true},
		{"pending alias", route, goodTokens(), cache, req("pending@example.com", 10), false},
		{"unknown from", route, goodTokens(), cache, req("other@example.com", 10), false},
		{"lookup failed", route, goodTokens(), failed, req("me@gmail.com", 10), false},
		{"no route", nil, goodTokens(), cache, req("me@gmail.com", 10), false},
		{"wrong route", &account.MailAPIRoute{Route: account.MailRouteGraphAPI, State: "staged"}, goodTokens(), cache, req("me@gmail.com", 10), false},
		{"disabled state", &account.MailAPIRoute{Route: account.MailRouteGmailAPI, State: "off"}, goodTokens(), cache, req("me@gmail.com", 10), false},
		{"no send capability", route, noSend, cache, req("me@gmail.com", 10), false},
		{"unknown grant", route, &credentials.OAuthTokens{Provider: "google"}, cache, req("me@gmail.com", 10), false},
		{"at limit", route, goodTokens(), cache, req("me@gmail.com", MaxRawBytes), true},
		{"over limit", route, goodTokens(), cache, req("me@gmail.com", MaxRawBytes+1), false},
	}
	for _, tt := range tests {
		if err := Preflight(tt.route, tt.tokens, tt.cache, tt.req); (err == nil) != tt.ok {
			t.Errorf("%s: err = %v", tt.name, err)
		}
	}
}

func TestSendLargeAttachmentNearLimit(t *testing.T) {
	_, got := capture(t, 200)
	raw := append([]byte("To: b@example.com\r\n\r\n"), bytes.Repeat([]byte("A"), 3<<20)...)
	if _, err := send(t, submission.Request{From: "a@example.com", Recipients: []string{"b@example.com"}, Raw: raw}); err != nil || !bytes.Equal(*got, raw) {
		t.Fatalf("err %v equal %v", err, bytes.Equal(*got, raw))
	}
}
