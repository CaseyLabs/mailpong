//go:build linux || darwin

package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSocketDir(t *testing.T) {
	tests := []struct {
		name    string
		xdg     bool
		setup   func(t *testing.T, dir string) // prepares the expected dir
		wantErr bool
	}{
		{"xdg runtime dir", true, nil, false},
		{"tmp fallback", false, nil, false},
		{"loose mode tightened", true, func(t *testing.T, dir string) {
			if err := os.Mkdir(dir, 0o777); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(dir, 0o777); err != nil {
				t.Fatal(err)
			}
		}, false},
		{"planted symlink rejected", true, func(t *testing.T, dir string) {
			if err := os.Symlink(t.TempDir(), dir); err != nil {
				t.Fatal(err)
			}
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := t.TempDir()
			want := filepath.Join(base, "mailven")
			t.Setenv("XDG_RUNTIME_DIR", base)
			if !tt.xdg {
				t.Setenv("XDG_RUNTIME_DIR", "")
				t.Setenv("TMPDIR", base)
				want = filepath.Join(base, fmt.Sprintf("mailven-%d", os.Getuid()))
			}
			if tt.setup != nil {
				tt.setup(t, want)
			}
			got, err := SocketDir()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("SocketDir() = %q, want error", got)
				}
				return
			}
			if err != nil || got != want {
				t.Fatalf("SocketDir() = %q, %v; want %q", got, err, want)
			}
			info, err := os.Stat(got)
			if err != nil || info.Mode().Perm() != 0o700 {
				t.Errorf("mode = %v, %v; want 0700", info.Mode().Perm(), err)
			}
		})
	}
}
