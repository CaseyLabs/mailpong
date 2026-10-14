package apiread

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type tokens struct{ refreshed atomic.Int32 }

func (t *tokens) Token(context.Context) (string, error) { return "old", nil }
func (t *tokens) Refresh(context.Context, string) (string, error) {
	t.refreshed.Add(1)
	return "new", nil
}

var errBig = errors.New("big")

func TestGet(t *testing.T) {
	tests := []struct {
		name        string
		handler     http.HandlerFunc
		forbidden   func([]byte) bool
		wantErr     string
		wantStatus  int
		wantRate    bool
		wantRetry   time.Duration
		wantRefresh int32
		wantBig     bool
	}{
		{name: "ok", handler: func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("hi")) }},
		{name: "refresh once then ok", wantRefresh: 1, handler: func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer new" {
				w.WriteHeader(401)
				return
			}
			_, _ = w.Write([]byte("hi"))
		}},
		{name: "401 twice fails", wantRefresh: 1, wantStatus: 401, wantErr: "HTTP 401",
			handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(401) }},
		{name: "429 retry-after", wantStatus: 429, wantRate: true, wantRetry: 7 * time.Second, wantErr: "HTTP 429",
			handler: func(w http.ResponseWriter, _ *http.Request) { w.Header().Set("Retry-After", "7"); w.WriteHeader(429) }},
		{name: "503 transient", wantStatus: 503, wantRate: true, wantErr: "HTTP 503",
			handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(503) }},
		{name: "403 policy", wantStatus: 403, wantErr: "HTTP 403",
			handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(403) }},
		{name: "403 quota", wantStatus: 403, wantRate: true, wantErr: "HTTP 403",
			forbidden: func([]byte) bool { return true },
			handler:   func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(403) }},
		{name: "over cap", wantErr: "exceeds 4 bytes", wantBig: true,
			handler: func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("12345")) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()
			tok := &tokens{}
			c := Client{Tokens: tok, HTTP: srv.Client(), Label: "x read", ForbiddenRateLimited: tc.forbidden, TooLarge: errBig}
			_, err := c.Get(context.Background(), srv.URL, 4)
			if tok.refreshed.Load() != tc.wantRefresh {
				t.Errorf("refreshes: got %d, want %d", tok.refreshed.Load(), tc.wantRefresh)
			}
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error: got %v, want containing %q", err, tc.wantErr)
			}
			if errors.Is(err, errBig) != tc.wantBig {
				t.Errorf("TooLarge wrapped: got %v, want %v", errors.Is(err, errBig), tc.wantBig)
			}
			var re *Error
			if !errors.As(err, &re) {
				if tc.wantStatus != 0 {
					t.Fatalf("want *Error, got %T", err)
				}
				return
			}
			if re.Status != tc.wantStatus || re.RateLimited != tc.wantRate || re.RetryAfter != tc.wantRetry {
				t.Errorf("got status=%d rate=%v retry=%v", re.Status, re.RateLimited, re.RetryAfter)
			}
		})
	}
}
