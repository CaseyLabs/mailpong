package oauth2

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// Gmail API scopes requested together with https://mail.google.com/, which
// already covers them, so the consent screen need not list them.
var coveredByFullMail = []string{
	"https://www.googleapis.com/auth/gmail.readonly",
	"https://www.googleapis.com/auth/gmail.send",
	"https://www.googleapis.com/auth/gmail.compose",
	"https://www.googleapis.com/auth/gmail.settings.basic",
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestGoogleSetupScopesCoverProviders(t *testing.T) {
	scopes := GoogleSetupScopes()
	seen := map[string]bool{}
	for _, s := range scopes {
		if seen[s] {
			t.Errorf("scope %q listed twice", s)
		}
		seen[s] = true
	}
	for _, p := range []ProviderConfig{GoogleProvider(), GoogleContactsOnlyProvider(), GoogleCalendarProvider()} {
		for _, s := range p.Scopes {
			if !seen[s] {
				t.Errorf("%s requests %q, missing from the setup guide", p.Name, s)
			}
		}
	}
}

// TestGoogleSetupScopesCoverSources fails when Go code requests a Google scope
// the guide neither lists nor documents as covered by the full-mail scope.
func TestGoogleSetupScopesCoverSources(t *testing.T) {
	re := regexp.MustCompile(`"(https://www\.googleapis\.com/auth/[a-z.]+)"`)
	scopes := GoogleSetupScopes()
	root := repoRoot(t)
	for _, dir := range []string{"app", "internal", "extensions"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, m := range re.FindAllStringSubmatch(string(data), -1) {
				if !slices.Contains(scopes, m[1]) && !slices.Contains(coveredByFullMail, m[1]) {
					t.Errorf("%s requests %q, missing from GoogleSetupScopes", strings.TrimPrefix(path, root+"/"), m[1])
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// TestGoogleCredentialsGuideListsScopes keeps the user guide's scope table equal
// to the scope list the app shows.
func TestGoogleCredentialsGuideListsScopes(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs/user-guide/getting-started/first-account/google-oauth-credentials.md"))
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile("(?m)^\\| `([^`]+)` \\|")
	var documented []string
	for _, m := range re.FindAllStringSubmatch(string(data), -1) {
		documented = append(documented, m[1])
	}
	want := GoogleSetupScopes()
	slices.Sort(documented)
	slices.Sort(want)
	if !slices.Equal(documented, want) {
		t.Errorf("guide scope table = %v, want %v", documented, want)
	}
}

func TestGoogleSetupStepsAreConsoleLinks(t *testing.T) {
	steps := GoogleSetupSteps()
	if len(steps) == 0 {
		t.Fatal("no steps")
	}
	for _, s := range steps {
		if !strings.HasPrefix(s.URL, "https://console.cloud.google.com/") || s.FallbackURL == "" || s.MenuPath == "" {
			t.Errorf("step %q incomplete: %+v", s.ID, s)
		}
	}
	for _, api := range []string{"gmail.googleapis.com", "people.googleapis.com", "calendar-json.googleapis.com"} {
		if !strings.Contains(steps[1].URL, api) {
			t.Errorf("enable-APIs link lacks %s", api)
		}
	}
}
