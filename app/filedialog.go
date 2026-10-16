package app

import (
	"context"

	"github.com/caseylabs/mailven/internal/logging"
	"github.com/caseylabs/mailven/internal/platform"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// openFileDialog shows a single-file Open dialog and returns the chosen path
// ("" when cancelled).
func openFileDialog(ctx context.Context, opts wailsRuntime.OpenDialogOptions) (string, error) {
	if platform.IsFlatpak() {
		if paths, ok := portalOpenFiles(opts, false); ok {
			if len(paths) == 0 {
				return "", nil
			}
			return paths[0], nil
		}
	}
	return wailsRuntime.OpenFileDialog(ctx, opts)
}

// openMultipleFilesDialog shows a multi-select Open dialog and returns the
// chosen paths (nil when cancelled).
func openMultipleFilesDialog(ctx context.Context, opts wailsRuntime.OpenDialogOptions) ([]string, error) {
	if platform.IsFlatpak() {
		if paths, ok := portalOpenFiles(opts, true); ok {
			return paths, nil
		}
	}
	return wailsRuntime.OpenMultipleFilesDialog(ctx, opts)
}

// portalOpenFiles routes an Open dialog through the XDG FileChooser portal.
// In Flatpak the Wails GTK dialog runs inside the sandbox and cannot see host
// files; the portal shows the desktop's own picker and grants access to the
// chosen files. ok is false when the portal failed and the caller should fall
// back to the Wails dialog.
func portalOpenFiles(opts wailsRuntime.OpenDialogOptions, multiple bool) (paths []string, ok bool) {
	filters := make([]platform.FileFilter, len(opts.Filters))
	for i, f := range opts.Filters {
		filters[i] = platform.FileFilter{DisplayName: f.DisplayName, Pattern: f.Pattern}
	}
	paths, err := platform.PortalOpenFiles(opts.Title, filters, multiple)
	if err != nil {
		log := logging.WithComponent("filedialog")
		log.Warn().Err(err).Msg("Portal open dialog failed, falling back to GTK dialog")
		return nil, false
	}
	return paths, true
}
