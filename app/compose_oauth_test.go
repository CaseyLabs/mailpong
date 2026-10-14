package app

import (
	"context"
	"errors"
	"fmt"
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

func linkedOAuthOps(t *testing.T, tokenURL string) (*composeOps, *account.Account, *account.Account) {
	t.Helper()
	tmp := t.TempDir()
	db, err := database.Open(filepath.Join(tmp, "linked-oauth.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	accounts := account.NewStore(db)
	parent, err := accounts.Create(&account.AccountConfig{
		Name: "Parent", DisplayName: "Parent", Email: "parent@example.test", Username: "parent@example.test",
		IMAPHost: "imap.example.test", IMAPPort: 993, SMTPHost: "smtp.example.test", SMTPPort: 587,
		AuthType: account.AuthOAuth2,
	})
	if err != nil {
		t.Fatal(err)
	}
	child, err := accounts.Create(&account.AccountConfig{
		Name: "Shared", DisplayName: "Shared", Email: "shared@example.test", Username: "shared@example.test",
		IMAPHost: "imap.example.test", IMAPPort: 993, SMTPHost: "smtp.example.test", SMTPPort: 587,
		AuthType: account.AuthOAuth2, SharedMailboxParentID: parent.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	cred, err := credentials.NewStore(db.DB, tmp)
	if err != nil {
		t.Fatal(err)
	}
	if err := cred.SetCustomOAuthProvider(parent.ID, credentials.CustomOAuthProvider{
		AuthURL: "https://issuer.example.test/auth", TokenURL: tokenURL, ClientID: "client",
	}); err != nil {
		t.Fatal(err)
	}
	return &composeOps{accountStore: accounts, credStore: cred, oauth2Manager: oauth2.NewManager()}, parent, child
}

func TestLinkedParentAndChildExpiryShareSingleRefresh(t *testing.T) {
	var refreshes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		if got := r.Form.Get("refresh_token"); got != "old-refresh" {
			t.Errorf("refresh token = %q, want old-refresh", got)
		}
		refreshes.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new-access","refresh_token":"rotated-refresh","expires_in":3600}`))
	}))
	defer server.Close()
	ops, parent, child := linkedOAuthOps(t, server.URL)
	expired := &credentials.OAuthTokens{Provider: "custom", AccessToken: "old-access", RefreshToken: "old-refresh", ExpiresAt: time.Now().Add(-time.Minute)}
	for _, id := range []string{parent.ID, child.ID} {
		if err := ops.credStore.SetOAuthTokens(id, expired); err != nil {
			t.Fatal(err)
		}
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	for _, id := range []string{parent.ID, child.ID} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			<-start
			tokens, err := ops.getValidOAuthToken(t.Context(), id)
			if err == nil && (tokens.AccessToken != "new-access" || tokens.RefreshToken != "rotated-refresh") {
				err = fmt.Errorf("unexpected refreshed tokens: %+v", tokens)
			}
			errCh <- err
		}(id)
	}
	close(start)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := refreshes.Load(); got != 1 {
		t.Fatalf("token endpoint called %d times, want one shared parent refresh", got)
	}
	for _, id := range []string{parent.ID, child.ID} {
		tokens, err := ops.credStore.GetOAuthTokens(id)
		if err != nil || tokens.AccessToken != "new-access" || tokens.RefreshToken != "rotated-refresh" {
			t.Fatalf("refreshed credentials were not retained for %s: %+v, %v", id, tokens, err)
		}
	}
}

func TestLinkedRefreshRetainsOmittedRefreshToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new-access","expires_in":3600}`))
	}))
	defer server.Close()
	ops, parent, child := linkedOAuthOps(t, server.URL)
	for _, id := range []string{parent.ID, child.ID} {
		if err := ops.credStore.SetOAuthTokens(id, &credentials.OAuthTokens{
			Provider: "custom", AccessToken: "old-access", RefreshToken: "retained-refresh", ExpiresAt: time.Now().Add(-time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ops.getValidOAuthToken(t.Context(), child.ID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{parent.ID, child.ID} {
		tokens, err := ops.credStore.GetOAuthTokens(id)
		if err != nil || tokens.AccessToken != "new-access" || tokens.RefreshToken != "retained-refresh" {
			t.Fatalf("omitted refresh token was not retained for %s: %+v, %v", id, tokens, err)
		}
	}
}

func TestLinkedIMAPUsesChildSASLIdentityAndParentToken(t *testing.T) {
	ops, parent, child := linkedOAuthOps(t, "http://127.0.0.1:1/token")
	for id, access := range map[string]string{parent.ID: "parent-access", child.ID: "stale-child-access"} {
		if err := ops.credStore.SetOAuthTokens(id, &credentials.OAuthTokens{
			Provider: "custom", AccessToken: access, RefreshToken: "refresh", ExpiresAt: time.Now().Add(time.Hour),
		}); err != nil {
			t.Fatal(err)
		}
	}
	config, err := ops.getIMAPCredentials(t.Context(), child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if config.Username != child.Username || config.AccessToken != "parent-access" {
		t.Fatalf("IMAP auth mixed mailbox and grant identities: username=%q access=%q", config.Username, config.AccessToken)
	}
}

func TestLinkedInvalidGrantRequestsParentReauthorization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"revoked"}`))
	}))
	defer server.Close()
	ops, parent, child := linkedOAuthOps(t, server.URL)
	if err := ops.credStore.SetOAuthTokens(parent.ID, &credentials.OAuthTokens{
		Provider: "custom", AccessToken: "expired", RefreshToken: "revoked-refresh", ExpiresAt: time.Now().Add(-time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if err := ops.credStore.SetOAuthTokens(child.ID, &credentials.OAuthTokens{
		Provider: "custom", AccessToken: "expired", RefreshToken: "revoked-refresh", ExpiresAt: time.Now().Add(-time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	var emitted map[string]interface{}
	orig := eventsEmit
	eventsEmit = func(_ context.Context, name string, values ...interface{}) {
		if name == "oauth:reauth-required" && len(values) > 0 {
			emitted, _ = values[0].(map[string]interface{})
		}
	}
	t.Cleanup(func() { eventsEmit = orig })
	if _, err := ops.getValidOAuthToken(t.Context(), child.ID); err == nil || !errors.Is(err, oauth2.ErrInvalidGrant) {
		t.Fatalf("linked refresh error = %v, want invalid_grant", err)
	}
	if emitted["accountId"] != parent.ID {
		t.Fatalf("reauthorization event = %+v, want parent account %s", emitted, parent.ID)
	}
	for _, id := range []string{parent.ID, child.ID} {
		tokens, err := ops.credStore.GetOAuthTokens(id)
		if err != nil || tokens.RefreshToken != "" {
			t.Fatalf("revoked refresh credential remains for %s: %+v, %v", id, tokens, err)
		}
	}
	status, err := (&App{accountStore: ops.accountStore, credStore: ops.credStore}).GetOAuthStatus(child.ID)
	if err != nil || !status.NeedsReauth {
		t.Fatalf("linked child status did not surface parent reauthorization: %+v, %v", status, err)
	}
}

func TestOAuthRefreshFailedPromptsOnlyForInvalidGrant(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantReauth bool
	}{
		{"invalid_grant", fmt.Errorf("%w - Bad Request", oauth2.ErrInvalidGrant), true},
		{"not configured", fmt.Errorf("%w: google", oauth2.ErrNotConfigured), false},
		{"server error", errors.New("token refresh failed: server_error - "), false},
		{"network error", errors.New("request failed: dial tcp: connection refused"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var emitted []string
			orig := eventsEmit
			eventsEmit = func(_ context.Context, name string, _ ...interface{}) { emitted = append(emitted, name) }
			t.Cleanup(func() { eventsEmit = orig })

			err := (&composeOps{}).oauthRefreshFailed(context.Background(), "acc", "google", tt.err)
			if !errors.Is(err, tt.err) {
				t.Fatalf("error %v does not wrap %v", err, tt.err)
			}
			if got := len(emitted) == 1 && emitted[0] == "oauth:reauth-required"; got != tt.wantReauth {
				t.Errorf("emitted %v, want reauth %v", emitted, tt.wantReauth)
			}
			if got := strings.Contains(err.Error(), "re-authorization required"); got != tt.wantReauth {
				t.Errorf("message %q: re-authorization wording %v, want %v", err, got, tt.wantReauth)
			}
		})
	}
}
