// Package graphapi is a minimal Microsoft Graph client. It currently only
// submits mail, and no account route uses it yet.
package graphapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/credentials"
	"github.com/caseylabs/mailven/internal/smtp"
	"github.com/caseylabs/mailven/internal/submission"
)

// graphBase is a variable so tests can point it at an httptest server.
var graphBase = "https://graph.microsoft.com/v1.0"

// MaxRawBytes is a conservative local ceiling. The sendMail reference documents
// no size limit for MIME content (unverified); Exchange Online limits vary by
// tenant and administrator, so larger messages are rejected locally rather than
// probed or truncated and smaller ones may still be refused by the service.
const MaxRawBytes = 25 << 20

func sameAddress(a, b string) bool {
	na, errA := smtp.NormalizeEnvelopeAddress(a)
	nb, errB := smtp.NormalizeEnvelopeAddress(b)
	if errA != nil || errB != nil {
		return strings.EqualFold(a, b)
	}
	return strings.EqualFold(na, nb)
}

// Preflight rejects a send the stored route and evidence do not support. A From
// other than the account's own mailbox needs the Shared send permission.
func Preflight(route *account.MailAPIRoute, tokens *credentials.OAuthTokens, ownAddress string, req submission.Request) error {
	if err := submission.CheckRoute(route, account.MailRouteGraphAPI, tokens, !sameAddress(req.From, ownAddress)); err != nil {
		return err
	}
	return submission.CheckSize(req.Raw, MaxRawBytes, "Graph")
}

// Sender submits with the documented MIME form of sendMail.
type Sender struct {
	Tokens submission.TokenSource
	Own    string       // the signed-in mailbox address
	HTTP   *http.Client // nil uses submission.NewHTTPClient
}

var _ submission.Sender = (*Sender)(nil)

// Send posts the base64 MIME as text/plain; 202 means accepted, not delivered.
// Graph's Bcc handling for MIME is undocumented (unverified); hidden envelope
// recipients are added as a Bcc header, and the route stays inert until a live
// check confirms they are not exposed. Sent Items saving is the service default.
func (s *Sender) Send(ctx context.Context, req submission.Request, before func(smtp.SubmissionResult) error) (smtp.SubmissionResult, error) {
	fail := func(err error) (smtp.SubmissionResult, error) {
		result := smtp.SubmissionResult{Outcome: smtp.OutcomeNotSubmitted, Stage: "preflight"}
		return result, &submission.APIError{Result: result, Err: err}
	}
	hidden, err := submission.HiddenBcc(req.Raw, req.Recipients)
	if err != nil {
		return fail(err)
	}
	payload := []byte(base64.StdEncoding.EncodeToString(submission.WithBcc(req.Raw, hidden)))
	target := graphBase + "/me/sendMail"
	if !sameAddress(req.From, s.Own) {
		target = fmt.Sprintf("%s/users/%s/sendMail", graphBase, url.PathEscape(req.From))
	}
	hc := s.HTTP
	if hc == nil {
		hc = submission.NewHTTPClient()
	}
	return submission.Post(ctx, hc, s.Tokens, before, func(ctx context.Context, token string) (*http.Request, error) {
		r, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("Content-Type", "text/plain")
		return r, nil
	})
}
