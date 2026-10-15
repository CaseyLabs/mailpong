package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	coreapi "github.com/caseylabs/mailven/internal/core/api/v1"
	"github.com/caseylabs/mailven/internal/googlecheck"
	"github.com/caseylabs/mailven/internal/logging"
)

const (
	googleContactsScope = "https://www.googleapis.com/auth/contacts.readonly"
	googleCalendarScope = "https://www.googleapis.com/auth/calendar"
)

// GoogleSetupCheck is the result of "Check Google setup" for one account.
type GoogleSetupCheck struct {
	AccountID string               `json:"accountId"`
	CheckedAt time.Time            `json:"checkedAt"`
	Results   []googlecheck.Result `json:"results"`
}

// bearerTransport adds a fixed access token to each request.
type bearerTransport struct{ token string }

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

// CheckGoogleSetup probes the Google APIs a Google OAuth account relies on
// with read-only requests: Gmail always, People and Calendar when the account
// has a grant for them. It runs only when called; nothing polls.
func (a *App) CheckGoogleSetup(accountID string) (*GoogleSetupCheck, error) {
	log := logging.WithComponent("google-check")
	acc, err := a.accountStore.Get(accountID)
	if err != nil {
		return nil, err
	}
	if !isGoogleOAuth(acc) {
		return nil, errors.New("Check Google setup is only available for Google OAuth accounts")
	}
	ctx, cancel := context.WithTimeout(a.ctx, 60*time.Second)
	defer cancel()

	out := &GoogleSetupCheck{AccountID: accountID, CheckedAt: time.Now().UTC()}
	if tokens, err := a.composeOps.sendAsToken(ctx, accountID); err != nil {
		out.Results = append(out.Results, googlecheck.Failure(googlecheck.APIGmail, err))
	} else {
		client := &http.Client{Transport: bearerTransport{tokens.AccessToken}}
		out.Results = append(out.Results, googlecheck.Gmail(ctx, client))
	}
	out.Results = append(out.Results,
		a.checkBrokered(ctx, accountID, googlecheck.APIPeople, googleContactsScope, "Contacts access has not been granted for this account.", googlecheck.People),
		a.checkBrokered(ctx, accountID, googlecheck.APICalendar, googleCalendarScope, "Calendar access has not been granted for this account.", googlecheck.Calendar),
	)
	for _, r := range out.Results {
		log.Info().Str("account_id", accountID).Str("api", r.API).Str("state", r.State).Msg("Google setup check")
	}
	return out, nil
}

// checkBrokered probes an API through the auth broker, which refreshes tokens
// and reports a missing grant instead of prompting for consent.
func (a *App) checkBrokered(ctx context.Context, accountID, api, scope, notConnected string, probe func(context.Context, *http.Client) googlecheck.Result) googlecheck.Result {
	client, err := a.authBroker.HTTPClient(accountID, []coreapi.AuthScope{{Resource: scope, Reason: "Check Google setup"}})
	var consent *coreapi.ErrAdditionalConsentRequired
	switch {
	case errors.As(err, &consent):
		return googlecheck.NotConnected(api, notConnected)
	case err != nil:
		return googlecheck.Failure(api, fmt.Errorf("could not get access: %w", err))
	}
	return probe(ctx, client)
}
