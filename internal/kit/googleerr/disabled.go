// Package googleerr classifies Google API error responses that the caller can
// act on. It is generic: any Google REST client (Gmail, People, Calendar) can
// use it without extension-specific behavior.
package googleerr

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// DisabledError means Google refused a request with 403 SERVICE_DISABLED: the
// API is not enabled in the Cloud project that owns the OAuth client.
// Re-authorizing cannot fix it; enabling the API can.
type DisabledError struct {
	// Service is the API's service name, such as "gmail.googleapis.com".
	Service string
	// Project is the consumer project number or ID.
	Project string
}

var (
	serviceRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*\.googleapis\.com$`)
	projectRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
)

// serviceNames maps service names to the names users see in the console.
var serviceNames = map[string]string{
	"gmail.googleapis.com":         "Gmail API",
	"people.googleapis.com":        "Google People API",
	"calendar-json.googleapis.com": "Google Calendar API",
}

// Name is the user-facing API name, falling back to the service name.
func (e *DisabledError) Name() string {
	if name, ok := serviceNames[e.Service]; ok {
		return name
	}
	return e.Service
}

// EnableURL is the console page that enables the API. Mailven builds it from
// the validated service and project; it never copies a link from a response.
func (e *DisabledError) EnableURL() string {
	return "https://console.developers.google.com/apis/api/" + e.Service +
		"/overview?project=" + url.QueryEscape(e.Project)
}

// UserMessage is the sentence shown to the user.
func (e *DisabledError) UserMessage() string {
	return fmt.Sprintf("%s is disabled in Google Cloud project %s. Enable it at %s, wait a minute, then try again.",
		e.Name(), e.Project, e.EnableURL())
}

func (e *DisabledError) Error() string { return e.UserMessage() }

// ParseDisabled returns the typed error when status and body are a Google
// 403 SERVICE_DISABLED response whose service and consumer are well formed.
// It returns nil for every other response, including other 403s.
func ParseDisabled(status int, body []byte) *DisabledError {
	if status != http.StatusForbidden {
		return nil
	}
	var resp struct {
		Error struct {
			Details []struct {
				Type     string            `json:"@type"`
				Reason   string            `json:"reason"`
				Metadata map[string]string `json:"metadata"`
			} `json:"details"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &resp) != nil {
		return nil
	}
	for _, d := range resp.Error.Details {
		if d.Reason != "SERVICE_DISABLED" || !strings.HasSuffix(d.Type, "google.rpc.ErrorInfo") {
			continue
		}
		service := d.Metadata["service"]
		project := strings.TrimPrefix(d.Metadata["consumer"], "projects/")
		if !serviceRE.MatchString(service) || !projectRE.MatchString(project) {
			return nil
		}
		return &DisabledError{Service: service, Project: project}
	}
	return nil
}

// Snippet returns a single-line, length-bounded excerpt of a response body for
// logs and error text.
func Snippet(body []byte, limit int) string {
	s := strings.Join(strings.Fields(string(body)), " ")
	if len(s) > limit {
		s = strings.ToValidUTF8(s[:limit], "") + "…"
	}
	return s
}
