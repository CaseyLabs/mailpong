package gmailapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/credentials"
	"github.com/caseylabs/mailven/internal/smtp"
	"github.com/caseylabs/mailven/internal/submission"
)

// sendURL is a variable so tests can point it at an httptest server.
var sendURL = "https://gmail.googleapis.com/gmail/v1/users/me/messages/send"

// MaxRawBytes is a local 25 MiB ceiling on req.Raw, measured before the added
// Bcc header and base64url encoding. The messages.send reference gives no
// API-specific limit, so a larger message is rejected locally, not truncated.
const MaxRawBytes = 25 << 20

// Preflight rejects, without fallback, a send the stored route and evidence do
// not support. The send-as cache must show From as verified from a lookup that
// has not since failed.
func Preflight(route *account.MailAPIRoute, tokens *credentials.OAuthTokens, cache Cached, req submission.Request) error {
	if err := submission.CheckRoute(route, account.MailRouteGmailAPI, tokens, false); err != nil {
		return err
	}
	if cache.LookupFailed || cache.StatusFor(req.From) != StatusVerified {
		return fmt.Errorf("%s is not a verified Gmail send-as address", req.From)
	}
	return submission.CheckSize(req.Raw, MaxRawBytes, "Gmail")
}

// Sender submits with users.messages.send.
type Sender struct {
	Tokens submission.TokenSource
	HTTP   *http.Client // nil uses submission.NewHTTPClient
}

var _ submission.Sender = (*Sender)(nil)

// Send posts the raw MIME, base64url encoded. Gmail takes recipients from the
// To, Cc and Bcc headers; hidden envelope recipients are added as a Bcc header.
// Delivered-copy handling of that header is unverified (MA1); active routes
// send through here.
func (s *Sender) Send(ctx context.Context, req submission.Request, before func(smtp.SubmissionResult) error) (smtp.SubmissionResult, error) {
	hidden, err := submission.HiddenBcc(req.Raw, req.Recipients)
	if err != nil {
		result := smtp.SubmissionResult{Outcome: smtp.OutcomeNotSubmitted, Stage: "preflight"}
		return result, &submission.APIError{Result: result, Err: err}
	}
	payload, err := json.Marshal(map[string]string{"raw": base64.RawURLEncoding.EncodeToString(submission.WithBcc(req.Raw, hidden))})
	if err != nil {
		result := smtp.SubmissionResult{Outcome: smtp.OutcomeNotSubmitted, Stage: "preflight"}
		return result, &submission.APIError{Result: result, Err: err}
	}
	hc := s.HTTP
	if hc == nil {
		hc = submission.NewHTTPClient()
	}
	return submission.Post(ctx, hc, s.Tokens, before, func(ctx context.Context, token string) (*http.Request, error) {
		r, err := http.NewRequestWithContext(ctx, http.MethodPost, sendURL, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("Content-Type", "application/json")
		return r, nil
	})
}
