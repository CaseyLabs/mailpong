package submission

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caseylabs/mailven/internal/oauth2"
	"github.com/caseylabs/mailven/internal/smtp"
)

type fakeTokens struct {
	refreshes  int
	refreshErr error
}

func (f *fakeTokens) Token(context.Context) (string, error) { return "old", nil }
func (f *fakeTokens) Refresh(context.Context, string) (string, error) {
	f.refreshes++
	return "new", f.refreshErr
}

func post(t *testing.T, srv *httptest.Server, tokens TokenSource, before func(smtp.SubmissionResult) error) (smtp.SubmissionResult, error) {
	t.Helper()
	return Post(context.Background(), NewHTTPClient(), tokens, before, func(ctx context.Context, token string) (*http.Request, error) {
		r, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL, strings.NewReader("body"))
		if r != nil {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		return r, err
	})
}

func TestPostOutcomes(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		headers map[string]string
		want    smtp.Outcome
		failure oauth2.MailAPIFailure
		retry   time.Duration
	}{
		{"accepted 200", 200, nil, smtp.OutcomeAccepted, "", 0},
		{"accepted 202", 202, nil, smtp.OutcomeAccepted, "", 0},
		{"policy 403", 403, nil, smtp.OutcomeRejectedPermanent, oauth2.MailAPIFailurePolicy, 0},
		{"resource 404", 404, nil, smtp.OutcomeRejectedPermanent, oauth2.MailAPIFailureResource, 0},
		{"other 400", 400, nil, smtp.OutcomeRejectedPermanent, oauth2.MailAPIFailureOther, 0},
		{"too large 413", 413, nil, smtp.OutcomeRejectedPermanent, oauth2.MailAPIFailureOther, 0},
		{"throttled 429", 429, map[string]string{"Retry-After": "7"}, smtp.OutcomeUnknown, "", 7 * time.Second},
		{"server 500", 500, nil, smtp.OutcomeUnknown, "", 0},
		{"unavailable 503", 503, map[string]string{"Retry-After": "3"}, smtp.OutcomeUnknown, "", 3 * time.Second},
		{"redirect not followed", 307, map[string]string{"Location": "/elsewhere"}, smtp.OutcomeUnknown, "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var hits atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				for k, v := range tt.headers {
					w.Header().Set(k, v)
				}
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()
			var checkpoints []smtp.Outcome
			result, err := post(t, srv, &fakeTokens{}, func(r smtp.SubmissionResult) error {
				checkpoints = append(checkpoints, r.Outcome)
				return nil
			})
			if result.Outcome != tt.want {
				t.Fatalf("outcome = %s, want %s (err %v)", result.Outcome, tt.want, err)
			}
			if len(checkpoints) != 1 || checkpoints[0] != smtp.OutcomeUnknown {
				t.Fatalf("checkpoints = %v, want one unknown before the request", checkpoints)
			}
			if hits.Load() != 1 {
				t.Fatalf("server hit %d times, want 1 (no replay)", hits.Load())
			}
			var apiErr *APIError
			if tt.want == smtp.OutcomeAccepted {
				if err != nil {
					t.Fatalf("unexpected error %v", err)
				}
				return
			}
			if !errors.As(err, &apiErr) || apiErr.Failure != tt.failure || apiErr.RetryAfter != tt.retry {
				t.Fatalf("err = %#v, want failure %q retry %v", err, tt.failure, tt.retry)
			}
		})
	}
}

func TestPostRefreshesOnceOn401(t *testing.T) {
	var auth []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = append(auth, r.Header.Get("Authorization"))
		if r.Header.Get("Authorization") == "Bearer old" {
			w.WriteHeader(401)
			return
		}
		w.WriteHeader(202)
	}))
	defer srv.Close()
	tokens := &fakeTokens{}
	result, err := post(t, srv, tokens, func(smtp.SubmissionResult) error { return nil })
	if err != nil || result.Outcome != smtp.OutcomeAccepted || tokens.refreshes != 1 || len(auth) != 2 {
		t.Fatalf("result %v err %v refreshes %d auth %v", result, err, tokens.refreshes, auth)
	}
}

func TestPostSecond401IsRejectedWithoutThirdRequest(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); w.WriteHeader(401) }))
	defer srv.Close()
	tokens := &fakeTokens{}
	result, err := post(t, srv, tokens, func(smtp.SubmissionResult) error { return nil })
	var apiErr *APIError
	if !errors.As(err, &apiErr) || result.Outcome != smtp.OutcomeRejectedPermanent || apiErr.Failure != oauth2.MailAPIFailureReauthorize || hits.Load() != 2 || tokens.refreshes != 1 {
		t.Fatalf("result %v err %v hits %d refreshes %d", result, err, hits.Load(), tokens.refreshes)
	}
}

func TestPostRefreshInvalidGrantIsRejectedAndReauthorize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) }))
	defer srv.Close()
	tokens := &fakeTokens{refreshErr: oauth2.NewProviderError("invalid_grant", "revoked")}
	result, err := post(t, srv, tokens, func(smtp.SubmissionResult) error { return nil })
	var apiErr *APIError
	if !errors.As(err, &apiErr) || result.Outcome != smtp.OutcomeRejectedPermanent || apiErr.Failure != oauth2.MailAPIFailureReauthorize {
		t.Fatalf("result %v err %v", result, err)
	}
}

func TestPostNoSecondSendAfterUnknown(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		hj, _ := w.(http.Hijacker)
		conn, _, _ := hj.Hijack()
		_ = conn.Close() // body was sent, response lost
	}))
	defer srv.Close()
	tokens := &fakeTokens{}
	result, err := post(t, srv, tokens, func(smtp.SubmissionResult) error { return nil })
	if err == nil || result.Outcome != smtp.OutcomeUnknown || hits.Load() != 1 || tokens.refreshes != 0 {
		t.Fatalf("result %v err %v hits %d", result, err, hits.Load())
	}
}

func TestPostTimeoutAfterWriteIsUnknown(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		<-release
	}))
	defer srv.Close()
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	result, err := Post(ctx, NewHTTPClient(), &fakeTokens{}, func(smtp.SubmissionResult) error { return nil },
		func(ctx context.Context, token string) (*http.Request, error) {
			return http.NewRequestWithContext(ctx, http.MethodPost, srv.URL, strings.NewReader("x"))
		})
	if err == nil || result.Outcome != smtp.OutcomeUnknown {
		t.Fatalf("result %v err %v", result, err)
	}
}

func TestPostConnectFailureIsNotSubmitted(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	var last smtp.Outcome
	result, err := Post(context.Background(), NewHTTPClient(), &fakeTokens{}, func(r smtp.SubmissionResult) error { last = r.Outcome; return nil },
		func(ctx context.Context, token string) (*http.Request, error) {
			return http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader("x"))
		})
	if err == nil || result.Outcome != smtp.OutcomeNotSubmitted || last != smtp.OutcomeUnknown {
		t.Fatalf("result %v err %v", result, err)
	}
}

func TestPostCheckpointFailureSendsNothing(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer srv.Close()
	result, err := post(t, srv, &fakeTokens{}, func(smtp.SubmissionResult) error { return errors.New("disk full") })
	if err == nil || result.Outcome != smtp.OutcomeNotSubmitted || hits.Load() != 0 {
		t.Fatalf("result %v err %v hits %d", result, err, hits.Load())
	}
}

func TestPostCancelledBeforeSendIsNotSubmitted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("request sent") }))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := Post(ctx, NewHTTPClient(), &fakeTokens{}, func(smtp.SubmissionResult) error { return nil },
		func(ctx context.Context, token string) (*http.Request, error) {
			return http.NewRequestWithContext(ctx, http.MethodPost, srv.URL, strings.NewReader("x"))
		})
	if err == nil || result.Outcome != smtp.OutcomeNotSubmitted {
		t.Fatalf("result %v err %v", result, err)
	}
}

func TestHiddenBcc(t *testing.T) {
	raw := []byte("From: a@example.com\r\nTo: \"Zoë\" <zoe@exämple.org>, b@example.com\r\nCc: c@example.com\r\nSubject: =?utf-8?q?h=C3=A9?=\r\n\r\nbody\r\n")
	hidden, err := HiddenBcc(raw, []string{"zoe@xn--exmple-cua.org", "b@example.com", "C@Example.com", "secret@example.net"})
	if err != nil || len(hidden) != 1 || hidden[0] != "secret@example.net" {
		t.Fatalf("hidden = %v err %v", hidden, err)
	}
	if got := WithBcc(raw, nil); &got[0] != &raw[0] {
		t.Fatal("no hidden recipients must pass the bytes through unchanged")
	}
	out := string(WithBcc(raw, hidden))
	if !strings.HasPrefix(out, "Bcc: secret@example.net\r\n") || strings.Contains(strings.SplitN(out, "\r\n", 2)[0], "b@example.com") {
		t.Fatalf("bcc header = %q", strings.SplitN(out, "\r\n", 2)[0])
	}
}
