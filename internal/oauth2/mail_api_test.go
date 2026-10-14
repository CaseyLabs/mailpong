package oauth2

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestMailAPIProfiles(t *testing.T) {
	for _, tt := range []struct {
		name           string
		base           ProviderConfig
		shared         bool
		slot, resource string
		scopes         []string
	}{
		{"gmail", GoogleProvider(), false, GoogleMailAPIClientConfigID, "https://www.googleapis.com", []string{"https://www.googleapis.com/auth/gmail.modify", "https://www.googleapis.com/auth/userinfo.email", "openid"}},
		{"graph", MicrosoftProvider(), false, MicrosoftMailAPIClientConfigID, "https://graph.microsoft.com", []string{"https://graph.microsoft.com/Mail.ReadWrite", "https://graph.microsoft.com/Mail.Send", "offline_access", "openid", "email"}},
		{"graph shared", MicrosoftProvider(), true, MicrosoftMailAPIClientConfigID, "https://graph.microsoft.com", []string{"https://graph.microsoft.com/Mail.ReadWrite", "https://graph.microsoft.com/Mail.Send", "https://graph.microsoft.com/Mail.ReadWrite.Shared", "https://graph.microsoft.com/Mail.Send.Shared", "offline_access", "openid", "email"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			base := tt.base
			base.ClientID, base.ClientSecret, base.LoginHint = "user-client", "user-secret", "identity@example.test"
			if base.Name == "microsoft" {
				base.AuthURL = strings.Replace(base.AuthURL, "/common/", "/organizations/", 1)
				base.TokenURL = strings.Replace(base.TokenURL, "/common/", "/organizations/", 1)
			}
			original := base
			original.Scopes = append([]string(nil), base.Scopes...)
			profile, err := BuildMailAPIProfile(base, tt.shared)
			if err != nil {
				t.Fatal(err)
			}
			if profile.ClientConfigID != tt.slot || ScopeResource(profile.Provider) != tt.resource || !reflect.DeepEqual(profile.Provider.Scopes, tt.scopes) {
				t.Fatalf("incorrect API profile: %+v", profile)
			}
			want := base
			want.Scopes = tt.scopes
			if !reflect.DeepEqual(profile.Provider, want) || !reflect.DeepEqual(base, original) {
				t.Fatal("profile changed credentials/endpoints/hints or mutated base")
			}
			profile.Provider.Scopes[0] = "modified"
			if !reflect.DeepEqual(base, original) {
				t.Fatal("profile aliases protocol scopes")
			}
		})
	}
	for _, base := range []ProviderConfig{GoogleProvider(), {Name: "custom"}, GoogleContactsOnlyProvider(), MicrosoftCalendarProvider()} {
		if _, err := BuildMailAPIProfile(base, true); err == nil {
			t.Fatalf("unsupported profile accepted: %s", base.Name)
		}
	}
	if !reflect.DeepEqual(SupportedProviders(), []string{"google", "microsoft"}) {
		t.Fatal("API routes activated")
	}
	if ScopeResource(GoogleProvider()) != "https://www.googleapis.com" || ScopeResource(MicrosoftProvider()) != "https://outlook.office.com" {
		t.Fatal("protocol resources changed")
	}
}

func TestMailAPIProfileUsesResolvedCredentials(t *testing.T) {
	prior := UserOverrideLookup
	t.Cleanup(func() { UserOverrideLookup = prior })
	UserOverrideLookup = func(slot string) (ClientCredentials, bool) {
		return ClientCredentials{ClientID: "explicit-client", ClientSecret: "explicit-secret"}, slot == "google-mail" || slot == "microsoft-mail"
	}
	for _, name := range []string{"google", "microsoft"} {
		base, err := GetProvider(name)
		if err != nil {
			t.Fatal(err)
		}
		profile, err := BuildMailAPIProfile(base, false)
		if err != nil {
			t.Fatal(err)
		}
		if profile.Provider.ClientID != "explicit-client" || profile.Provider.ClientSecret != "explicit-secret" {
			t.Fatal("resolved override lost")
		}
	}
}

func TestMailAPIExchangeAndRefresh(t *testing.T) {
	for _, base := range []ProviderConfig{GoogleProvider(), MicrosoftProvider()} {
		t.Run(base.Name, func(t *testing.T) {
			profile, err := BuildMailAPIProfile(base, base.Name == "microsoft")
			if err != nil {
				t.Fatal(err)
			}
			var forms []url.Values
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				forms = append(forms, r.PostForm)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"access_token":"fixture-access","expires_in":3600}`))
			}))
			defer server.Close()
			config := profile.Provider
			config.ClientID, config.TokenURL = "fixture-client", server.URL
			authURL, err := url.Parse(buildAuthURL(config, "state", "challenge", 1234))
			if err != nil {
				t.Fatal(err)
			}
			if authURL.Query().Get("scope") != strings.Join(config.Scopes, " ") || authURL.Query().Get("code_challenge_method") != "S256" {
				t.Fatal("authorization scopes/PKCE lost")
			}
			manager := NewManager()
			response, err := manager.exchangeCode(config, "code", "verifier", 1234)
			if err != nil {
				t.Fatal(err)
			}
			refreshed, err := manager.RefreshTokenWithProvider(config, "retained-refresh")
			if err != nil {
				t.Fatal(err)
			}
			for _, tokens := range []*TokenResponse{response, refreshed} {
				if tokens.Provider != base.Name || tokens.ScopeResource != ScopeResource(profile.Provider) || !reflect.DeepEqual(tokens.RequestedScopes, config.Scopes) || tokens.ScopePresent {
					t.Fatalf("grant context lost: %+v", tokens)
				}
			}
			if refreshed.RefreshToken != "retained-refresh" || forms[1].Has("scope") || forms[1].Get("grant_type") != "refresh_token" || forms[0].Get("code_verifier") != "verifier" {
				t.Fatal("refresh narrowed grant or lost refresh material/PKCE")
			}
		})
	}
}

func TestClassifyMailAPIError(t *testing.T) {
	provider := func(code, description string) error {
		return fmt.Errorf("wrapped: %w", &ProviderError{Code: code, Description: description, message: code})
	}
	for _, tt := range []struct {
		name string
		err  error
		want MailAPIFailure
	}{
		{"none", nil, MailAPIFailureNone},
		{"network", errors.New("request failed"), MailAPIFailureOther},
		{"revoked", provider("invalid_grant", "revoked"), MailAPIFailureReauthorize},
		{"admin consent", provider("invalid_grant", "AADSTS65001: needs consent"), MailAPIFailurePolicy},
		{"conditional access", provider("interaction_required", "AADSTS53003 blocked"), MailAPIFailurePolicy},
		{"denied", provider("access_denied", ""), MailAPIFailurePolicy},
		{"unknown resource", provider("invalid_resource", "AADSTS500011: resource not found"), MailAPIFailureResource},
		{"scope", provider("invalid_scope", ""), MailAPIFailureResource},
		{"other code", provider("temporarily_unavailable", ""), MailAPIFailureOther},
	} {
		if got := ClassifyMailAPIError(tt.err); got != tt.want {
			t.Errorf("%s: got %q want %q", tt.name, got, tt.want)
		}
	}
}

func TestProviderErrorsKeepInvalidGrant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"AADSTS65001 consent"}`))
	}))
	defer server.Close()
	config := GoogleProvider()
	config.ClientID, config.TokenURL = "client", server.URL
	_, err := NewManager().RefreshTokenWithProvider(config, "refresh")
	if !errors.Is(err, ErrInvalidGrant) || ClassifyMailAPIError(err) != MailAPIFailurePolicy {
		t.Fatalf("refresh error lost code or grant: %v", err)
	}
	_, err = NewManager().exchangeCode(config, "code", "verifier", 1)
	if ClassifyMailAPIError(err) != MailAPIFailurePolicy || !strings.Contains(err.Error(), "token exchange failed (400)") {
		t.Fatalf("exchange error lost code: %v", err)
	}
}
