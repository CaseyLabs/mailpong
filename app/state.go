package app

import (
	"github.com/caseylabs/mailven/internal/appstate"
	"github.com/caseylabs/mailven/internal/platform"
	"github.com/caseylabs/mailven/internal/smtp"
)

// ============================================================================
// UI State Persistence
// ============================================================================

// GetUIState retrieves the last saved UI state
func (a *App) GetUIState() (*appstate.UIState, error) {
	return a.appStateStore.GetUIState()
}

// SaveUIState persists the current UI state
func (a *App) SaveUIState(state *appstate.UIState) error {
	return a.appStateStore.SaveUIState(state)
}

// ============================================================================
// App Info API - Exposed to frontend via Wails bindings
// ============================================================================

// Version is the Mailven release version. Bump on each release; consumed by
// the About dialog via GetAppInfo() and by the --version CLI flag in main.go.
// (wails.json, frontend/package.json, and metainfo.xml each carry their own
// version strings for their respective tooling.)
const Version = "0.4.0"

func init() { smtp.ClientVersion = Version }

// AppInfo contains application metadata
type AppInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Website     string `json:"website"`
	License     string `json:"license"`
}

// GetAppInfo returns application metadata for the About dialog
func (a *App) GetAppInfo() AppInfo {
	return AppInfo{
		Name:        "Mailven",
		Version:     Version,
		Description: "Email, reimagined as chat.",
		Website:     "https://github.com/CaseyLabs/mailven",
		License:     "Apache 2.0",
	}
}

// IsFlatpak returns true if the application is running inside a Flatpak sandbox.
func (a *App) IsFlatpak() bool {
	return platform.IsFlatpak()
}

// GetPendingMailto returns and clears any pending mailto: URL data.
// This is used when Mailven is launched with a mailto: URL argument.
func (a *App) GetPendingMailto() *MailtoData {
	data := a.PendingMailto
	a.PendingMailto = nil // Clear after reading
	return data
}
