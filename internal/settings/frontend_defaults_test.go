package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The backend-free UI test mode (.agents/skills/run-mailven/fake-backend.js)
// answers settings getters from this fixture instead of a real store.
const frontendDefaultsFixture = "../../.agents/skills/run-mailven/settings-defaults.json"

// TestFrontendFakeDefaults keeps that fixture equal to a fresh store's values.
// Regenerate it with UPDATE_SETTINGS_DEFAULTS=1 go test ./internal/settings -run TestFrontendFakeDefaults.
func TestFrontendFakeDefaults(t *testing.T) {
	store := reflect.ValueOf(NewStore(openTestDB(t)))
	errType := reflect.TypeFor[error]()
	got := map[string]any{}
	for i := range store.NumMethod() {
		m := store.Type().Method(i)
		ft := m.Func.Type()
		if !strings.HasPrefix(m.Name, "Get") || ft.NumIn() != 1 || ft.NumOut() != 2 || ft.Out(1) != errType {
			continue
		}
		out := store.Method(i).Call(nil)
		if err, _ := out[1].Interface().(error); err != nil {
			t.Fatalf("%s: %v", m.Name, err)
		}
		got[m.Name] = out[0].Interface()
	}
	data, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	path := filepath.FromSlash(frontendDefaultsFixture)
	if os.Getenv("UPDATE_SETTINGS_DEFAULTS") != "" {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != string(data) {
		t.Fatalf("%s is stale; regenerate with UPDATE_SETTINGS_DEFAULTS=1 go test ./internal/settings -run TestFrontendFakeDefaults", frontendDefaultsFixture)
	}
}
