// Package apiwrite is the shared transport of the Gmail and Graph write
// clients: one request with the 401-once refresh policy, bounded rate-limit
// retries and an honest outcome, never a redirect and never a replay of a
// request that may have been applied.
package apiwrite

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"strconv"
	"strings"
	"time"

	"github.com/caseylabs/mailven/internal/oauth2"
	"github.com/caseylabs/mailven/internal/submission"
)

// Documented behaviour recorded as constants, never probed (Gmail usage
// limits and Graph throttling guidance both direct clients to honor
// Retry-After on 429).
const (
	// MaxRetries is how many times a rate-limited request is re-sent. A 429
	// is answered before the operation runs, so re-sending cannot duplicate it.
	MaxRetries = 3
	// MaxRetryAfter caps one wait, whatever Retry-After says.
	MaxRetryAfter = 60 * time.Second
	// defaultBackoff is the first wait when Retry-After is absent; it doubles.
	defaultBackoff = time.Second
	maxResponse    = 1 << 20
)

// Outcome says what is known about a write.
type Outcome string

const (
	// Applied: a 2xx answer.
	Applied Outcome = "applied"
	// Rejected: a definitive 4xx; the provider did not apply it (404 included).
	Rejected Outcome = "rejected"
	// Unknown: the request may have been applied (transport failure after the
	// request was written, 5xx, redirect, or rate limit that outlasted the
	// retries). Callers verify remote state before any re-send.
	Unknown Outcome = "unknown"
	// NotSent: nothing reached the provider (no token, capability refused, or
	// failure before a byte was written); safe to retry.
	NotSent Outcome = "not-sent"
	// Reauthorize: the grant is rejected (401 after one refresh) or the
	// provider's policy refuses the access (403); Failure says which.
	Reauthorize Outcome = "reauthorize"
)

// Error is a failed write.
type Error struct {
	Outcome    Outcome
	Status     int
	RetryAfter time.Duration
	Failure    oauth2.MailAPIFailure
	Err        error
}

func (e *Error) Error() string { return e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

// OutcomeOf returns the outcome of err: Applied for nil, and Unknown for an
// error that is not an *Error (the conservative reading).
func OutcomeOf(err error) Outcome {
	if err == nil {
		return Applied
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Outcome
	}
	return Unknown
}

// StatusOf returns the HTTP status carried by err, or 0.
func StatusOf(err error) int {
	var e *Error
	if errors.As(err, &e) {
		return e.Status
	}
	return 0
}

// Request is one write. Body is kept so a rate-limited request can be re-sent.
type Request struct {
	Method string
	URL    string
	Header http.Header
	Body   []byte
}

// sleep is a variable so tests do not wait.
var sleep = func(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// SetSleepForTest replaces the wait and returns a restore function.
func SetSleepForTest(f func(context.Context, time.Duration) error) func() {
	old := sleep
	sleep = f
	return func() { sleep = old }
}

// Do sends r and returns the response body of a 2xx answer (at most 1 MiB).
// hc nil uses submission.NewHTTPClient.
func Do(ctx context.Context, hc *http.Client, tokens submission.TokenSource, r Request) ([]byte, error) {
	if hc == nil {
		hc = submission.NewHTTPClient()
	}
	token, err := tokens.Token(ctx)
	if err != nil {
		return nil, &Error{Outcome: NotSent, Failure: oauth2.ClassifyMailAPIError(err), Err: err}
	}
	refreshed := false
	retries := 0
	backoff := defaultBackoff
	for {
		req, err := http.NewRequestWithContext(ctx, r.Method, r.URL, bytes.NewReader(r.Body))
		if err != nil {
			return nil, &Error{Outcome: NotSent, Err: err}
		}
		for k, v := range r.Header {
			req.Header[k] = v
		}
		req.Header.Set("Authorization", "Bearer "+token)
		wrote := false
		req = req.WithContext(httptrace.WithClientTrace(req.Context(), &httptrace.ClientTrace{
			WroteHeaders: func() { wrote = true },
		}))
		resp, err := hc.Do(req)
		if err != nil {
			outcome := Unknown
			if !wrote {
				outcome = NotSent
			}
			return nil, &Error{Outcome: outcome, Err: fmt.Errorf("%s %s: %w", r.Method, redact(r.URL), err)}
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponse))
		_ = resp.Body.Close()
		status := resp.StatusCode
		detail := strings.TrimSpace(string(body[:min(len(body), 200)]))
		wait := parseRetryAfter(resp.Header)
		fail := func(o Outcome, f oauth2.MailAPIFailure) error {
			return &Error{Outcome: o, Status: status, RetryAfter: wait, Failure: f,
				Err: fmt.Errorf("%s %s: HTTP %d: %s", r.Method, redact(r.URL), status, detail)}
		}
		switch {
		case status >= 200 && status < 300:
			return body, nil
		case status == http.StatusUnauthorized && !refreshed:
			refreshed = true
			if token, err = tokens.Refresh(ctx, token); err != nil {
				return nil, &Error{Outcome: Reauthorize, Status: status, Failure: oauth2.ClassifyMailAPIError(err), Err: err}
			}
			continue
		case status == http.StatusUnauthorized:
			return nil, fail(Reauthorize, oauth2.ClassifyMailAPIError(oauth2.NewProviderError("invalid_grant", detail)))
		case status == http.StatusTooManyRequests, status == http.StatusForbidden && isRateLimit(body):
			if retries >= MaxRetries {
				return nil, fail(Unknown, oauth2.MailAPIFailureNone)
			}
			retries++
			d := wait
			if d == 0 {
				d = backoff
				backoff *= 2
			}
			if err := sleep(ctx, min(d, MaxRetryAfter)); err != nil {
				return nil, &Error{Outcome: Unknown, Status: status, Err: err}
			}
			continue
		case status == http.StatusForbidden:
			return nil, fail(Reauthorize, oauth2.ClassifyMailAPIError(oauth2.NewProviderError("access_denied", detail)))
		case status >= 400 && status < 500:
			return nil, fail(Rejected, oauth2.MailAPIFailureNone)
		}
		// 5xx and redirects (never followed) leave the result unknown.
		return nil, fail(Unknown, oauth2.MailAPIFailureNone)
	}
}

// isRateLimit recognizes Gmail's 403 rate-limit answers.
func isRateLimit(body []byte) bool {
	s := strings.ToLower(string(body))
	return strings.Contains(s, "ratelimitexceeded") || strings.Contains(s, "userratelimitexceeded")
}

func parseRetryAfter(h http.Header) time.Duration {
	v := h.Get("Retry-After")
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		return max(time.Until(t), 0)
	}
	return 0
}

// redact drops the query so errors never carry tokens or paging state.
func redact(u string) string {
	if i := strings.IndexByte(u, '?'); i >= 0 {
		return u[:i]
	}
	return u
}
