package platform

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/caseylabs/mailven/internal/logging"
	"github.com/godbus/dbus/v5"
)

// FileFilter is one file-type filter for PortalOpenFiles. Pattern holds
// semicolon-separated globs, as in Wails dialog filters ("*.pem;*.crt").
type FileFilter struct {
	DisplayName string
	Pattern     string
}

// portalFilterRule and portalFilter marshal to the portal's a(sa(us)) filter type.
type portalFilterRule struct {
	Kind    uint32 // 0 = glob pattern
	Pattern string
}

type portalFilter struct {
	Name  string
	Rules []portalFilterRule
}

// PortalSaveFile shows the XDG FileChooser portal Save dialog for a single file.
// Returns the chosen path, or ("", nil) if the user cancelled.
func PortalSaveFile(title, suggestedName, directory string) (string, error) {
	options := map[string]dbus.Variant{
		"current_name": dbus.MakeVariant(suggestedName),
	}
	if directory != "" {
		// current_folder is type "ay" (byte array), null-terminated
		options["current_folder"] = dbus.MakeVariant(append([]byte(directory), 0))
	}
	paths, err := portalFileChooser("SaveFile", title, options, 60*time.Second)
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}

// PortalSaveFiles shows the XDG FileChooser portal SaveFiles dialog for multiple files.
// Returns the list of chosen paths, or (nil, nil) if the user cancelled.
func PortalSaveFiles(title string, filenames []string, directory string) ([]string, error) {
	// Build files as aay (array of null-terminated byte arrays)
	files := make([][]byte, len(filenames))
	for i, name := range filenames {
		files[i] = append([]byte(name), 0)
	}

	options := map[string]dbus.Variant{
		"files": dbus.MakeVariant(files),
	}
	if directory != "" {
		options["current_folder"] = dbus.MakeVariant(append([]byte(directory), 0))
	}
	return portalFileChooser("SaveFiles", title, options, 60*time.Second)
}

// PortalOpenFiles shows the XDG FileChooser portal Open dialog. Inside
// Flatpak this is the only picker that can see host files: the portal grants
// the sandbox access to each chosen file. Returns the chosen paths, or
// (nil, nil) if the user cancelled.
func PortalOpenFiles(title string, filters []FileFilter, multiple bool) ([]string, error) {
	options := map[string]dbus.Variant{
		"multiple": dbus.MakeVariant(multiple),
		"modal":    dbus.MakeVariant(true),
	}
	if pf := portalFilters(filters); len(pf) > 0 {
		options["filters"] = dbus.MakeVariant(pf)
	}
	// Browsing for a file to open can take a while; only give up on a stuck portal.
	return portalFileChooser("OpenFile", title, options, 30*time.Minute)
}

// portalFilters converts filters to the portal's form, splitting each
// semicolon-separated pattern into glob rules and dropping empty filters.
func portalFilters(filters []FileFilter) []portalFilter {
	var pf []portalFilter
	for _, f := range filters {
		var rules []portalFilterRule
		for _, glob := range strings.Split(f.Pattern, ";") {
			if glob = strings.TrimSpace(glob); glob != "" {
				rules = append(rules, portalFilterRule{Kind: 0, Pattern: glob})
			}
		}
		if len(rules) > 0 {
			pf = append(pf, portalFilter{Name: f.DisplayName, Rules: rules})
		}
	}
	return pf
}

// portalFileChooser calls org.freedesktop.portal.FileChooser.<method> and
// waits for its Response. Returns the chosen paths, or (nil, nil) if the user
// cancelled.
func portalFileChooser(method, title string, options map[string]dbus.Variant, timeout time.Duration) ([]string, error) {
	log := logging.WithComponent("filechooser")

	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to session bus: %w", err)
	}
	// Don't close conn — it's the shared session bus used by GTK/WebKit

	// Check if the portal service is actually running
	var hasOwner bool
	if err := conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, "org.freedesktop.portal.Desktop").Store(&hasOwner); err != nil || !hasOwner {
		return nil, fmt.Errorf("portal service not available")
	}

	handleToken := fmt.Sprintf("mailven_%d", time.Now().UnixNano())

	// Compute expected request path from our unique bus name and handle token
	sender := conn.Names()[0]
	senderPath := strings.ReplaceAll(sender[1:], ".", "_")
	requestPath := dbus.ObjectPath(fmt.Sprintf(
		"/org/freedesktop/portal/desktop/request/%s/%s", senderPath, handleToken,
	))

	// Subscribe to Response signal BEFORE calling the method to avoid races
	matchRule := fmt.Sprintf(
		"type='signal',interface='org.freedesktop.portal.Request',member='Response',path='%s'",
		requestPath,
	)
	if err := conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, matchRule).Err; err != nil {
		return nil, fmt.Errorf("failed to subscribe to portal response: %w", err)
	}
	defer conn.BusObject().Call("org.freedesktop.DBus.RemoveMatch", 0, matchRule)

	signals := make(chan *dbus.Signal, 1)
	conn.Signal(signals)
	defer conn.RemoveSignal(signals)

	options["handle_token"] = dbus.MakeVariant(handleToken)

	obj := conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")
	call := obj.Call("org.freedesktop.portal.FileChooser."+method, 0, "", title, options)
	if call.Err != nil {
		return nil, fmt.Errorf("%s portal call failed: %w", method, call.Err)
	}

	// Wait for the Response signal
	deadline := time.After(timeout)
	for {
		select {
		case signal := <-signals:
			if signal == nil {
				continue
			}
			if signal.Path != requestPath || signal.Name != "org.freedesktop.portal.Request.Response" {
				continue
			}
			if len(signal.Body) < 2 {
				return nil, fmt.Errorf("incomplete response from FileChooser portal")
			}
			response, ok := signal.Body[0].(uint32)
			if !ok {
				return nil, fmt.Errorf("unexpected response type from FileChooser portal")
			}
			// 1 = user cancelled
			if response == 1 {
				log.Debug().Str("method", method).Msg("User cancelled file chooser")
				return nil, nil
			}
			if response != 0 {
				return nil, fmt.Errorf("%s dialog failed (response: %d)", method, response)
			}

			results, ok := signal.Body[1].(map[string]dbus.Variant)
			if !ok {
				return nil, fmt.Errorf("unexpected results type from FileChooser portal")
			}

			uris, ok := results["uris"].Value().([]string)
			if !ok || len(uris) == 0 {
				return nil, fmt.Errorf("no URIs in FileChooser portal response")
			}

			paths := make([]string, len(uris))
			for i, uri := range uris {
				p, err := uriToPath(uri)
				if err != nil {
					return nil, err
				}
				paths[i] = p
			}

			log.Debug().Str("method", method).Int("count", len(paths)).Msg("Portal file chooser paths selected")
			return paths, nil

		case <-deadline:
			return nil, fmt.Errorf("FileChooser portal request timed out")
		}
	}
}

// PortalOpenDirectory opens the folder containing a file using the OpenURI portal.
// This resolves sandboxed document portal paths to the real host location.
func PortalOpenDirectory(filePath string) error {
	log := logging.WithComponent("filechooser")

	conn, err := dbus.SessionBus()
	if err != nil {
		return fmt.Errorf("failed to connect to session bus: %w", err)
	}

	// Check if the portal service is actually running
	var hasOwner bool
	if err := conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, "org.freedesktop.portal.Desktop").Store(&hasOwner); err != nil || !hasOwner {
		return fmt.Errorf("portal service not available")
	}

	// Open the file to get a file descriptor — the portal uses the fd
	// to resolve the real host path even from inside the sandbox
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %q: %w", filePath, err)
	}
	defer f.Close()

	obj := conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")
	options := map[string]dbus.Variant{}

	call := obj.Call("org.freedesktop.portal.OpenURI.OpenDirectory", 0, "", dbus.UnixFD(f.Fd()), options)
	if call.Err != nil {
		return fmt.Errorf("OpenDirectory portal call failed: %w", call.Err)
	}

	log.Debug().Str("path", filePath).Msg("Opened directory via portal")
	return nil
}

// IsDocPortalPath checks if a path is on the document portal FUSE mount.
func IsDocPortalPath(path string) bool {
	return strings.HasPrefix(path, "/run/user/") && strings.Contains(path, "/doc/")
}

// PortalOpenFile opens a file with the default application using the OpenURI portal.
func PortalOpenFile(filePath string) error {
	log := logging.WithComponent("filechooser")

	conn, err := dbus.SessionBus()
	if err != nil {
		return fmt.Errorf("failed to connect to session bus: %w", err)
	}

	var hasOwner bool
	if err := conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, "org.freedesktop.portal.Desktop").Store(&hasOwner); err != nil || !hasOwner {
		return fmt.Errorf("portal service not available")
	}

	f, err := os.OpenFile(filePath, os.O_RDWR, 0)
	if err != nil {
		f, err = os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open file %q: %w", filePath, err)
		}
	}
	defer f.Close()

	obj := conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")
	options := map[string]dbus.Variant{
		"writable": dbus.MakeVariant(true),
	}
	call := obj.Call("org.freedesktop.portal.OpenURI.OpenFile", 0, "", dbus.UnixFD(f.Fd()), options)
	if call.Err != nil {
		return fmt.Errorf("OpenFile portal call failed: %w", call.Err)
	}

	log.Debug().Str("path", filePath).Msg("Opened file via portal")
	return nil
}

// PortalOpenURI opens a URI with the user's default handler via the XDG
// OpenURI portal. Works inside the Flatpak sandbox (unlike xdg-open) and
// triggers the host's URL-handler notification on Wayland DEs.
// Callers should fall back to wailsRuntime.BrowserOpenURL on error.
func PortalOpenURI(uri string) error {
	log := logging.WithComponent("filechooser")

	conn, err := dbus.SessionBus()
	if err != nil {
		return fmt.Errorf("failed to connect to session bus: %w", err)
	}

	var hasOwner bool
	if err := conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, "org.freedesktop.portal.Desktop").Store(&hasOwner); err != nil || !hasOwner {
		return fmt.Errorf("portal service not available")
	}

	obj := conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")
	options := map[string]dbus.Variant{}
	call := obj.Call("org.freedesktop.portal.OpenURI.OpenURI", 0, "", uri, options)
	if call.Err != nil {
		return fmt.Errorf("OpenURI portal call failed: %w", call.Err)
	}

	log.Debug().Str("uri", uri).Msg("Opened URI via portal")
	return nil
}

// uriToPath converts a file:// URI to a filesystem path.
func uriToPath(uri string) (string, error) {
	parsed, err := url.Parse(uri)
	if err != nil {
		return "", fmt.Errorf("failed to parse URI %q: %w", uri, err)
	}
	if parsed.Scheme != "file" {
		return "", fmt.Errorf("unexpected URI scheme %q (expected file://)", parsed.Scheme)
	}
	return parsed.Path, nil
}
