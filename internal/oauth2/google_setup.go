package oauth2

import "slices"

// Google scopes that Mailven requests only on request, so no provider's
// default Scopes list carries them.
const (
	// ScopeGmailModify is requested when a Gmail account migrates to the API route.
	ScopeGmailModify = "https://www.googleapis.com/auth/gmail.modify"
	// ScopeContactsWrite is requested when the Contacts extension enables write access.
	ScopeContactsWrite = "https://www.googleapis.com/auth/contacts"
)

// GoogleSetupStep is one console page in the guided Google Cloud setup. URL is
// the deep link; FallbackURL is the console home, used when the deep link does
// not land on the right page; MenuPath names where to click from there.
type GoogleSetupStep struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	FallbackURL string `json:"fallbackUrl"`
	MenuPath    string `json:"menuPath"`
}

// GoogleSetupGuide is the data behind the in-app checklist. Scopes is generated
// from the provider configurations, so it cannot drift from what Mailven requests.
type GoogleSetupGuide struct {
	Steps  []GoogleSetupStep `json:"steps"`
	Scopes []string          `json:"scopes"`
}

const googleConsoleHome = "https://console.cloud.google.com/"

// GoogleSetupSteps returns the console pages of the guided setup, in order.
// Steps without a console page (importing, signing in) are not listed.
func GoogleSetupSteps() []GoogleSetupStep {
	step := func(id, url, menu string) GoogleSetupStep {
		return GoogleSetupStep{ID: id, URL: url, FallbackURL: googleConsoleHome, MenuPath: menu}
	}
	return []GoogleSetupStep{
		step("project", googleConsoleHome+"projectcreate", "Project picker → New Project"),
		step("apis", googleConsoleHome+"flows/enableapi?apiid=gmail.googleapis.com,people.googleapis.com,calendar-json.googleapis.com", "APIs & Services → Library"),
		step("branding", googleConsoleHome+"auth/branding", "Google Auth Platform → Branding"),
		step("audience", googleConsoleHome+"auth/audience", "Google Auth Platform → Audience"),
		step("scopes", googleConsoleHome+"auth/scopes", "Google Auth Platform → Data Access"),
		step("clients", googleConsoleHome+"auth/clients", "Google Auth Platform → Clients"),
	}
}

// GoogleSetupScopes returns every Google scope to list under Data Access:
// those the Mail, Contacts and Calendar providers request by default, plus the
// two requested on demand. Order follows first use; no scope repeats.
func GoogleSetupScopes() []string {
	var scopes []string
	for _, p := range []ProviderConfig{GoogleProvider(), GoogleContactsOnlyProvider(), GoogleCalendarProvider()} {
		for _, s := range p.Scopes {
			if !slices.Contains(scopes, s) {
				scopes = append(scopes, s)
			}
		}
	}
	return append(scopes, ScopeGmailModify, ScopeContactsWrite)
}

// GetGoogleSetupGuide assembles the checklist data.
func GetGoogleSetupGuide() GoogleSetupGuide {
	return GoogleSetupGuide{Steps: GoogleSetupSteps(), Scopes: GoogleSetupScopes()}
}
