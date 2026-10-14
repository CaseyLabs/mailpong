package app

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/contact"
	"github.com/caseylabs/mailven/internal/credentials"
)

func TestAutocompleteRequiresOnlyOtherContactsGrant(t *testing.T) {
	for _, tt := range []struct {
		name       string
		scopes     []string
		wantRemote bool
	}{
		{"other contacts only", []string{"https://www.googleapis.com/auth/contacts.other.readonly"}, true},
		{"saved contacts only", []string{"https://www.googleapis.com/auth/contacts.readonly"}, false},
		{"unknown", nil, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f, _, _ := initAppSendFixture(t, appSMTPConfig{})
			acc, err := f.ops.accountStore.Get(f.accountID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.ops.accountStore.Update(acc.ID, &account.AccountConfig{Name: acc.Name, DisplayName: "Sender", Email: acc.Email, Username: acc.Username, IMAPHost: "imap.gmail.com", SMTPHost: acc.SMTPHost, SMTPPort: acc.SMTPPort, AuthType: account.AuthOAuth2}); err != nil {
				t.Fatal(err)
			}
			if err := f.ops.credStore.SetOAuthTokens(acc.ID, &credentials.OAuthTokens{Provider: "google", AccessToken: "autocomplete-fixture", RefreshToken: "fixture-refresh", ExpiresAt: time.Now().Add(time.Hour), Scopes: tt.scopes, ScopesKnown: tt.scopes != nil}); err != nil {
				t.Fatal(err)
			}
			local := contact.NewStore(f.db.DB)
			if err := local.AddOrUpdate("local@example.test", "Search Local"); err != nil {
				t.Fatal(err)
			}
			requests := 0
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			http.DefaultTransport = senderRoundTripper(func(r *http.Request) (*http.Response, error) {
				requests++
				if r.URL.Host != "people.googleapis.com" || r.URL.Path != "/v1/otherContacts:search" {
					t.Fatalf("unexpected request: %s", r.URL)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"results":[{"person":{"names":[{"displayName":"Search Remote"}],"emailAddresses":[{"value":"remote@example.test"}]}}]}`))}, nil
			})
			a := &App{composeOps: *f.ops, accountStore: f.ops.accountStore, contactStore: local, googleContactsClient: contact.NewGoogleContactsClient()}
			results, err := a.SearchContacts("Search", 10)
			want := 1
			if tt.wantRemote {
				want++
			}
			if err != nil || len(results) != want || (requests > 0) != tt.wantRemote {
				t.Fatalf("results=%+v requests=%d err=%v; want %d", results, requests, err, want)
			}
		})
	}
}
