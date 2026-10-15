package googleerr

import (
	"errors"
	"strings"
	"testing"
)

func disabledBody(service, consumer string) []byte {
	return []byte(`{"error":{"code":403,"message":"Gmail API has not been used in project 123 before or it is disabled. Enable it by visiting https://evil.example/enable","status":"PERMISSION_DENIED","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"SERVICE_DISABLED","domain":"googleapis.com","metadata":{"service":"` + service + `","consumer":"` + consumer + `","activationUrl":"https://evil.example/enable"}}]}}`)
}

func TestParseDisabled(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    []byte
		want    bool
		service string
		project string
	}{
		{"gmail", 403, disabledBody("gmail.googleapis.com", "projects/123456"), true, "gmail.googleapis.com", "123456"},
		{"people", 403, disabledBody("people.googleapis.com", "projects/my-proj-1"), true, "people.googleapis.com", "my-proj-1"},
		{"calendar", 403, disabledBody("calendar-json.googleapis.com", "projects/987"), true, "calendar-json.googleapis.com", "987"},
		{"not forbidden", 400, disabledBody("gmail.googleapis.com", "projects/1"), false, "", ""},
		{"missing scope", 403, []byte(`{"error":{"details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"ACCESS_TOKEN_SCOPE_INSUFFICIENT"}]}}`), false, "", ""},
		{"rate limit", 403, []byte(`{"error":{"errors":[{"reason":"userRateLimitExceeded"}]}}`), false, "", ""},
		{"bad service", 403, disabledBody("evil.example/x?", "projects/1"), false, "", ""},
		{"bad project", 403, disabledBody("gmail.googleapis.com", "projects/1&x=2"), false, "", ""},
		{"not json", 403, []byte(`<html>`), false, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseDisabled(tt.status, tt.body)
			if (got != nil) != tt.want {
				t.Fatalf("ParseDisabled = %v, want match %v", got, tt.want)
			}
			if got == nil {
				return
			}
			if got.Service != tt.service || got.Project != tt.project {
				t.Errorf("got %+v", got)
			}
			msg := got.UserMessage()
			if !strings.Contains(msg, "project "+tt.project) || strings.Contains(msg, "evil.example") {
				t.Errorf("message = %q", msg)
			}
			want := "https://console.developers.google.com/apis/api/" + tt.service + "/overview?project=" + tt.project
			if got.EnableURL() != want {
				t.Errorf("EnableURL = %q, want %q", got.EnableURL(), want)
			}
			var target *DisabledError
			if !errors.As(error(got), &target) {
				t.Error("errors.As failed")
			}
		})
	}
}

func TestDisabledNames(t *testing.T) {
	for service, name := range map[string]string{
		"gmail.googleapis.com": "Gmail API", "people.googleapis.com": "Google People API",
		"calendar-json.googleapis.com": "Google Calendar API", "drive.googleapis.com": "drive.googleapis.com",
	} {
		if got := (&DisabledError{Service: service, Project: "1"}).Name(); got != name {
			t.Errorf("%s: %q", service, got)
		}
	}
}

func TestSnippet(t *testing.T) {
	if got := Snippet([]byte("a\n  b\tc"), 10); got != "a b c" {
		t.Errorf("got %q", got)
	}
	if got := Snippet([]byte(strings.Repeat("x", 50)), 10); got != strings.Repeat("x", 10)+"…" {
		t.Errorf("got %q", got)
	}
}
