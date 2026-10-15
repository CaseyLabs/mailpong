package oauth2

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestParseGoogleClientJSON(t *testing.T) {
	const secret = "GOCSPX-supersecretvalue"
	valid := `{"installed":{"client_id":"123-abc.apps.googleusercontent.com","project_id":"p","client_secret":"` + secret + `","redirect_uris":["http://localhost"]}}`
	tests := []struct {
		name    string
		in      string
		wantErr string
	}{
		{"desktop", valid, ""},
		{"bom", "\xef\xbb\xbf" + valid, ""},
		{"web", `{"web":{"client_id":"a","client_secret":"` + secret + `"}}`, "Web application"},
		{"other type", `{"service_account":{"client_id":"a","client_secret":"` + secret + `"}}`, "no \"installed\""},
		{"malformed", `{"installed":`, "not valid JSON"},
		{"not object", `["` + secret + `"]`, "not valid JSON"},
		{"installed wrong type", `{"installed":"` + secret + `"}`, "malformed"},
		{"no id", `{"installed":{"client_secret":"` + secret + `"}}`, "no client_id"},
		{"no secret", `{"installed":{"client_id":"a"}}`, "no client_secret"},
		{"non-utf8", "{\"installed\":{\"client_id\":\"a\xff\",\"client_secret\":\"" + secret + "\"}}", "UTF-8"},
		{"oversized", `{"installed":{"client_id":"a","client_secret":"` + secret + `","x":"` + strings.Repeat("a", MaxClientJSONSize) + `"}}`, "larger than"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseGoogleClientJSON([]byte(tt.in))
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got.ClientID != "123-abc.apps.googleusercontent.com" || got.ClientSecret != secret {
					t.Fatalf("wrong client parsed")
				}
				return
			}
			if err == nil || !errors.Is(err, ErrClientJSON) || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("error leaks the secret: %v", err)
			}
		})
	}
}

func TestGoogleClientRedactsSecret(t *testing.T) {
	c := GoogleClient{ClientID: "id", ClientSecret: "GOCSPX-supersecretvalue"}
	for _, s := range []string{fmt.Sprint(c), fmt.Sprintf("%v", c), fmt.Sprintf("%+v", c), fmt.Sprintf("%#v", c), c.String()} {
		if strings.Contains(s, "supersecret") {
			t.Fatalf("formatted client leaks the secret: %s", s)
		}
	}
}
