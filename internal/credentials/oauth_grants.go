package credentials

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/caseylabs/mailven/internal/oauth2"
)

// OAuthGrant records scope evidence independently of the authorization request.
// The existing scopes column stores this versioned object; old arrays are requests,
// never evidence of consent. Provider and client-config remain on the token row.
type OAuthGrant struct {
	Version              int      `json:"version"`
	Granted              []string `json:"granted"`
	Requested            []string `json:"requested"`
	Known                bool     `json:"known"`
	Resource             string   `json:"resource"`
	MicrosoftAccountType string   `json:"microsoftAccountType,omitempty"`
}

func marshalOAuthGrant(t *OAuthTokens) ([]byte, error) {
	return json.Marshal(OAuthGrant{1, t.Scopes, t.RequestedScopes, t.ScopesKnown || t.Scopes != nil, t.ScopeResource, t.MicrosoftAccountType})
}

func readOAuthGrant(raw string, t *OAuthTokens) {
	var grant OAuthGrant
	if json.Unmarshal([]byte(raw), &grant) == nil && grant.Version == 1 {
		t.Scopes, t.RequestedScopes, t.ScopesKnown, t.ScopeResource, t.MicrosoftAccountType = grant.Granted, grant.Requested, grant.Known, grant.Resource, grant.MicrosoftAccountType
		if !grant.Known {
			t.Scopes = nil
		}
		return
	}
	// Legacy metadata recorded requested scopes, even when consent was partial.
	_ = json.Unmarshal([]byte(raw), &t.RequestedScopes)
}

// ApplyTokenResponse handles authorization and refresh without conflating requests
// with grants. A refresh omitting scope retains prior known evidence (RFC 6749
// section 6: this client sends no narrowing scope). A new authorization omitting
// scope is recorded as unknown rather than inferring consent from a static config.
func (t *OAuthTokens) ApplyTokenResponse(response *oauth2.TokenResponse, requested []string, resource string, refresh bool) {
	t.AccessToken, t.ExpiresAt = response.AccessToken, response.ExpiresAt()
	if response.RefreshToken != "" {
		t.RefreshToken = response.RefreshToken
	}
	if !refresh {
		if response.RequestedScopes != nil {
			requested = response.RequestedScopes
		}
		if response.ScopeResource != "" {
			resource = response.ScopeResource
		}
		t.RequestedScopes = append([]string(nil), requested...)
		t.ScopeResource = resource
		t.Scopes, t.ScopesKnown = nil, false
	}
	if response.ScopePresent || response.Scope != "" {
		if response.ScopeResource != "" {
			t.ScopeResource = response.ScopeResource
		}
		t.Scopes = append([]string{}, strings.Fields(response.Scope)...)
		t.ScopesKnown = true
	}
}

// RequireMailCapability preserves unknown legacy routes; a known partial grant
// blocks only the missing protocol. The provider still enforces policy and consent.
func (t *OAuthTokens) RequireMailCapability(protocol string) error {
	if !t.ScopesKnown {
		return nil
	}
	var required string
	switch t.Provider {
	case "google":
		required = "https://mail.google.com/"
	case "microsoft":
		if protocol == "imap" {
			required = "https://outlook.office.com/IMAP.AccessAsUser.All"
		} else {
			required = "https://outlook.office.com/SMTP.Send"
		}
	default:
		return nil // Custom scopes are opaque; the server enforces them.
	}
	if t.HasScope(required) {
		return nil
	}
	return fmt.Errorf("%s authorization does not include %s access; reauthorize this account and grant the missing permission", t.Provider, protocol)
}

func (t *OAuthTokens) HasScope(required string) bool {
	for _, scope := range t.EffectiveScopes() {
		if scope == required {
			return true
		}
	}
	return false
}

// EffectiveScopes qualifies Microsoft's relative scope names using recorded
// resource context while retaining the original token-response values in storage.
func (t *OAuthTokens) EffectiveScopes() []string {
	scopes := append([]string(nil), t.Scopes...)
	if strings.HasPrefix(t.Provider, "microsoft") && (t.ScopeResource == "https://outlook.office.com" || t.ScopeResource == "https://graph.microsoft.com") {
		for _, scope := range t.Scopes {
			if !strings.Contains(scope, "/") {
				scopes = append(scopes, t.ScopeResource+"/"+scope)
			}
		}
	}
	return scopes
}

// RequireContactsRead reports missing or unknown contacts evidence independently
// of mail access. Custom CardDAV grants are opaque and remain server-enforced.
func (t *OAuthTokens) RequireContactsRead() error {
	var scope string
	switch t.Provider {
	case "google", "google-contacts":
		scope = "https://www.googleapis.com/auth/contacts.readonly"
	case "microsoft", "microsoft-contacts":
		scope = "https://graph.microsoft.com/Contacts.Read"
	default:
		return nil
	}
	if t.HasScope(scope) {
		return nil
	}
	return fmt.Errorf("contacts authorization is missing or unknown; authorize this contact source separately")
}
