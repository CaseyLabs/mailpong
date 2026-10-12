package platform

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGetPaths(t *testing.T) {
	paths, err := GetPaths()
	if err != nil {
		t.Fatalf("GetPaths() returned error: %v", err)
	}
	if paths == nil {
		t.Fatal("GetPaths() returned nil")
	}
	if paths.Config == "" {
		t.Error("GetPaths().Config is empty")
	}
	if paths.Data == "" {
		t.Error("GetPaths().Data is empty")
	}
	if paths.Cache == "" {
		t.Error("GetPaths().Cache is empty")
	}
}

func TestDatabasePath(t *testing.T) {
	paths, err := GetPaths()
	if err != nil {
		t.Fatalf("GetPaths() returned error: %v", err)
	}
	dbPath := paths.DatabasePath()
	if !strings.HasSuffix(dbPath, "mailven.db") {
		t.Errorf("DatabasePath() = %q, want suffix 'mailven.db'", dbPath)
	}
}

func TestContactsDatabasePath(t *testing.T) {
	paths, err := GetPaths()
	if err != nil {
		t.Fatalf("GetPaths() returned error: %v", err)
	}
	contactsPath := paths.ContactsDatabasePath()
	if !strings.HasSuffix(contactsPath, "contacts.db") {
		t.Errorf("ContactsDatabasePath() = %q, want suffix 'contacts.db'", contactsPath)
	}
}

func TestSearchIndexPath(t *testing.T) {
	paths, err := GetPaths()
	if err != nil {
		t.Fatalf("GetPaths() returned error: %v", err)
	}
	accountID := "test-account-123"
	indexPath := paths.SearchIndexPath(accountID)
	if !strings.Contains(indexPath, accountID) {
		t.Errorf("SearchIndexPath() = %q, want to contain %q", indexPath, accountID)
	}
}

func TestKeyringPath(t *testing.T) {
	paths, err := GetPaths()
	if err != nil {
		t.Fatalf("GetPaths() returned error: %v", err)
	}
	keyringPath := paths.KeyringPath()
	if !strings.HasSuffix(keyringPath, "keys") {
		t.Errorf("KeyringPath() = %q, want suffix 'keys'", keyringPath)
	}
}

func TestIsFlatpak(t *testing.T) {
	tests := []struct {
		name      string
		flatpakID string
		want      bool
	}{
		{
			name:      "not flatpak by default",
			flatpakID: "",
			want:      false,
		},
		{
			name:      "flatpak when FLATPAK_ID is set",
			flatpakID: "com.example.App",
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("FLATPAK_ID", tt.flatpakID)
			got := IsFlatpak()
			if got != tt.want {
				t.Errorf("IsFlatpak() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOSPathsUseMailvenNames(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	for _, k := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "APPDATA", "LOCALAPPDATA"} {
		t.Setenv(k, "")
	}

	tests := []struct {
		name                string
		fn                  func() (*Paths, error)
		config, data, cache string
	}{
		{"linux", getLinuxPaths,
			filepath.Join(home, ".config", "mailven"),
			filepath.Join(home, ".local", "share", "mailven"),
			filepath.Join(home, ".cache", "mailven")},
		{"darwin", getDarwinPaths,
			filepath.Join(home, "Library", "Application Support", "Mailven"),
			filepath.Join(home, "Library", "Application Support", "Mailven"),
			filepath.Join(home, "Library", "Caches", "Mailven")},
		{"windows", getWindowsPaths,
			filepath.Join(home, "AppData", "Roaming", "Mailven"),
			filepath.Join(home, "AppData", "Roaming", "Mailven"),
			filepath.Join(home, "AppData", "Local", "Mailven", "Cache")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := tt.fn()
			if err != nil {
				t.Fatalf("paths: %v", err)
			}
			if p.Config != tt.config || p.Data != tt.data || p.Cache != tt.cache {
				t.Errorf("got %+v, want config=%q data=%q cache=%q", *p, tt.config, tt.data, tt.cache)
			}
			for _, dir := range []string{p.Config, p.Data, p.Cache, p.DatabasePath()} {
				rel := strings.TrimPrefix(dir, home)
				if strings.Contains(strings.ToLower(rel), "aerion") {
					t.Errorf("path %q contains aerion", dir)
				}
			}
			if got := filepath.Base(p.DatabasePath()); got != "mailven.db" {
				t.Errorf("database file = %q, want mailven.db", got)
			}
		})
	}
}
