//go:build linux || darwin

package platform

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// SocketDir returns the private per-user directory for MailPong's Unix
// sockets: $XDG_RUNTIME_DIR/mailpong when set, else <tmp>/mailpong-<uid>. The
// directory must be a real directory owned by the current user, so another
// user can't pre-create it to intercept the sockets; its mode is set to 0700.
func SocketDir() (string, error) {
	base, name := os.Getenv("XDG_RUNTIME_DIR"), "mailpong"
	if base == "" {
		base, name = os.TempDir(), fmt.Sprintf("mailpong-%d", os.Getuid())
	}
	dir := filepath.Join(base, name)
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return "", fmt.Errorf("create socket directory: %w", err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return "", fmt.Errorf("stat socket directory: %w", err)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !info.IsDir() || !ok || int(st.Uid) != os.Getuid() {
		return "", fmt.Errorf("socket directory %s is not a directory owned by this user", dir)
	}
	if info.Mode().Perm() != 0o700 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return "", fmt.Errorf("set socket directory permissions: %w", err)
		}
	}
	return dir, nil
}
