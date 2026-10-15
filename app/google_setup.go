package app

import "github.com/caseylabs/mailven/internal/oauth2"

// GetGoogleSetupGuide returns the console links and the scope list for the
// in-app Google Cloud setup checklist.
//
// Wails-bound.
func (a *App) GetGoogleSetupGuide() oauth2.GoogleSetupGuide {
	return oauth2.GetGoogleSetupGuide()
}
