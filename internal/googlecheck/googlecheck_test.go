package googlecheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const disabledBody = `{"error":{"code":403,"status":"PERMISSION_DENIED","message":"disabled","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"SERVICE_DISABLED","domain":"googleapis.com","metadata":{"service":"%s","consumer":"projects/1234"}}]}}`

func serve(t *testing.T, h http.HandlerFunc) (*httptest.Server, *[]string) {
	t.Helper()
	var mu sync.Mutex
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.Path)
		mu.Unlock()
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func TestProbes(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		state  string
	}{
		{"ok", 200, `{}`, StateOK},
		{"disabled", 403, strings.Replace(disabledBody, "%s", "gmail.googleapis.com", 1), StateDisabled},
		{"missing scope", 403, `{"error":{"errors":[{"reason":"insufficientPermissions"}]}}`, StateMissingScope},
		{"rate limited is an error", 403, `{"error":{"errors":[{"reason":"rateLimitExceeded"}]}}`, StateError},
		{"unauthorized", 401, `{"error":{"message":"Invalid Credentials"}}`, StateError},
		{"server error", 500, `oops`, StateError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, seen := serve(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(c.status)
				_, _ = w.Write([]byte(c.body))
			})
			oldP, oldS, oldPe, oldC := gmailProfileURL, gmailSendAsURL, peopleMeURL, calendarListURL
			gmailProfileURL, gmailSendAsURL, peopleMeURL, calendarListURL = srv.URL+"/profile", srv.URL+"/sendAs", srv.URL+"/people", srv.URL+"/cal"
			t.Cleanup(func() { gmailProfileURL, gmailSendAsURL, peopleMeURL, calendarListURL = oldP, oldS, oldPe, oldC })

			for api, run := range map[string]func(context.Context, *http.Client) Result{APIGmail: Gmail, APIPeople: People, APICalendar: Calendar} {
				r := run(context.Background(), srv.Client())
				if r.API != api || r.State != c.state {
					t.Fatalf("%s: got %+v, want state %s", api, r, c.state)
				}
				if c.state == StateDisabled && (r.Project != "1234" || !strings.Contains(r.EnableURL, "gmail.googleapis.com")) {
					t.Fatalf("disabled result lacks project or link: %+v", r)
				}
				if c.state != StateDisabled && r.EnableURL != "" {
					t.Fatalf("non-disabled result carries a link: %+v", r)
				}
			}
			for _, s := range *seen {
				if !strings.HasPrefix(s, "GET ") {
					t.Fatalf("probe made a non-read request: %s", s)
				}
			}
		})
	}
}

func TestGmailStopsAtFirstFailure(t *testing.T) {
	srv, seen := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/profile" {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(200)
	})
	gmailProfileURL, gmailSendAsURL = srv.URL+"/profile", srv.URL+"/sendAs"
	r := Gmail(context.Background(), srv.Client())
	if r.State != StateError || len(*seen) != 1 {
		t.Fatalf("got %+v after %v", r, *seen)
	}
}

func TestGmailChecksSendAsAfterProfile(t *testing.T) {
	srv, _ := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sendAs" {
			w.WriteHeader(403)
			_, _ = w.Write([]byte(strings.Replace(disabledBody, "%s", "gmail.googleapis.com", 1)))
			return
		}
		w.WriteHeader(200)
	})
	gmailProfileURL, gmailSendAsURL = srv.URL+"/profile", srv.URL+"/sendAs"
	if r := Gmail(context.Background(), srv.Client()); r.State != StateDisabled {
		t.Fatalf("got %+v", r)
	}
}
