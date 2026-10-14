// Package apiread is the GET transport shared by the Gmail and Graph read
// clients: the 401-once token refresh, the response size cap and the failure
// classification the sync driver acts on.
package apiread

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/caseylabs/mailven/internal/oauth2"
	"github.com/caseylabs/mailven/internal/submission"
)

// Error is a failed read request. It carries the classification the sync
// driver needs: rate limits pause sync, policy and reauthorization failures
// are reported, and none of them clears credentials.
type Error struct {
	Status      int
	RetryAfter  time.Duration
	RateLimited bool
	Failure     oauth2.MailAPIFailure
	Err         error
}

func (e *Error) Error() string { return e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

// Client is one provider's GET policy. The zero value of an optional field
// selects the common behavior.
type Client struct {
	Tokens submission.TokenSource
	HTTP   *http.Client // nil uses submission.NewHTTPClient
	// Label prefixes errors, for example "gmail read".
	Label string
	// Header adds provider request headers to every attempt.
	Header http.Header
	// ForbiddenRateLimited reports whether a 403 body is a quota answer
	// rather than a policy denial (nil: never).
	ForbiddenRateLimited func(body []byte) bool
	// TooLarge, when set, is wrapped into the error for a response over the cap.
	TooLarge error
	// Detail names the request in transport and cap errors ("" omits it).
	Detail string
}

// Get performs one GET with the 401-once refresh policy, capping the body at
// maxBytes, and classifies failures as *Error.
func (c *Client) Get(ctx context.Context, target string, maxBytes int64) ([]byte, error) {
	hc := c.HTTP
	if hc == nil {
		hc = submission.NewHTTPClient()
	}
	token, err := c.Tokens.Token(ctx)
	if err != nil {
		return nil, &Error{Failure: oauth2.ClassifyMailAPIError(err), Err: err}
	}
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		for k, v := range c.Header {
			req.Header[k] = v
		}
		resp, err := hc.Do(req)
		if err != nil {
			return nil, c.wrap(err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
		_ = resp.Body.Close()
		status := resp.StatusCode
		if status >= 200 && status < 300 {
			if readErr != nil {
				return nil, c.wrap(readErr)
			}
			if int64(len(body)) > maxBytes {
				err := fmt.Errorf("%s: response exceeds %d bytes", c.prefix(), maxBytes)
				if c.TooLarge != nil {
					err = fmt.Errorf("%w: %w", err, c.TooLarge)
				}
				return nil, err
			}
			return body, nil
		}
		if status == http.StatusUnauthorized && attempt == 0 {
			if token, err = c.Tokens.Refresh(ctx, token); err != nil {
				return nil, &Error{Status: status, Failure: oauth2.ClassifyMailAPIError(err), Err: err}
			}
			continue
		}
		return nil, c.classify(status, resp.Header, body)
	}
}

func (c *Client) prefix() string {
	if c.Detail != "" {
		return c.Label + " " + c.Detail
	}
	return c.Label
}

func (c *Client) wrap(err error) error { return fmt.Errorf("%s: %w", c.prefix(), err) }

func (c *Client) classify(status int, h http.Header, body []byte) error {
	detail := strings.TrimSpace(string(body[:min(len(body), 200)]))
	e := &Error{Status: status, RetryAfter: ParseRetryAfter(h),
		Err: fmt.Errorf("%s: HTTP %d: %s", c.Label, status, detail)}
	switch {
	case status == http.StatusTooManyRequests,
		status == http.StatusForbidden && c.ForbiddenRateLimited != nil && c.ForbiddenRateLimited(body):
		e.RateLimited = true
	case status == http.StatusForbidden:
		e.Failure = oauth2.ClassifyMailAPIError(oauth2.NewProviderError("access_denied", detail))
	case status == http.StatusUnauthorized:
		e.Failure = oauth2.ClassifyMailAPIError(oauth2.NewProviderError("invalid_grant", detail))
	case status >= 500:
		e.RateLimited = true // transient: pause like a throttle, never replay in a loop
	}
	return e
}

// ParseRetryAfter reads a Retry-After header as delta-seconds or an HTTP date.
func ParseRetryAfter(h http.Header) time.Duration {
	v := h.Get("Retry-After")
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		return max(time.Until(t), 0)
	}
	return 0
}
