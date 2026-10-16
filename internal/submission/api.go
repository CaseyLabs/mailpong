package submission

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/credentials"
	"github.com/caseylabs/mailven/internal/oauth2"
	"github.com/caseylabs/mailven/internal/smtp"
)

// Request is an already-built final message. The client never alters its
// content, other than adding a Bcc header for hidden recipients (see HiddenBcc).
type Request struct {
	From       string
	Recipients []string // envelope list: To, Cc and Bcc
	Raw        []byte
}

// Sender submits a Request through a provider API. before is called with an
// unknown outcome immediately before each HTTP request is sent; if it fails,
// nothing is sent. The returned result is the final state; callers commit it
// before any local cleanup. A nil error never means delivery.
type Sender interface {
	Send(ctx context.Context, req Request, before func(smtp.SubmissionResult) error) (smtp.SubmissionResult, error)
}

// APIError carries the classified result alongside the failure.
type APIError struct {
	Result     smtp.SubmissionResult
	Status     int                   // HTTP status, 0 when none was received
	RetryAfter time.Duration         // throttling hint only; never retried automatically
	Failure    oauth2.MailAPIFailure // actionable category for a definitive rejection
	Err        error
}

func (e *APIError) Error() string { return e.Err.Error() }
func (e *APIError) Unwrap() error { return e.Err }

// TokenSource supplies the API access token. Refresh forces a refresh through
// the locked snapshot refresh and is used at most once per submission. The
// rejected token belongs to that request; sources may be shared by fetch workers.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
	Refresh(ctx context.Context, rejectedToken string) (string, error)
}

// CheckRoute is the provider-neutral preflight: the route must be staged or
// active for the expected provider and its recorded grant must include send.
// shared says the From is another mailbox. It claims no mailbox right.
func CheckRoute(route *account.MailAPIRoute, want string, tokens *credentials.OAuthTokens, shared bool) error {
	if route == nil || route.Route != want {
		return fmt.Errorf("account has no %s route", want)
	}
	if route.State != account.MailRouteStateStaged && route.State != "active" {
		return fmt.Errorf("%s route is %q, not staged or active", want, route.State)
	}
	if shared && !route.Shared {
		return fmt.Errorf("sending as another mailbox needs shared-mailbox authorization")
	}
	return tokens.RequireMailAPICapability("send", shared)
}

// CheckSize rejects without truncating.
func CheckSize(raw []byte, limit int, provider string) error {
	if len(raw) > limit {
		return fmt.Errorf("message is %d bytes, over the %s API limit of %d", len(raw), provider, limit)
	}
	return nil
}

// HiddenBcc returns the envelope recipients that appear in none of the To, Cc
// or Bcc headers of raw. Both provider APIs read recipients from the message
// headers, so these must be supplied as a Bcc header.
func HiddenBcc(raw []byte, envelope []string) ([]string, error) {
	end := bytes.Index(raw, []byte("\r\n\r\n"))
	if end < 0 {
		end = len(raw)
	}
	msg, err := mail.ReadMessage(bytes.NewReader(append(append([]byte{}, raw[:end]...), "\r\n\r\n"...)))
	if err != nil {
		return nil, fmt.Errorf("parse message headers: %w", err)
	}
	seen := map[string]bool{}
	for _, h := range []string{"To", "Cc", "Bcc"} {
		list, err := msg.Header.AddressList(h)
		if err != nil && !errors.Is(err, mail.ErrHeaderNotPresent) {
			return nil, fmt.Errorf("parse %s header: %w", h, err)
		}
		for _, a := range list {
			seen[normalize(a.Address)] = true
		}
	}
	var hidden []string
	for _, r := range envelope {
		if !seen[normalize(r)] {
			hidden = append(hidden, r)
		}
	}
	return hidden, nil
}

func normalize(addr string) string {
	if n, err := smtp.NormalizeEnvelopeAddress(addr); err == nil {
		addr = n
	}
	return strings.ToLower(addr)
}

// bccFoldWidth keeps each physical line of the added Bcc header well under the
// 998-byte RFC 5322 ceiling.
const bccFoldWidth = 900

// WithBcc prepends a Bcc header naming only the hidden recipients. Visible
// recipients are not repeated, and nothing is added when there are none, so the
// message stays byte-identical in the common case. A long list is folded at
// address boundaries (RFC 5322 §2.2.3) so no physical line grows past the
// limit and no recipient is dropped; the added header is outside any signature.
func WithBcc(raw []byte, hidden []string) []byte {
	if len(hidden) == 0 {
		return raw
	}
	var b strings.Builder
	line := len("Bcc:")
	b.WriteString("Bcc:")
	for i, addr := range hidden {
		sep := " "
		if i > 0 {
			sep = ", "
			if line+len(sep)+len(addr) > bccFoldWidth {
				b.WriteString(",\r\n")
				sep, line = " ", 0
			}
		}
		b.WriteString(sep + addr)
		line += len(sep) + len(addr)
	}
	b.WriteString("\r\n")
	return append([]byte(b.String()), raw...)
}

// Post sends one request built by build and maps the HTTP outcome honestly:
// 2xx accepted; 401 gets one refresh and one re-attempt; other 4xx is a
// definitive rejection; 429, 5xx, redirects and any failure after the request
// was written are unknown and never replayed; failure before a byte is written
// is not-submitted.
func Post(ctx context.Context, hc *http.Client, tokens TokenSource, before func(smtp.SubmissionResult) error,
	build func(ctx context.Context, token string) (*http.Request, error)) (smtp.SubmissionResult, error) {
	result := smtp.SubmissionResult{Outcome: smtp.OutcomeNotSubmitted, Stage: "auth"}
	token, err := tokens.Token(ctx)
	if err != nil {
		return result, &APIError{Result: result, Failure: oauth2.ClassifyMailAPIError(err), Err: err}
	}
	for attempt := 0; ; attempt++ {
		result = smtp.SubmissionResult{Outcome: smtp.OutcomeNotSubmitted, Stage: "request"}
		req, err := build(ctx, token)
		if err != nil {
			return result, &APIError{Result: result, Err: err}
		}
		wrote := false
		req = req.WithContext(httptrace.WithClientTrace(req.Context(), &httptrace.ClientTrace{
			WroteHeaders: func() { wrote = true },
		}))
		unknown := smtp.SubmissionResult{Outcome: smtp.OutcomeUnknown, Stage: "api-send"}
		if err := before(unknown); err != nil {
			result.Stage = "journal"
			return result, &APIError{Result: result, Err: fmt.Errorf("record submission checkpoint: %w", err)}
		}
		resp, err := hc.Do(req)
		if err != nil {
			if !wrote {
				return result, &APIError{Result: result, Err: fmt.Errorf("send request: %w", err)}
			}
			return unknown, &APIError{Result: unknown, Err: fmt.Errorf("send request: %w", err)}
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		_ = resp.Body.Close()
		detail := strings.TrimSpace(string(body))
		status := resp.StatusCode
		switch {
		case status >= 200 && status < 300:
			return smtp.SubmissionResult{Outcome: smtp.OutcomeAccepted, Stage: "api-send"}, nil
		case status == http.StatusUnauthorized && attempt == 0:
			// Rejected before any send; refresh once and try again.
			if token, err = tokens.Refresh(ctx, token); err != nil {
				rejected := smtp.SubmissionResult{Outcome: smtp.OutcomeRejectedPermanent, Stage: "auth"}
				return rejected, &APIError{Result: rejected, Status: status, Failure: oauth2.ClassifyMailAPIError(err), Err: err}
			}
			continue
		case status == http.StatusTooManyRequests || status >= 500 || status < 400:
			return unknown, &APIError{Result: unknown, Status: status, RetryAfter: retryAfter(resp.Header),
				Err: fmt.Errorf("API returned HTTP %d: %s", status, detail)}
		}
		outcome := smtp.OutcomeRejectedPermanent
		if status == http.StatusForbidden && strings.Contains(strings.ToLower(detail), "ratelimit") {
			outcome = smtp.OutcomeRejectedTemporary
		}
		rejected := smtp.SubmissionResult{Outcome: outcome, Stage: "api-send"}
		return rejected, &APIError{Result: rejected, Status: status, RetryAfter: retryAfter(resp.Header),
			Failure: oauth2.ClassifyMailAPIError(oauth2.NewProviderError(statusCode(status), detail)),
			Err:     fmt.Errorf("API rejected the message with HTTP %d: %s", status, detail)}
	}
}

func statusCode(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "invalid_grant"
	case http.StatusForbidden:
		return "access_denied"
	case http.StatusNotFound:
		return "invalid_scope"
	}
	return strconv.Itoa(status)
}

func retryAfter(h http.Header) time.Duration {
	v := h.Get("Retry-After")
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		return max(time.Until(t), 0)
	}
	return 0
}

// NewHTTPClient never follows redirects, so a POST cannot be replayed.
func NewHTTPClient() *http.Client {
	return &http.Client{Timeout: 2 * time.Minute,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
