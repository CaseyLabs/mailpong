package credentials

import (
	"encoding/json"
	"github.com/caseylabs/mailven/internal/oauth2"
	"reflect"
	"testing"
)

func TestOAuthGrantEvidence(t *testing.T) {
	for _, tt := range []struct {
		name, raw string
		refresh   bool
		prior     []string
		known     bool
		want      []string
	}{
		{"full", `{"scope":"mail contacts"}`, false, nil, true, []string{"mail", "contacts"}},
		{"partial", `{"scope":"mail"}`, false, nil, true, []string{"mail"}},
		{"omitted authorization", `{}`, false, []string{"mail"}, false, nil},
		{"omitted refresh retains known", `{}`, true, []string{"mail"}, true, []string{"mail"}},
		{"omitted refresh unknown", `{}`, true, nil, false, nil},
		{"refresh narrows grant", `{"scope":"contacts"}`, true, []string{"mail", "contacts"}, true, []string{"contacts"}},
		{"explicit empty", `{"scope":""}`, true, []string{"mail"}, true, []string{}},
		{"null unknown", `{"scope":null}`, false, nil, false, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tokens := &OAuthTokens{Provider: "google", Scopes: tt.prior, ScopesKnown: tt.prior != nil, RefreshToken: "retained", ScopeResource: "resource"}
			var response oauth2.TokenResponse
			if err := json.Unmarshal([]byte(tt.raw), &response); err != nil {
				t.Fatal(err)
			}
			response.AccessToken = "new"
			tokens.ApplyTokenResponse(&response, []string{"mail", "contacts"}, "resource", tt.refresh)
			raw, err := marshalOAuthGrant(tokens)
			if err != nil {
				t.Fatal(err)
			}
			restored := &OAuthTokens{}
			readOAuthGrant(string(raw), restored)
			if restored.ScopesKnown != tt.known || !reflect.DeepEqual(restored.Scopes, tt.want) {
				t.Fatalf("restored grant %+v, want known=%v scopes=%v", restored, tt.known, tt.want)
			}
			if tokens.RefreshToken != "retained" {
				t.Fatal("omitted refresh token was lost")
			}
		})
	}
}

func TestLegacyOAuthScopesAreNotGrantEvidence(t *testing.T) {
	for _, raw := range []string{`["https://mail.google.com/","contacts"]`, `null`, `broken`, `{"version":2,"known":true,"granted":["mail"]}`} {
		tokens := &OAuthTokens{}
		readOAuthGrant(raw, tokens)
		tokens.ApplyTokenResponse(&oauth2.TokenResponse{AccessToken: "refreshed"}, nil, "", true)
		if tokens.ScopesKnown || len(tokens.Scopes) != 0 {
			t.Fatalf("fabricated legacy consent: %+v", tokens)
		}
	}
}

func TestMailGrantCapabilities(t *testing.T) {
	for _, tt := range []struct {
		name, provider, scope, resource string
		imap, smtp                      bool
	}{
		{"Google mail only", "google", "https://mail.google.com/", "https://www.googleapis.com", true, true},
		{"Google contacts only", "google", "https://www.googleapis.com/auth/contacts.readonly", "https://www.googleapis.com", false, false},
		{"Microsoft IMAP only", "microsoft", "https://outlook.office.com/IMAP.AccessAsUser.All", "https://outlook.office.com", true, false},
		{"Microsoft SMTP relative", "microsoft", "SMTP.Send", "https://outlook.office.com", false, true},
		{"Microsoft full relative", "microsoft", "IMAP.AccessAsUser.All SMTP.Send", "https://outlook.office.com", true, true},
		{"Graph is separate", "microsoft", "Mail.ReadWrite Contacts.Read", "https://graph.microsoft.com", false, false},
		{"custom opaque", "custom", "opaque", "issuer", true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tokens := &OAuthTokens{Provider: tt.provider}
			tokens.ApplyTokenResponse(&oauth2.TokenResponse{Scope: tt.scope}, nil, tt.resource, false)
			if (tokens.RequireMailCapability("imap") == nil) != tt.imap || (tokens.RequireMailCapability("smtp") == nil) != tt.smtp {
				t.Fatalf("wrong capability handling: %+v", tokens)
			}
		})
	}
	if err := (&OAuthTokens{Provider: "microsoft"}).RequireMailCapability("imap"); err != nil {
		t.Fatal("unknown legacy mail route blocked", err)
	}
}

func TestOAuthGrantStorageAndRotation(t *testing.T) {
	store, db := newTestStore(t)
	store.keyringEnabled = false // Exercise secure encrypted fallback deterministically.
	insertTestAccount(t, db, "grant")
	tokens := &OAuthTokens{Provider: "microsoft", MicrosoftAccountType: "personal"}
	tokens.ApplyTokenResponse(&oauth2.TokenResponse{AccessToken: "access-secret", RefreshToken: "refresh-secret", Scope: "SMTP.Send"}, []string{"IMAP.AccessAsUser.All", "SMTP.Send"}, "https://outlook.office.com", false)
	if err := store.SetOAuthTokens("grant", tokens); err != nil {
		t.Fatal(err)
	}
	restored, err := store.GetOAuthTokens("grant")
	if err != nil {
		t.Fatal(err)
	}
	if !restored.ScopesKnown || restored.MicrosoftAccountType != "personal" || !reflect.DeepEqual(restored.RequestedScopes, tokens.RequestedScopes) {
		t.Fatalf("metadata not preserved: %+v", restored)
	}
	restored.ApplyTokenResponse(&oauth2.TokenResponse{AccessToken: "new-access", RefreshToken: "rotated", Scope: "IMAP.AccessAsUser.All"}, nil, "", true)
	if err := store.SetOAuthTokensForClientConfig("grant", "microsoft-mail", restored); err != nil {
		t.Fatal(err)
	}
	refreshed, err := store.GetOAuthTokens("grant")
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.RefreshToken != "rotated" || refreshed.RequireMailCapability("smtp") == nil {
		t.Fatalf("rotation/narrowing lost: %+v", refreshed)
	}
	var stored string
	if err := db.QueryRow("SELECT scopes FROM oauth_tokens WHERE account_id='grant'").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	var grant OAuthGrant
	if err := json.Unmarshal([]byte(stored), &grant); err != nil || grant.Resource != "https://outlook.office.com" {
		t.Fatalf("resource context lost: %s", stored)
	}
	if _, err := db.Exec(`UPDATE oauth_tokens SET scopes='["IMAP.AccessAsUser.All","SMTP.Send"]' WHERE account_id='grant'`); err != nil {
		t.Fatal(err)
	}
	legacy, err := store.GetOAuthTokensForClientConfig("grant", "microsoft-mail")
	if err != nil {
		t.Fatal(err)
	}
	if legacy.ScopesKnown || len(legacy.Scopes) > 0 || len(legacy.RequestedScopes) != 2 {
		t.Fatalf("legacy read fabricated grant: %+v", legacy)
	}
}

func TestContactsGrantSeparateFromMail(t *testing.T) {
	for _, provider := range []string{"google", "google-contacts", "microsoft", "microsoft-contacts"} {
		t.Run(provider, func(t *testing.T) {
			tokens := &OAuthTokens{Provider: provider}
			if tokens.RequireContactsRead() == nil {
				t.Fatal("unknown contacts grant permitted")
			}
			tokens.ApplyTokenResponse(&oauth2.TokenResponse{Scope: "https://mail.google.com/"}, nil, "https://www.googleapis.com", false)
			if tokens.RequireContactsRead() == nil {
				t.Fatal("mail grant permitted contacts")
			}
			scope, resource := "https://www.googleapis.com/auth/contacts.readonly", "https://www.googleapis.com"
			if provider == "microsoft" || provider == "microsoft-contacts" {
				scope, resource = "Contacts.Read", "https://graph.microsoft.com"
			}
			tokens.ApplyTokenResponse(&oauth2.TokenResponse{Scope: scope}, nil, resource, false)
			if err := tokens.RequireContactsRead(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
