package apiwrite

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type tokens struct{ refreshed atomic.Int32 }

func (*tokens) Token(context.Context) (string, error) { return "old", nil }
func (t *tokens) Refresh(context.Context, string) (string, error) {
	t.refreshed.Add(1)
	return "new", nil
}

func noSleep(t *testing.T) *[]time.Duration {
	var waits []time.Duration
	restore := SetSleepForTest(func(_ context.Context, d time.Duration) error { waits = append(waits, d); return nil })
	t.Cleanup(restore)
	return &waits
}

func TestWriteOutcomes(t *testing.T) {
	noSleep(t)
	cases := []struct {
		name   string
		status int
		body   string
		want   Outcome
	}{
		{"ok", 200, `{}`, Applied},
		{"no content", 204, ``, Applied},
		{"gone", 404, `{}`, Rejected},
		{"bad request", 400, `{}`, Rejected},
		{"forbidden policy", 403, `{"error":"denied"}`, Reauthorize},
		{"server error", 500, ``, Unknown},
		{"bad gateway", 502, ``, Unknown},
		{"redirect not followed", 307, ``, Unknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var hits atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits.Add(1)
				w.Header().Set("Location", "http://127.0.0.1:1/elsewhere")
				w.WriteHeader(c.status)
				_, _ = w.Write([]byte(c.body))
			}))
			defer srv.Close()
			_, err := Do(context.Background(), nil, &tokens{}, Request{Method: "POST", URL: srv.URL})
			if got := OutcomeOf(err); got != c.want {
				t.Fatalf("outcome = %s (%v), want %s", got, err, c.want)
			}
			if hits.Load() != 1 {
				t.Fatalf("%d requests, want exactly 1 (no replay)", hits.Load())
			}
		})
	}
}

func TestWriteRefreshesOnce(t *testing.T) {
	var auth []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = append(auth, r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	tk := &tokens{}
	_, err := Do(context.Background(), nil, tk, Request{Method: "POST", URL: srv.URL})
	if OutcomeOf(err) != Reauthorize || tk.refreshed.Load() != 1 || len(auth) != 2 || auth[1] != "Bearer new" {
		t.Fatalf("err=%v refreshed=%d auth=%v", err, tk.refreshed.Load(), auth)
	}
}

func TestWriteRateLimitRetriesWithCap(t *testing.T) {
	waits := noSleep(t)
	var hits atomic.Int32
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		b := make([]byte, 8)
		n, _ := r.Body.Read(b)
		bodies = append(bodies, string(b[:n]))
		switch hits.Load() {
		case 1:
			w.Header().Set("Retry-After", "7")
		case 2:
			w.Header().Set("Retry-After", "3600")
		}
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	_, err := Do(context.Background(), nil, &tokens{}, Request{Method: "POST", URL: srv.URL, Body: []byte("payload")})
	if OutcomeOf(err) != Unknown || hits.Load() != MaxRetries+1 {
		t.Fatalf("outcome=%s hits=%d err=%v", OutcomeOf(err), hits.Load(), err)
	}
	if len(*waits) != 3 || (*waits)[0] != 7*time.Second || (*waits)[1] != MaxRetryAfter || (*waits)[2] != time.Second {
		t.Fatalf("waits = %v", *waits)
	}
	for _, b := range bodies {
		if b != "payload" {
			t.Fatalf("retried body = %q", b)
		}
	}
}

func TestWriteRateLimitThenSuccess(t *testing.T) {
	noSleep(t)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":{"errors":[{"reason":"rateLimitExceeded"}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	if _, err := Do(context.Background(), nil, &tokens{}, Request{Method: "POST", URL: srv.URL}); err != nil || hits.Load() != 2 {
		t.Fatalf("err=%v hits=%d", err, hits.Load())
	}
}

func TestWriteTransportFailure(t *testing.T) {
	// Connection refused: nothing was written, so the request was not sent.
	_, err := Do(context.Background(), nil, &tokens{}, Request{Method: "POST", URL: "http://127.0.0.1:1/x"})
	if OutcomeOf(err) != NotSent {
		t.Fatalf("outcome = %s, want not-sent", OutcomeOf(err))
	}
	// The server drops the connection after reading the request: unknown.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		c, _, _ := w.(http.Hijacker).Hijack()
		_ = c.Close()
	}))
	defer srv.Close()
	_, err = Do(context.Background(), nil, &tokens{}, Request{Method: "POST", URL: srv.URL})
	if OutcomeOf(err) != Unknown {
		t.Fatalf("outcome = %s, want unknown (%v)", OutcomeOf(err), err)
	}
}

func TestWriteCancellationDuringWaitIsUnknown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	restore := SetSleepForTest(func(ctx context.Context, _ time.Duration) error { cancel(); return ctx.Err() })
	defer restore()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(429) }))
	defer srv.Close()
	_, err := Do(ctx, nil, &tokens{}, Request{Method: "POST", URL: srv.URL})
	if !errors.Is(err, context.Canceled) || OutcomeOf(err) != Unknown {
		t.Fatalf("err=%v outcome=%s", err, OutcomeOf(err))
	}
}
