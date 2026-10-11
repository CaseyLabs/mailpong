package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLinuxAutostartFilename(t *testing.T) {
	if autostartFilename != "io.github.caseylabs.MailPong.desktop" {
		t.Errorf("autostartFilename = %q", autostartFilename)
	}
}

func TestLinuxAutostartLeavesOtherFilesAlone(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	dir := filepath.Join(cfg, "autostart")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	others := []string{
		"io.github.hkdb.Aerion.desktop",
		"io.github.clabs_public_forks.Aerion.desktop",
	}
	for _, name := range others {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("keep"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	m := &linuxAutostartManager{}
	if m.IsEnabled() {
		t.Fatal("IsEnabled() = true before Enable(); other apps' entries must not count")
	}
	if err := m.Enable(); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, autostartFilename)); err != nil {
		t.Fatalf("autostart file not written: %v", err)
	}
	if !m.IsEnabled() {
		t.Error("IsEnabled() = false after Enable()")
	}
	if err := m.Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if m.IsEnabled() {
		t.Error("IsEnabled() = true after Disable()")
	}
	for _, name := range others {
		if b, err := os.ReadFile(filepath.Join(dir, name)); err != nil || string(b) != "keep" {
			t.Errorf("%s was touched (err=%v, content=%q)", name, err, b)
		}
	}
}
