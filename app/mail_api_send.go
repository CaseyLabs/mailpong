package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/gmailapi"
	"github.com/caseylabs/mailven/internal/graphapi"
	"github.com/caseylabs/mailven/internal/smtp"
	"github.com/caseylabs/mailven/internal/submission"
)

// apiTokenSource adapts the F4b locked snapshot refresh to submission.TokenSource.
// Refresh is forced only for a token the API rejected with 401.
type apiTokenSource struct {
	ops       *mailAPIOps
	accountID string
}

func (s *apiTokenSource) Token(context.Context) (string, error) {
	tokens, err := s.ops.validToken(s.accountID)
	if err != nil {
		return "", err
	}
	return tokens.AccessToken, nil
}

func (s *apiTokenSource) Refresh(_ context.Context, rejectedToken string) (string, error) {
	tokens, err := s.ops.tokenAfter(s.accountID, rejectedToken)
	if err != nil {
		return "", err
	}
	return tokens.AccessToken, nil
}

// submitViaAPI submits the final MIME through the account's staged or active
// Gmail/Graph API route. It is not called from any send path: the route switch
// is a later milestone. The attempt's unknown checkpoint is written before each
// request, and the final result is committed to the journal before returning,
// so callers may clean up locally only on a nil error. There is no SMTP
// fallback and no replay after an unknown outcome. A rejected grant is
// reported through refreshFailed, which asks for reauthorization and leaves
// protocol credentials alone.
func (ops *mailAPIOps) submitViaAPI(ctx context.Context, store *submission.Store, sendAs *gmailapi.Store,
	hc *http.Client, attempt *submission.Attempt, acc *account.Account, req submission.Request) (smtp.SubmissionResult, error) {
	route, err := ops.accountStore.GetMailAPIRoute(acc.ID)
	if err != nil {
		return smtp.SubmissionResult{Outcome: smtp.OutcomeNotSubmitted, Stage: "preflight"}, err
	}
	if route == nil {
		return smtp.SubmissionResult{Outcome: smtp.OutcomeNotSubmitted, Stage: "preflight"}, errors.New("account has no API route")
	}
	grant, err := ops.grantForRoute(route)
	if err != nil {
		return smtp.SubmissionResult{Outcome: smtp.OutcomeNotSubmitted, Stage: "preflight"}, err
	}
	// A linked shared mailbox sends as its own address through the owner's
	// mailbox grant; the signed-in mailbox is the owner's.
	own := acc.Email
	if route.Linked() {
		owner, oerr := ops.accountStore.Get(grant.ownerID)
		if oerr != nil {
			return smtp.SubmissionResult{Outcome: smtp.OutcomeNotSubmitted, Stage: "preflight"}, oerr
		}
		own = owner.Email
		if !strings.EqualFold(strings.TrimSpace(req.From), route.MailboxAddress) {
			return smtp.SubmissionResult{Outcome: smtp.OutcomeNotSubmitted, Stage: "preflight"},
				fmt.Errorf("a shared mailbox sends only as %s through the API", route.MailboxAddress)
		}
	}
	tokens, err := ops.credStore.GetOAuthTokensForClientConfig(grant.ownerID, grant.slot())
	if err != nil {
		return smtp.SubmissionResult{Outcome: smtp.OutcomeNotSubmitted, Stage: "preflight"}, fmt.Errorf("failed to get API authorization: %w", err)
	}
	source := &apiTokenSource{ops: ops, accountID: acc.ID}
	var sender submission.Sender
	switch route.Route {
	case account.MailRouteGmailAPI:
		cache, cacheErr := sendAs.Get(acc.ID)
		if cacheErr != nil {
			return smtp.SubmissionResult{Outcome: smtp.OutcomeNotSubmitted, Stage: "preflight"}, cacheErr
		}
		if err := gmailapi.Preflight(route, tokens, cache, req); err != nil {
			return smtp.SubmissionResult{Outcome: smtp.OutcomeNotSubmitted, Stage: "preflight"}, err
		}
		sender = &gmailapi.Sender{Tokens: source, HTTP: hc}
	case account.MailRouteGraphAPI:
		if err := graphapi.Preflight(route, tokens, own, req); err != nil {
			return smtp.SubmissionResult{Outcome: smtp.OutcomeNotSubmitted, Stage: "preflight"}, err
		}
		sender = &graphapi.Sender{Tokens: source, Own: own, HTTP: hc}
	default:
		return smtp.SubmissionResult{Outcome: smtp.OutcomeNotSubmitted, Stage: "preflight"}, fmt.Errorf("route %q is not an API route", route.Route)
	}
	result, err := sender.Send(ctx, req, func(r smtp.SubmissionResult) error {
		applySubmissionResult(attempt, r)
		return store.Update(attempt)
	})
	applySubmissionResult(attempt, result)
	if err != nil {
		return result, err
	}
	// Commit acceptance before any local cleanup; the unknown checkpoint stays
	// durable if this fails.
	if err := store.Update(attempt); err != nil {
		attempt.Outcome = "unknown"
		return result, fmt.Errorf("acceptance could not be recorded: %w; %v", err, submission.ReviewError(attempt))
	}
	return result, nil
}
