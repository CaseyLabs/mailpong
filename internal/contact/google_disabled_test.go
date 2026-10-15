package contact

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caseylabs/mailven/internal/kit/googleerr"
)

const peopleDisabledBody = `{"error":{"code":403,"status":"PERMISSION_DENIED","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"SERVICE_DISABLED","metadata":{"service":"people.googleapis.com","consumer":"projects/555"}}]}}`

const peopleScopeBody = `{"error":{"code":403,"details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"ACCESS_TOKEN_SCOPE_INSUFFICIENT"}]}}`

type redirectTransport struct{ target string }

func (r redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme, req.URL.Host = "http", strings.TrimPrefix(r.target, "http://")
	return http.DefaultTransport.RoundTrip(req)
}

func forbiddenServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestGoogleSearchDisabledAPI(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		wantDisabled bool
	}{
		{"people api disabled", peopleDisabledBody, true},
		// An ungranted scope keeps returning empty results, as before.
		{"missing scope", peopleScopeBody, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := forbiddenServer(t, tt.body)
			c := NewGoogleContactsClient()
			c.httpClient = &http.Client{Transport: redirectTransport{srv.URL}}
			got, err := c.Search("tok", "ann", 5)
			var disabled *googleerr.DisabledError
			if errors.As(err, &disabled) != tt.wantDisabled {
				t.Fatalf("err = %v, disabled = %v", err, tt.wantDisabled)
			}
			if tt.wantDisabled {
				if disabled.Project != "555" || !strings.Contains(err.Error(), "Google People API is disabled in Google Cloud project 555") {
					t.Errorf("err = %v", err)
				}
			} else if err != nil || len(got) != 0 {
				t.Errorf("got %v, %v; want empty results", got, err)
			}
		})
	}
}

func TestGoogleSyncDisabledAPI(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		wantDisabled bool
	}{
		{"people api disabled", peopleDisabledBody, true},
		{"missing scope", peopleScopeBody, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := forbiddenServer(t, tt.body)
			old := googleConnectionsURL
			googleConnectionsURL = srv.URL
			t.Cleanup(func() { googleConnectionsURL = old })
			_, err := NewGoogleContactsSyncer().SyncContactsDelta("tok", "")
			var disabled *googleerr.DisabledError
			if errors.As(err, &disabled) != tt.wantDisabled {
				t.Fatalf("err = %v, disabled = %v", err, tt.wantDisabled)
			}
			if !tt.wantDisabled && (err == nil || !strings.Contains(err.Error(), "access denied")) {
				t.Errorf("other 403 lost its handling: %v", err)
			}
		})
	}
}
