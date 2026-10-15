// Package googlecheck runs the read-only probes behind "Check Google setup":
// one GET per Google API an account relies on, classified into a state the UI
// can show. It makes no writes and never retries; the caller decides when to run.
package googlecheck

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/caseylabs/mailven/internal/kit/googleerr"
)

// Result states.
const (
	StateOK           = "ok"
	StateDisabled     = "disabled"      // API not enabled in the Cloud project; EnableURL fixes it
	StateMissingScope = "missing-scope" // token lacks permission; re-authorize
	StateNotConnected = "not-connected" // the account has no grant for this API's slot
	StateError        = "error"
)

// API identifiers.
const (
	APIGmail    = "gmail"
	APIPeople   = "people"
	APICalendar = "calendar"
)

// Result is the outcome of probing one API.
type Result struct {
	API       string `json:"api"`
	State     string `json:"state"`
	Detail    string `json:"detail"`    // short user-facing explanation; empty when OK
	EnableURL string `json:"enableUrl"` // set only for StateDisabled
	Project   string `json:"project"`   // set only for StateDisabled
}

// Endpoints are variables so tests can point them at an httptest server.
var (
	gmailProfileURL = "https://gmail.googleapis.com/gmail/v1/users/me/profile"
	gmailSendAsURL  = "https://gmail.googleapis.com/gmail/v1/users/me/settings/sendAs"
	peopleMeURL     = "https://people.googleapis.com/v1/people/me/connections?personFields=names&pageSize=1"
	calendarListURL = "https://www.googleapis.com/calendar/v3/users/me/calendarList?maxResults=1&fields=items(id)"
)

const probeTimeout = 15 * time.Second

// Gmail probes users.getProfile and the send-as list; the first failure is reported.
func Gmail(ctx context.Context, client *http.Client) Result {
	for _, u := range []string{gmailProfileURL, gmailSendAsURL} {
		if r := probe(ctx, client, APIGmail, u); r.State != StateOK {
			return r
		}
	}
	return Result{API: APIGmail, State: StateOK}
}

// People lists one connection (people.connections.list), the call the contacts.readonly scope covers.
func People(ctx context.Context, client *http.Client) Result {
	return probe(ctx, client, APIPeople, peopleMeURL)
}

// Calendar probes the calendar list.
func Calendar(ctx context.Context, client *http.Client) Result {
	return probe(ctx, client, APICalendar, calendarListURL)
}

// NotConnected is the result for an API whose slot has no grant on the account.
func NotConnected(api, detail string) Result {
	return Result{API: api, State: StateNotConnected, Detail: detail}
}

// Failure classifies a probe that could not run (token or client errors).
func Failure(api string, err error) Result {
	return Result{API: api, State: StateError, Detail: googleerr.Snippet([]byte(err.Error()), 200)}
}

// Disabled builds the result for a disabled API.
func Disabled(api string, d *googleerr.DisabledError) Result {
	return Result{API: api, State: StateDisabled, Detail: d.UserMessage(), EnableURL: d.EnableURL(), Project: d.Project}
}

func probe(ctx context.Context, client *http.Client, api, url string) Result {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Failure(api, err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return Failure(api, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Failure(api, err)
	}
	return classify(api, resp.StatusCode, body)
}

func classify(api string, status int, body []byte) Result {
	if d := googleerr.ParseDisabled(status, body); d != nil {
		return Disabled(api, d)
	}
	switch {
	case status >= 200 && status < 300:
		return Result{API: api, State: StateOK}
	case status == http.StatusForbidden && !rateLimited(body):
		return Result{API: api, State: StateMissingScope, Detail: "Google denied access. Re-authorize the account and grant the requested permissions."}
	default:
		return Result{API: api, State: StateError, Detail: fmt.Sprintf("HTTP %d: %s", status, googleerr.Snippet(body, 200))}
	}
}

func rateLimited(body []byte) bool {
	s := string(body)
	for _, r := range []string{"rateLimitExceeded", "userRateLimitExceeded", "dailyLimitExceeded", "quotaExceeded", "RATE_LIMIT_EXCEEDED"} {
		if strings.Contains(s, r) {
			return true
		}
	}
	return false
}
