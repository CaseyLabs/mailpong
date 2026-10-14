package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/credentials"
	"github.com/caseylabs/mailven/internal/database"
	"github.com/caseylabs/mailven/internal/oauth2"
)

const gmailScopes = "https://www.googleapis.com/auth/gmail.modify openid"

type mailAPIFixture struct {
	db      *database.DB
	ops     *mailAPIOps
	acc     *account.Account
	secret  *string
	clients *string
	events  *[]string
	mu      *sync.Mutex
}

func newMailAPIFixture(t *testing.T, provider, tokenURL string) *mailAPIFixture {
	t.Helper()
	tmp := t.TempDir()
	db, err := database.Open(filepath.Join(tmp, "mailapi.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	accounts := account.NewStore(db)
	acc, err := accounts.Create(&account.AccountConfig{
		Name: "A", DisplayName: "A", Email: "user@example.test", Username: "user@example.test",
		IMAPHost: "imap.example.test", IMAPPort: 993, SMTPHost: "smtp.example.test", SMTPPort: 587,
		AuthType: account.AuthOAuth2,
	})
	if err != nil {
		t.Fatal(err)
	}
	cred, err := credentials.NewStore(db.DB, tmp)
	if err != nil {
		t.Fatal(err)
	}
	slot := provider + "-mail"
	protocol := &credentials.OAuthTokens{Provider: provider, AccessToken: "proto-access", RefreshToken: "proto-refresh",
		ExpiresAt: time.Now().Add(time.Hour)}
	if err := cred.SetOAuthTokensForClientConfig(acc.ID, slot, protocol); err != nil {
		t.Fatal(err)
	}
	secret, clientID := "secret", "client"
	var events []string
	mu := &sync.Mutex{}
	fx := &mailAPIFixture{db: db, acc: acc, secret: &secret, clients: &clientID, events: &events, mu: mu}
	fx.ops = &mailAPIOps{accountStore: accounts, credStore: cred, oauth2Manager: oauth2.NewManager(),
		baseProvider: func(name string) (oauth2.ProviderConfig, error) {
			return oauth2.ProviderConfig{Name: name, AuthURL: "https://issuer.example.test/auth", TokenURL: tokenURL,
				ClientID: clientID, ClientSecret: secret}, nil
		},
		emit: func(event string, _ any) { mu.Lock(); events = append(events, event); mu.Unlock() }}
	return fx
}

func (fx *mailAPIFixture) consent(t *testing.T, shared bool) *mailAPIConsent {
	t.Helper()
	c, err := fx.ops.prepare(fx.acc.ID, shared)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func (fx *mailAPIFixture) protocolTokens(t *testing.T) *credentials.OAuthTokens {
	t.Helper()
	tok, err := fx.ops.credStore.GetOAuthTokensForClientConfig(fx.acc.ID, "google-mail")
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestMailAPIConsentFinish(t *testing.T) {
	fx := newMailAPIFixture(t, "google", "https://issuer.example.test/token")
	c := fx.consent(t, false)
	resp := &oauth2.TokenResponse{AccessToken: "api-access", RefreshToken: "api-refresh", ExpiresIn: 3600, Scope: gmailScopes}
	if err := fx.ops.finish(c, resp, "other@example.test"); err == nil {
		t.Fatal("identity mismatch accepted")
	}
	if r, _ := fx.ops.accountStore.GetMailAPIRoute(fx.acc.ID); r != nil {
		t.Fatal("route saved after identity mismatch")
	}
	if err := fx.ops.finish(c, resp, "USER@example.test"); err != nil {
		t.Fatal(err)
	}
	st, err := fx.ops.status(fx.acc.ID)
	if err != nil || !st.Authorized || st.Route != account.MailRouteGmailAPI || st.State != account.MailRouteStateStaged {
		t.Fatalf("status = %+v, %v", st, err)
	}
	if p := fx.protocolTokens(t); p.AccessToken != "proto-access" || p.RefreshToken != "proto-refresh" {
		t.Fatalf("protocol slot changed: %+v", p)
	}
	route, _ := fx.ops.accountStore.GetMailAPIRoute(fx.acc.ID)
	if route.Snapshot.ClientID != "client" || strings.Contains(route.Snapshot.TokenURL, "secret") {
		t.Fatalf("snapshot = %+v", route.Snapshot)
	}
}

func TestMailAPIPartialConsentNotSaved(t *testing.T) {
	fx := newMailAPIFixture(t, "google", "https://issuer.example.test/token")
	c := fx.consent(t, false)
	for _, scope := range []string{"openid", ""} {
		resp := &oauth2.TokenResponse{AccessToken: "a", RefreshToken: "r", ExpiresIn: 3600, Scope: scope}
		err := fx.ops.finish(c, resp, "user@example.test")
		if err == nil || classifyMailAPI(err) != oauth2.MailAPIFailurePartialConsent {
			t.Fatalf("scope %q: err = %v", scope, err)
		}
	}
	if r, _ := fx.ops.accountStore.GetMailAPIRoute(fx.acc.ID); r != nil {
		t.Fatal("route saved for partial consent")
	}
	if _, err := fx.ops.credStore.GetOAuthTokensForClientConfig(fx.acc.ID, c.profile.ClientConfigID); err == nil {
		t.Fatal("tokens saved for partial consent")
	}
}

func TestMailAPIPrepareRejections(t *testing.T) {
	fx := newMailAPIFixture(t, "google", "https://issuer.example.test/token")
	if _, err := fx.ops.prepare(fx.acc.ID, true); err == nil {
		t.Fatal("Gmail shared accepted")
	}
	child, err := fx.ops.accountStore.Create(&account.AccountConfig{
		Name: "C", DisplayName: "C", Email: "c@example.test", Username: "c@example.test",
		IMAPHost: "i", IMAPPort: 993, SMTPHost: "s", SMTPPort: 587,
		AuthType: account.AuthOAuth2, SharedMailboxParentID: fx.acc.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fx.ops.prepare(child.ID, false); err == nil {
		t.Fatal("shared-mailbox child accepted")
	}
	pw, _ := fx.ops.accountStore.Create(&account.AccountConfig{
		Name: "P", DisplayName: "P", Email: "p@example.test", Username: "p",
		IMAPHost: "i", IMAPPort: 993, SMTPHost: "s", SMTPPort: 587, AuthType: account.AuthPassword,
	})
	if _, err := fx.ops.prepare(pw.ID, false); err == nil {
		t.Fatal("password account accepted")
	}
	ms := newMailAPIFixture(t, "microsoft", "https://issuer.example.test/token")
	tok, _ := ms.ops.credStore.GetOAuthTokensForClientConfig(ms.acc.ID, "microsoft-mail")
	tok.MicrosoftAccountType = "personal"
	if err := ms.ops.credStore.SetOAuthTokensForClientConfig(ms.acc.ID, "microsoft-mail", tok); err != nil {
		t.Fatal(err)
	}
	if _, err := ms.ops.prepare(ms.acc.ID, false); err == nil {
		t.Fatal("personal Microsoft accepted")
	}
}

func stageGmailRoute(t *testing.T, fx *mailAPIFixture, expires time.Time) {
	t.Helper()
	c := fx.consent(t, false)
	resp := &oauth2.TokenResponse{AccessToken: "api-access", RefreshToken: "api-refresh", ExpiresIn: 3600, Scope: gmailScopes}
	if err := fx.ops.finish(c, resp, "user@example.test"); err != nil {
		t.Fatal(err)
	}
	tok, _ := fx.ops.credStore.GetOAuthTokensForClientConfig(fx.acc.ID, c.profile.ClientConfigID)
	if err := fx.ops.credStore.UpdateOAuthTokensForClientConfig(fx.acc.ID, c.profile.ClientConfigID, tok.AccessToken, tok.RefreshToken, expires); err != nil {
		t.Fatal(err)
	}
}

func TestMailAPIConcurrentRefreshOnce(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_ = r.ParseForm()
		if r.Form.Get("refresh_token") != "api-refresh" || r.Form.Get("client_secret") != "secret" {
			http.Error(w, "bad", 400)
			return
		}
		time.Sleep(50 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "fresh", "expires_in": 3600, "token_type": "Bearer"})
	}))
	defer srv.Close()
	fx := newMailAPIFixture(t, "google", srv.URL)
	stageGmailRoute(t, fx, time.Now().Add(-time.Minute))
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tok, err := fx.ops.validToken(fx.acc.ID)
			if err != nil || tok.AccessToken != "fresh" {
				t.Errorf("token = %+v, %v", tok, err)
			}
		}()
	}
	wg.Wait()
	if n := calls.Load(); n != 1 {
		t.Fatalf("refresh requests = %d, want 1", n)
	}
	tok, _ := fx.ops.credStore.GetOAuthTokensForClientConfig(fx.acc.ID, "google-mail-api")
	if err := tok.RequireMailAPICapability("send", false); err != nil || tok.RefreshToken != "api-refresh" {
		t.Fatalf("scope evidence/refresh lost: %v %+v", err, tok)
	}
	if p := fx.protocolTokens(t); p.AccessToken != "proto-access" {
		t.Fatal("protocol slot changed by API refresh")
	}
}

func TestMailAPIInvalidGrantClearsAndEmits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"revoked"}`))
	}))
	defer srv.Close()
	fx := newMailAPIFixture(t, "google", srv.URL)
	stageGmailRoute(t, fx, time.Now().Add(-time.Minute))
	if _, err := fx.ops.validToken(fx.acc.ID); err == nil {
		t.Fatal("expected error")
	}
	if tok, err := fx.ops.credStore.GetOAuthTokensForClientConfig(fx.acc.ID, "google-mail-api"); err == nil && tok.RefreshToken != "" {
		t.Fatal("refresh token not cleared")
	}
	if p := fx.protocolTokens(t); p.RefreshToken != "proto-refresh" {
		t.Fatal("protocol refresh token cleared")
	}
	fx.mu.Lock()
	defer fx.mu.Unlock()
	if len(*fx.events) != 1 || (*fx.events)[0] != "mail-api:reauth-required" {
		t.Fatalf("events = %v", *fx.events)
	}
	if st, _ := fx.ops.status(fx.acc.ID); st.Authorized || st.Failure != "reauthorize" {
		t.Fatalf("status = %+v", st)
	}
}

func TestMailAPITransientRefreshKeepsGrant(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer srv.Close()
	fx := newMailAPIFixture(t, "google", srv.URL)
	stageGmailRoute(t, fx, time.Now().Add(-time.Minute))
	if _, err := fx.ops.validToken(fx.acc.ID); err == nil {
		t.Fatal("expected error")
	}
	tok, err := fx.ops.credStore.GetOAuthTokensForClientConfig(fx.acc.ID, "google-mail-api")
	if err != nil || tok.RefreshToken != "api-refresh" || len(*fx.events) != 0 {
		t.Fatalf("grant disturbed: %+v %v events=%v", tok, err, *fx.events)
	}
}

func TestMailAPIRefreshRefusesChangedClient(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer srv.Close()
	fx := newMailAPIFixture(t, "google", srv.URL)
	stageGmailRoute(t, fx, time.Now().Add(-time.Minute))
	*fx.clients = "other-client"
	_, err := fx.ops.validToken(fx.acc.ID)
	if err == nil || calls.Load() != 0 {
		t.Fatalf("err=%v calls=%d", err, calls.Load())
	}
	if st, _ := fx.ops.status(fx.acc.ID); st.Failure != "reauthorize" {
		t.Fatalf("status = %+v", st)
	}
}

func TestMailAPIClearRoute(t *testing.T) {
	fx := newMailAPIFixture(t, "google", "https://issuer.example.test/token")
	stageGmailRoute(t, fx, time.Now().Add(time.Hour))
	if err := fx.ops.clear(fx.acc.ID); err != nil {
		t.Fatal(err)
	}
	if st, _ := fx.ops.status(fx.acc.ID); st.Route != mailAPIProtocolRoute || st.Authorized {
		t.Fatalf("status = %+v", st)
	}
	if _, err := fx.ops.credStore.GetOAuthTokensForClientConfig(fx.acc.ID, "google-mail-api"); err == nil {
		t.Fatal("API tokens remain")
	}
	if p := fx.protocolTokens(t); p.RefreshToken != "proto-refresh" {
		t.Fatal("protocol slot changed")
	}
	if err := fx.ops.clear(fx.acc.ID); err != nil {
		t.Fatalf("clear is not idempotent: %v", err)
	}
}

func TestMailAPICancelConsent(t *testing.T) {
	fx := newMailAPIFixture(t, "google", "https://issuer.example.test/token")
	var emitted []string
	old := eventsEmit
	eventsEmit = func(_ context.Context, event string, _ ...interface{}) { emitted = append(emitted, event) }
	defer func() { eventsEmit = old }()
	app := &App{ctx: context.Background(), oauth2Manager: fx.ops.oauth2Manager, accountStore: fx.ops.accountStore, credStore: fx.ops.credStore}
	c := fx.consent(t, false)
	if _, err := app.oauth2Manager.StartAuthFlowWithProvider(app.ctx, &c.profile.Provider); err != nil {
		t.Fatal(err)
	}
	app.CancelMailAPIConsent()
	if len(emitted) != 1 || emitted[0] != "mail-api:cancelled" {
		t.Fatalf("events = %v", emitted)
	}
	if r, _ := fx.ops.accountStore.GetMailAPIRoute(fx.acc.ID); r != nil {
		t.Fatal("route created by cancelled consent")
	}
	if p := fx.protocolTokens(t); p.RefreshToken != "proto-refresh" {
		t.Fatal("protocol slot changed")
	}
}

func TestAPITokenSourceConcurrentRejectedToken(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "new", "expires_in": 3600, "scope": gmailScopes})
	}))
	defer srv.Close()
	fx := newMailAPIFixture(t, "google", srv.URL)
	if err := fx.ops.finish(fx.consent(t, false), &oauth2.TokenResponse{AccessToken: "old", RefreshToken: "refresh", ExpiresIn: 3600, Scope: gmailScopes}, fx.acc.Email); err != nil {
		t.Fatal(err)
	}
	source := &apiTokenSource{ops: fx.ops, accountID: fx.acc.ID}
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			if _, err := source.Token(t.Context()); err != nil {
				t.Error(err)
			}
			if got, err := source.Refresh(t.Context(), "old"); err != nil || got != "new" {
				t.Errorf("refresh = %q, %v", got, err)
			}
		})
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("refresh requests = %d", calls.Load())
	}
	// A late rejection of old must reuse new, even after another Token call.
	if _, err := source.Token(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := source.Refresh(t.Context(), "old"); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatal("late old rejection refreshed the new token")
	}
}
