package oauth2

import "fmt"

const (
	GoogleMailAPIClientConfigID    = "google-mail-api"
	MicrosoftMailAPIClientConfigID = "microsoft-mail-api"
)

// MailAPIProfile keeps an API grant separate from the existing protocol grant.
// Callers must use ClientConfigID with the explicit per-slot credential APIs.
type MailAPIProfile struct {
	ClientConfigID string
	Provider       ProviderConfig
}

// BuildMailAPIProfile changes only the scopes of an already resolved config.
// In particular, it preserves user credentials and the registered tenant/audience
// endpoints. It does not register a provider, start consent or select an account
// route. Shared means access to another mailbox, not an ordinary send-as alias.
func BuildMailAPIProfile(base ProviderConfig, shared bool) (MailAPIProfile, error) {
	profile := MailAPIProfile{Provider: base}
	switch base.Name {
	case "google":
		if shared {
			return MailAPIProfile{}, fmt.Errorf("Gmail delegated-mailbox API access requires a separately supported authorization model")
		}
		profile.ClientConfigID = GoogleMailAPIClientConfigID
		profile.Provider.Scopes = []string{
			"https://www.googleapis.com/auth/gmail.modify",
			"https://www.googleapis.com/auth/userinfo.email",
			"openid",
		}
	case "microsoft":
		profile.ClientConfigID = MicrosoftMailAPIClientConfigID
		profile.Provider.Scopes = []string{
			"https://graph.microsoft.com/Mail.ReadWrite",
			"https://graph.microsoft.com/Mail.Send",
		}
		if shared {
			profile.Provider.Scopes = append(profile.Provider.Scopes,
				"https://graph.microsoft.com/Mail.ReadWrite.Shared",
				"https://graph.microsoft.com/Mail.Send.Shared")
		}
		profile.Provider.Scopes = append(profile.Provider.Scopes, "offline_access", "openid", "email")
	default:
		return MailAPIProfile{}, fmt.Errorf("mail API profile is unsupported for provider %q", base.Name)
	}
	return profile, nil
}
