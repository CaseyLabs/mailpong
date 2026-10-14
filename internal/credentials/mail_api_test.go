package credentials

import (
	"reflect"
	"strings"
	"testing"

	"github.com/caseylabs/mailven/internal/oauth2"
)

func TestMailAPICapabilities(t *testing.T) {
	const google = "https://www.googleapis.com"
	const graph = "https://graph.microsoft.com"
	for _, tt := range []struct {
		name, provider, resource, scopes, op string
		known, shared, allowed               bool
	}{
		{"unknown", "google", google, "https://mail.google.com/", "send", false, false, false},
		{"requested only", "microsoft", graph, "", "send", false, false, false},
		{"empty consent", "microsoft", graph, "", "send", true, false, false},
		{"gmail modify read", "google", google, google + "/auth/gmail.modify", "read", true, false, true},
		{"gmail modify write", "google", google, google + "/auth/gmail.modify", "write", true, false, true},
		{"gmail modify send", "google", google, google + "/auth/gmail.modify", "send", true, false, true},
		{"gmail modify lists aliases", "google", google, google + "/auth/gmail.modify", "aliases", true, false, true},
		{"gmail readonly lists aliases", "google", google, google + "/auth/gmail.readonly", "aliases", true, false, true},
		{"gmail readonly", "google", google, google + "/auth/gmail.readonly", "read", true, false, true},
		{"gmail readonly cannot write", "google", google, google + "/auth/gmail.readonly", "write", true, false, false},
		{"gmail send only", "google", google, google + "/auth/gmail.send", "send", true, false, true},
		{"gmail compose sends", "google", google, google + "/auth/gmail.compose", "send", true, false, true},
		{"gmail send cannot read", "google", google, google + "/auth/gmail.send", "read", true, false, false},
		{"gmail alias settings", "google", google, google + "/auth/gmail.settings.basic", "aliases", true, false, true},
		{"gmail full mail", "google", google, "https://mail.google.com/", "aliases", true, false, true},
		{"gmail wrong resource", "google", graph, "https://mail.google.com/", "send", true, false, false},
		{"gmail unknown resource", "google", "", "https://mail.google.com/", "send", true, false, false},
		{"gmail another mailbox", "google", google, "https://mail.google.com/", "read", true, true, false},
		{"graph readwrite reads", "microsoft", graph, "Mail.ReadWrite", "read", true, false, true},
		{"graph readwrite writes", "microsoft", graph, "Mail.ReadWrite", "write", true, false, true},
		{"graph readwrite cannot send", "microsoft", graph, "Mail.ReadWrite", "send", true, false, false},
		{"graph send", "microsoft", graph, graph + "/Mail.Send", "send", true, false, true},
		{"graph read cannot write", "microsoft", graph, "Mail.Read", "write", true, false, false},
		{"graph read", "microsoft", graph, "Mail.Read", "read", true, false, true},
		{"graph send cannot read", "microsoft", graph, "Mail.Send", "read", true, false, false},
		{"outlook relative wrong resource", "microsoft", "https://outlook.office.com", "Mail.Send", "send", true, false, false},
		{"outlook with qualified graph scope", "microsoft", "https://outlook.office.com", graph + "/Mail.Send", "send", true, false, false},
		{"graph missing resource", "microsoft", "", graph + "/Mail.Send", "send", true, false, false},
		{"graph mail is not shared send", "microsoft", graph, "Mail.Send", "send", true, true, false},
		{"graph shared send", "microsoft", graph, "Mail.Send.Shared", "send", true, true, true},
		{"graph shared read", "microsoft", graph, "Mail.Read.Shared", "read", true, true, true},
		{"graph shared write", "microsoft", graph, "Mail.ReadWrite.Shared", "write", true, true, true},
		{"graph own write is not shared", "microsoft", graph, "Mail.ReadWrite", "write", true, true, false},
		{"graph shared read cannot write", "microsoft", graph, "Mail.Read.Shared", "write", true, true, false},
		{"graph shared scope reads own", "microsoft", graph, "Mail.ReadWrite.Shared", "read", true, false, true},
		{"graph shared scope writes own", "microsoft", graph, "Mail.ReadWrite.Shared", "write", true, false, true},
		{"graph shared scope sends own", "microsoft", graph, "Mail.Send.Shared", "send", true, false, true},
		{"graph contacts are separate", "microsoft", graph, "Contacts.Read", "read", true, false, false},
		{"google contacts are separate", "google", google, google + "/auth/contacts.readonly", "read", true, false, false},
		{"unsupported provider", "custom", graph, "Mail.Send", "send", true, false, false},
		{"unsupported operation", "microsoft", graph, "Mail.ReadWrite Mail.Send", "delete-permanently", true, false, false},
		{"unsupported gmail operation", "google", google, "https://mail.google.com/", "delete-permanently", true, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tokens := &OAuthTokens{Provider: tt.provider, ScopeResource: tt.resource, Scopes: strings.Fields(tt.scopes), ScopesKnown: tt.known, RequestedScopes: []string{graph + "/Mail.Send", google + "/auth/gmail.modify"}}
			if got := tokens.RequireMailAPICapability(tt.op, tt.shared); (got == nil) != tt.allowed {
				t.Fatalf("allowed=%t, error=%v", tt.allowed, got)
			}
		})
	}
	if (*OAuthTokens)(nil).RequireMailAPICapability("read", false) == nil {
		t.Fatal("nil grant authorized")
	}
}

func TestMailAPIGrantSlotIsolation(t *testing.T) {
	for _, base := range []oauth2.ProviderConfig{oauth2.GoogleProvider(), oauth2.MicrosoftProvider()} {
		t.Run(base.Name, func(t *testing.T) {
			store, db := newTestStore(t)
			store.keyringEnabled = false
			insertTestAccount(t, db, "api-account")
			protocol := &OAuthTokens{Provider: base.Name}
			protocol.ApplyTokenResponse(&oauth2.TokenResponse{AccessToken: "protocol-access", RefreshToken: "protocol-refresh", Scope: strings.Join(base.Scopes, " ")}, base.Scopes, oauth2.ScopeResource(base), false)
			if err := store.SetOAuthTokens("api-account", protocol); err != nil {
				t.Fatal(err)
			}
			before, err := store.GetOAuthTokens("api-account")
			if err != nil {
				t.Fatal(err)
			}
			profile, err := oauth2.BuildMailAPIProfile(base, base.Name == "microsoft")
			if err != nil {
				t.Fatal(err)
			}
			tokens := &OAuthTokens{Provider: base.Name}
			tokens.ApplyTokenResponse(&oauth2.TokenResponse{AccessToken: "api-access", RefreshToken: "api-refresh", Scope: strings.Join(profile.Provider.Scopes, " ")}, profile.Provider.Scopes, oauth2.ScopeResource(profile.Provider), false)
			if err := store.SetOAuthTokensForClientConfig("api-account", profile.ClientConfigID, tokens); err != nil {
				t.Fatal(err)
			}
			restored, err := store.GetOAuthTokensForClientConfig("api-account", profile.ClientConfigID)
			if err != nil {
				t.Fatal(err)
			}
			if restored.AccessToken != "api-access" || restored.RefreshToken != "api-refresh" || !reflect.DeepEqual(restored.RequestedScopes, profile.Provider.Scopes) || restored.RequireMailAPICapability("send", false) != nil {
				t.Fatal("API grant did not round-trip")
			}
			var access, refresh string
			if err := db.QueryRow("SELECT encrypted_access_token, encrypted_refresh_token FROM oauth_tokens WHERE account_id=? AND client_config_id=?", "api-account", profile.ClientConfigID).Scan(&access, &refresh); err != nil {
				t.Fatal(err)
			}
			if access == "" || refresh == "" || strings.Contains(access, "api-access") || strings.Contains(refresh, "api-refresh") {
				t.Fatal("API secrets not encrypted")
			}
			for _, response := range []*oauth2.TokenResponse{
				{AccessToken: "rotated-access", RefreshToken: "rotated-refresh"},
				{AccessToken: "retained-access"},
				{AccessToken: "narrowed-access", Scope: "openid", ScopePresent: true},
			} {
				restored.ApplyTokenResponse(response, nil, "", true)
				if err := store.SetOAuthTokensForClientConfig("api-account", profile.ClientConfigID, restored); err != nil {
					t.Fatal(err)
				}
				restored, err = store.GetOAuthTokensForClientConfig("api-account", profile.ClientConfigID)
				if err != nil {
					t.Fatal(err)
				}
				if restored.RefreshToken != "rotated-refresh" || restored.ScopeResource != oauth2.ScopeResource(profile.Provider) || (restored.RequireMailAPICapability("send", false) == nil) != (response.Scope == "") {
					t.Fatal("rotation/omission/narrowing evidence lost")
				}
			}
			if _, err := db.Exec("UPDATE oauth_tokens SET scopes=? WHERE account_id=? AND client_config_id=?", `["https://mail.google.com/","Mail.Send"]`, "api-account", profile.ClientConfigID); err != nil {
				t.Fatal(err)
			}
			legacy, err := store.GetOAuthTokensForClientConfig("api-account", profile.ClientConfigID)
			if err != nil {
				t.Fatal(err)
			}
			if legacy.RequireMailAPICapability("send", false) == nil {
				t.Fatal("legacy requests authorized API")
			}
			if err := store.DeleteOAuthTokensForClientConfig("api-account", profile.ClientConfigID); err != nil {
				t.Fatal(err)
			}
			after, err := store.GetOAuthTokens("api-account")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatal("API lifecycle changed protocol grant")
			}
		})
	}
}
