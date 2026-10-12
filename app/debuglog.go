package app

import (
	"os"

	"github.com/caseylabs/mailven/internal/platform"
)

// maxDebugLogSize is the size at which the debug log is rotated to a single
// ".1" backup at startup.
const maxDebugLogSize = 10 << 20

// debugLogFile returns the path of the persistent debug log, or "" when debug
// logging is off or the path can't be prepared. A log file is optional, so
// failures fall back to console-only logging.
func debugLogFile(debug bool) string {
	if !debug {
		return ""
	}
	paths, err := platform.GetPaths()
	if err != nil {
		return ""
	}
	if err := paths.EnsureDirectories(); err != nil {
		return ""
	}
	path := paths.LogFilePath()
	if info, err := os.Stat(path); err == nil && info.Size() > maxDebugLogSize {
		_ = os.Rename(path, path+".1")
	}
	return path
}
