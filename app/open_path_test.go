package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/caseylabs/mailven/internal/platform"
)

func TestValidateOpenPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	a := &App{paths: &platform.Paths{Data: filepath.Join(home, "data")}}

	write := func(p string) string {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	attachment := write(filepath.Join(a.paths.AttachmentsPath(), "a.pdf"))
	download := write(filepath.Join(home, "Downloads", "b.pdf"))
	db := write(filepath.Join(a.paths.Data, "mailven.db"))
	outside := write(filepath.Join(t.TempDir(), "c.sh"))
	link := filepath.Join(home, "Downloads", "innocent.pdf")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
		ok   bool
	}{
		{"cached attachment", attachment, true},
		{"downloads file", download, true},
		{"downloads folder", filepath.Dir(download), true},
		{"database in data dir", db, false},
		{"outside file", outside, false},
		{"symlink out of downloads", link, false},
		{"dot-dot out of downloads", filepath.Join(home, "Downloads", "..", "data", "mailven.db"), false},
		{"missing file", filepath.Join(home, "Downloads", "nope.pdf"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := a.validateOpenPath(tt.path); (err == nil) != tt.ok {
				t.Errorf("validateOpenPath(%q) err = %v, want ok=%v", tt.path, err, tt.ok)
			}
		})
	}
}
