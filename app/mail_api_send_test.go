package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caseylabs/mailven/internal/gmailapi"
	"github.com/caseylabs/mailven/internal/smtp"
	"github.com/caseylabs/mailven/internal/submission"
)

// redirectClient sends every request to the fixture server.
func redirectClient(srv *httptest.Server) *http.Client {
	target, _ := url.Parse(srv.URL)
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r = r.Clone(r.Context())
		r.URL.Scheme, r.URL.Host = target.Scheme, target.Host
		return http.DefaultTransport.RoundTrip(r)
	}), CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type apiSendFixture struct {
	*mailAPIFixture
	store   *submission.Store
	sendAs  *gmailapi.Store
	attempt *submission.Attempt
	req     submission.Request
}

func newAPISendFixture(t *testing.T, tokenURL string, expires time.Time) *apiSendFixture {
	t.Helper()
	fx := newMailAPIFixture(t, "google", tokenURL)
	stageGmailRoute(t, fx, expires)
	f := &apiSendFixture{mailAPIFixture: fx, store: submission.NewStore(fx.db), sendAs: gmailapi.NewStore(fx.db.DB)}
	if err := f.sendAs.Replace(fx.acc.ID, []gmailapi.SendAs{{Email: "user@example.test", IsPrimary: true}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	msg := smtp.ComposeMessage{Subject: "s"}
	a, err := f.store.Begin(fx.acc.ID, "draft", msg)
	if err != nil {
		t.Fatal(err)
	}
	f.attempt = a
	f.req = submission.Request{From: "user@example.test", Recipients: []string{"to@example.test"},
		Raw: []byte("Date: Mon, 02 Jan 2006 15:04:05 +0000\r\nMessage-ID: <id@example.test>\r\nFrom: user@example.test\r\nTo: to@example.test\r\nSubject: s\r\n\r\nbody\r\n")}
	return f
}

func (f *apiSendFixture) submit(t *testing.T, srv *httptest.Server) (smtp.SubmissionResult, error) {
	t.Helper()
	return f.ops.submitViaAPI(context.Background(), f.store, f.sendAs, redirectClient(srv), f.attempt, f.acc, f.req)
}

func (f *apiSendFixture) journaled(t *testing.T) string {
	t.Helper()
	a, err := f.store.Get(f.attempt.ID)
	if err != nil {
		t.Fatal(err)
	}
	return a.Outcome
}

func TestMailAPISendAcceptedCommitsJournal(t *testing.T) {
	var sends atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sends.Add(1)
		if r.Header.Get("Authorization") != "Bearer api-access" {
			t.Errorf("token = %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	defer srv.Close()
	f := newAPISendFixture(t, "https://issuer.example.test/token", time.Now().Add(time.Hour))
	res, err := f.submit(t, srv)
	if err != nil || res.Outcome != smtp.OutcomeAccepted || sends.Load() != 1 {
		t.Fatalf("res=%+v err=%v sends=%d", res, err, sends.Load())
	}
	if got := f.journaled(t); got != string(smtp.OutcomeAccepted) {
		t.Fatalf("journal = %q", got)
	}
}

func TestMailAPISendUnknownStaysJournaledNoReplay(t *testing.T) {
	var sends atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sends.Add(1)
		w.WriteHeader(503)
	}))
	defer srv.Close()
	f := newAPISendFixture(t, "https://issuer.example.test/token", time.Now().Add(time.Hour))
	res, err := f.submit(t, srv)
	if err == nil || res.Outcome != smtp.OutcomeUnknown || sends.Load() != 1 {
		t.Fatalf("res=%+v err=%v sends=%d", res, err, sends.Load())
	}
	if got := f.journaled(t); got != string(smtp.OutcomeUnknown) {
		t.Fatalf("journal = %q", got)
	}
}

func TestMailAPISendPreflightRejectsWithoutRequest(t *testing.T) {
	var sends atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { sends.Add(1) }))
	defer srv.Close()
	f := newAPISendFixture(t, "https://issuer.example.test/token", time.Now().Add(time.Hour))
	f.req.From = "stranger@example.test"
	res, err := f.submit(t, srv)
	if err == nil || res.Outcome != smtp.OutcomeNotSubmitted || sends.Load() != 0 {
		t.Fatalf("res=%+v err=%v sends=%d", res, err, sends.Load())
	}
}

func TestMailAPISend401RefreshesOnceThenAccepts(t *testing.T) {
	var refreshes, sends atomic.Int32
	issuer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshes.Add(1)
		_, _ = w.Write([]byte(`{"access_token":"fresh","refresh_token":"api-refresh","expires_in":3600,"token_type":"Bearer"}`))
	}))
	defer issuer.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sends.Add(1)
		if r.Header.Get("Authorization") != "Bearer fresh" {
			w.WriteHeader(401)
			return
		}
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	defer srv.Close()
	f := newAPISendFixture(t, issuer.URL, time.Now().Add(time.Hour))
	res, err := f.submit(t, srv)
	if err != nil || res.Outcome != smtp.OutcomeAccepted || refreshes.Load() != 1 || sends.Load() != 2 {
		t.Fatalf("res=%+v err=%v refreshes=%d sends=%d", res, err, refreshes.Load(), sends.Load())
	}
}

func TestMailAPISendInvalidGrantRequestsReauthKeepsProtocol(t *testing.T) {
	issuer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer issuer.Close()
	var sends atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sends.Add(1)
		w.WriteHeader(401)
	}))
	defer srv.Close()
	f := newAPISendFixture(t, issuer.URL, time.Now().Add(time.Hour))
	res, err := f.submit(t, srv)
	if err == nil || res.Outcome == smtp.OutcomeAccepted || sends.Load() != 1 {
		t.Fatalf("res=%+v err=%v sends=%d", res, err, sends.Load())
	}
	f.mu.Lock()
	events := append([]string(nil), *f.events...)
	f.mu.Unlock()
	if len(events) != 1 || events[0] != "mail-api:reauth-required" {
		t.Fatalf("events = %v", events)
	}
	if p := f.protocolTokens(t); p.RefreshToken != "proto-refresh" {
		t.Fatal("protocol credentials cleared")
	}
}
