package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/caseylabs/mailven/internal/logging"
	"github.com/caseylabs/mailven/internal/oauth2"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// googleImportSlots are the config ids a Google client file may be applied
// to: the Mail, Contacts and Calendar OAuth slots.
var googleImportSlots = []string{"google-mail", "google-contacts", "google-calendar"}

// OAuthClientImportResult reports a finished import. It never carries the
// client secret.
type OAuthClientImportResult struct {
	Slots               []string `json:"slots"`
	ClientIDFingerprint string   `json:"clientIdFingerprint"`
}

// PickGoogleClientFile opens a file picker for Google's downloaded
// client_secret JSON and returns the chosen path ("" when cancelled).
//
// Wails-bound.
func (a *App) PickGoogleClientFile() (string, error) {
	path, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select Google client JSON",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "JSON files (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to open file dialog: %w", err)
	}
	return path, nil
}

// ImportGoogleClientFile reads a Desktop-app client_secret JSON from path and
// stores its client as the Custom credentials of each listed slot. Nothing
// changes unless the file is valid. The secret goes straight to the
// credential store and is never returned or logged.
//
// Wails-bound.
func (a *App) ImportGoogleClientFile(path string, configIDs []string) (*OAuthClientImportResult, error) {
	if a.credStore == nil {
		return nil, fmt.Errorf("credential store not initialized")
	}
	if len(configIDs) == 0 {
		return nil, fmt.Errorf("choose at least one slot to apply the client to")
	}
	for _, id := range configIDs {
		if !slices.Contains(googleImportSlots, id) {
			return nil, fmt.Errorf("slot %q cannot take a Google client file", id)
		}
	}

	client, err := readGoogleClientFile(path)
	if err != nil {
		return nil, err
	}

	var saved []string
	for _, id := range configIDs {
		if err := a.credStore.SetUserClientCreds(id, client.ClientID, client.ClientSecret); err != nil {
			return nil, fmt.Errorf("saved %d of %d slots, then %s failed: %w", len(saved), len(configIDs), id, err)
		}
		if err := a.credStore.SetOAuthActiveChoice(id, "custom"); err != nil {
			return nil, fmt.Errorf("saved %d of %d slots, then %s failed: %w", len(saved), len(configIDs), id, err)
		}
		saved = append(saved, id)
	}
	log := logging.WithComponent("app")
	log.Info().Strs("slots", saved).Msg("imported Google OAuth client file")
	return &OAuthClientImportResult{Slots: saved, ClientIDFingerprint: fingerprintClientID(true, client.ClientID)}, nil
}

// readGoogleClientFile reads at most MaxClientJSONSize bytes and parses them.
func readGoogleClientFile(path string) (oauth2.GoogleClient, error) {
	f, err := os.Open(path)
	if err != nil {
		return oauth2.GoogleClient{}, fmt.Errorf("could not open the file: %w", errors.Unwrap(err))
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, oauth2.MaxClientJSONSize+1))
	if err != nil {
		return oauth2.GoogleClient{}, fmt.Errorf("could not read the file")
	}
	return oauth2.ParseGoogleClientJSON(data)
}
