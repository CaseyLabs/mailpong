package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/credentials"
	"github.com/caseylabs/mailven/internal/logging"
	"github.com/caseylabs/mailven/internal/oauth2"
	"github.com/caseylabs/mailven/internal/platform"
)

// MailAPIRouteStatus describes an account's Gmail/Graph API authorization.
// Route "protocol" means no API route is selected. A staged route has consent
// but is not used for sync or sending yet.
type MailAPIRouteStatus struct {
	Route      string `json:"route"`
	State      string `json:"state"`
	Shared     bool   `json:"shared"`
	Authorized bool   `json:"authorized"`
	// Failure is the reason the stored API grant cannot be used: reauthorize,
	// partial-consent, or empty when none is known.
	Failure string `json:"failure"`
}

const mailAPIProtocolRoute = "protocol"

// mailAPIOps holds the API authorization lifecycle, kept apart from App so
// tests can drive it with local fixtures.
type mailAPIOps struct {
	accountStore  *account.Store
	credStore     *credentials.Store
	oauth2Manager *oauth2.Manager
	// baseProvider resolves the account's registered credentials and audience.
	baseProvider func(name string) (oauth2.ProviderConfig, error)
	emit         func(event string, data any)
}

// mailAPIConsent is a prepared authorization: the target and what it requests.
type mailAPIConsent struct {
	owner, route string
	account      *account.Account
	shared       bool
	profile      oauth2.MailAPIProfile
}

// prepare validates that accountID can start API consent and builds its profile.
func (ops *mailAPIOps) prepare(accountID string, shared bool) (*mailAPIConsent, error) {
	acc, err := ops.accountStore.Get(accountID)
	if err != nil {
		return nil, err
	}
	if acc.AuthType != account.AuthOAuth2 {
		return nil, fmt.Errorf("API authorization needs an OAuth account")
	}
	if acc.SharedMailboxParentID != "" {
		return nil, fmt.Errorf("API routes for linked shared mailboxes are not available yet; authorize the owning account")
	}
	provider, err := ops.credStore.GetOAuthProvider(accountID)
	if err != nil {
		return nil, fmt.Errorf("could not determine OAuth provider for account: %w", err)
	}
	consent := &mailAPIConsent{owner: accountID, account: acc, shared: shared}
	switch provider {
	case "google":
		consent.route = account.MailRouteGmailAPI
	case "microsoft":
		consent.route = account.MailRouteGraphAPI
		tokens, err := ops.credStore.GetOAuthTokens(accountID)
		if err != nil {
			return nil, fmt.Errorf("could not load account authorization: %w", err)
		}
		switch {
		case tokens.MicrosoftAccountType == "personal":
			return nil, fmt.Errorf("Microsoft Graph API routes support work/school accounts; personal Outlook.com accounts keep their current route")
		case shared && tokens.MicrosoftAccountType != "business":
			return nil, fmt.Errorf("shared-mailbox API access needs an account set up as work/school")
		}
	default:
		return nil, fmt.Errorf("API routes are unsupported for %q accounts", provider)
	}
	base, err := ops.baseProvider(provider)
	if err != nil {
		return nil, err
	}
	if base.ClientID == "" {
		return nil, fmt.Errorf("%w: %s", oauth2.ErrNotConfigured, provider)
	}
	base.LoginHint = acc.Email
	if consent.profile, err = oauth2.BuildMailAPIProfile(base, shared); err != nil {
		return nil, err
	}
	return consent, nil
}

// verifyIdentity requires the signed-in user to be the account's user. The
// stable Microsoft identity wins over the mutable email claim, as elsewhere.
func (ops *mailAPIOps) verifyIdentity(accountID string, tokens *oauth2.TokenResponse, email string) error {
	acc, err := ops.accountStore.Get(accountID)
	if err != nil {
		return err
	}
	stored, err := ops.accountStore.GetOAuthStableID(accountID)
	if err != nil {
		return err
	}
	if signedIn := oauth2.ExtractStableSubjectFromIDToken(tokens.IDToken); stored != "" && signedIn != "" {
		if stored != signedIn {
			return fmt.Errorf("the signed-in user is not this account's user")
		}
		return nil
	}
	if email == "" || !strings.EqualFold(email, acc.Email) {
		return fmt.Errorf("the signed-in user (%q) does not match this account", email)
	}
	return nil
}

// finish validates a completed consent and records the staged route. Nothing
// is stored when identity or granted permissions fall short, and protocol
// grants are never touched.
func (ops *mailAPIOps) finish(consent *mailAPIConsent, response *oauth2.TokenResponse, email string) error {
	slot := consent.profile.ClientConfigID
	if err := ops.verifyIdentity(consent.owner, response, email); err != nil {
		return err
	}
	tokens := &credentials.OAuthTokens{Provider: consent.profile.Provider.Name}
	tokens.ApplyTokenResponse(response, consent.profile.Provider.Scopes, oauth2.ScopeResource(consent.profile.Provider), false)
	for _, operation := range []string{"read", "write", "send"} {
		if err := tokens.RequireMailAPICapability(operation, consent.shared); err != nil {
			return &mailAPIFailure{kind: oauth2.MailAPIFailurePartialConsent, err: err}
		}
	}
	// Reauthorizing an active route keeps it active; it may not change route
	// type or mailbox scope while accounts are routed through it.
	state := account.MailRouteStateStaged
	if existing, gerr := ops.accountStore.GetMailAPIRoute(consent.owner); gerr == nil && existing != nil && existing.State == account.MailRouteStateActive {
		if existing.Route != consent.route || existing.Shared != consent.shared {
			return errors.New("deactivate the active API route before changing its route or mailbox scope")
		}
		state = account.MailRouteStateActive
	}
	unlock, err := ops.credStore.LockOAuthRefresh(consent.owner, slot)
	if err != nil {
		return err
	}
	defer unlock()
	prior, priorErr := ops.credStore.GetOAuthTokensForClientConfig(consent.owner, slot)
	if priorErr != nil && !errors.Is(priorErr, credentials.ErrCredentialNotFound) {
		return fmt.Errorf("could not read prior API authorization: %w", priorErr)
	}
	if tokens.RefreshToken == "" && prior != nil {
		tokens.RefreshToken = prior.RefreshToken
	}
	if err := ops.credStore.SetOAuthTokensForClientConfig(consent.owner, slot, tokens); err != nil {
		return fmt.Errorf("failed to store API authorization: %w", err)
	}
	p := consent.profile.Provider
	route := account.MailAPIRoute{
		AccountID: consent.owner, Route: consent.route, State: state,
		ClientConfigID: slot, Shared: consent.shared,
		Snapshot: account.MailAPISnapshot{Provider: p.Name, AuthURL: p.AuthURL, TokenURL: p.TokenURL,
			ClientID: p.ClientID, Scopes: append([]string(nil), p.Scopes...), Resource: oauth2.ScopeResource(p)},
	}
	if err := ops.accountStore.SetMailAPIRoute(route); err != nil {
		// Do not leave a grant without the route that describes it: drop a new
		// grant and put back the one the existing route still describes.
		if prior == nil {
			_ = ops.credStore.DeleteOAuthTokensForClientConfig(consent.owner, slot)
		} else {
			_ = ops.credStore.SetOAuthTokensForClientConfig(consent.owner, slot, prior)
		}
		return err
	}
	return nil
}

// mailAPIFailure carries a classified failure that did not come from the provider.
type mailAPIFailure struct {
	kind oauth2.MailAPIFailure
	err  error
}

func (f *mailAPIFailure) Error() string { return f.err.Error() }
func (f *mailAPIFailure) Unwrap() error { return f.err }

func classifyMailAPI(err error) oauth2.MailAPIFailure {
	var f *mailAPIFailure
	if errors.As(err, &f) {
		return f.kind
	}
	return oauth2.ClassifyMailAPIError(err)
}

// validToken returns a usable API access token, refreshing under the slot's
// cross-process lock. It refreshes with the consented snapshot, not current
// settings, and never reads or writes the protocol slot.
func (ops *mailAPIOps) validToken(accountID string) (*credentials.OAuthTokens, error) {
	return ops.tokenAfter(accountID, "")
}

// tokenAfter is validToken that also treats the rejected access token (a 401
// from the API) as unusable: it refreshes under the lock unless another holder
// already rotated it. An empty rejected token is the ordinary expiry check.
func (ops *mailAPIOps) tokenAfter(accountID, rejected string) (*credentials.OAuthTokens, error) {
	usable := func(t *credentials.OAuthTokens) bool {
		return !t.IsExpiringSoon(5*time.Minute) && (rejected == "" || t.AccessToken != rejected)
	}
	route, err := ops.accountStore.GetMailAPIRoute(accountID)
	if err != nil {
		return nil, err
	}
	if route == nil {
		return nil, fmt.Errorf("account has no API route")
	}
	tokens, err := ops.credStore.GetOAuthTokensForClientConfig(accountID, route.ClientConfigID)
	if err != nil {
		return nil, fmt.Errorf("failed to get API authorization: %w", err)
	}
	if rejected == "" && usable(tokens) {
		return tokens, nil
	}
	unlock, err := ops.credStore.LockOAuthRefresh(accountID, route.ClientConfigID)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire API refresh lock: %w", err)
	}
	defer unlock()
	// Another holder may have rotated the token while this caller waited.
	if tokens, err = ops.credStore.GetOAuthTokensForClientConfig(accountID, route.ClientConfigID); err != nil {
		return nil, fmt.Errorf("failed to get API authorization: %w", err)
	}
	if usable(tokens) {
		return tokens, nil
	}
	if tokens.RefreshToken == "" {
		return nil, ops.refreshFailed(accountID, route, fmt.Errorf("%w: no refresh token stored", oauth2.ErrInvalidGrant))
	}
	provider, err := ops.snapshotProvider(route)
	if err != nil {
		return nil, ops.refreshFailed(accountID, route, err)
	}
	response, err := ops.oauth2Manager.RefreshTokenWithProvider(provider, tokens.RefreshToken)
	if err != nil {
		return nil, ops.refreshFailed(accountID, route, err)
	}
	tokens.ApplyTokenResponse(response, nil, "", true)
	if err := ops.credStore.SetOAuthTokensForClientConfig(accountID, route.ClientConfigID, tokens); err != nil {
		return nil, fmt.Errorf("failed to persist refreshed API authorization: %w", err)
	}
	return tokens, nil
}

// snapshotProvider rebuilds the consented provider. The secret comes from the
// currently resolved mail credentials, and only for the same client.
func (ops *mailAPIOps) snapshotProvider(route *account.MailAPIRoute) (oauth2.ProviderConfig, error) {
	snapshot := route.Snapshot
	current, err := ops.baseProvider(snapshot.Provider)
	if err != nil {
		return oauth2.ProviderConfig{}, err
	}
	if current.ClientID != snapshot.ClientID {
		return oauth2.ProviderConfig{}, &mailAPIFailure{kind: oauth2.MailAPIFailureReauthorize,
			err: fmt.Errorf("OAuth client changed since API authorization: %w", oauth2.ErrInvalidGrant)}
	}
	return oauth2.ProviderConfig{Name: snapshot.Provider, TokenURL: snapshot.TokenURL, AuthURL: snapshot.AuthURL,
		ClientID: snapshot.ClientID, ClientSecret: current.ClientSecret, Scopes: snapshot.Scopes}, nil
}

// refreshFailed reports a refresh failure. Only a rejected grant clears the
// refresh token and asks for reauthorization; other failures are retried later.
func (ops *mailAPIOps) refreshFailed(accountID string, route *account.MailAPIRoute, err error) error {
	log := logging.WithComponent("app.mailapi")
	kind := classifyMailAPI(err)
	if errors.Is(err, oauth2.ErrInvalidGrant) {
		if clearErr := ops.credStore.ClearOAuthRefreshTokenForClientConfig(accountID, route.ClientConfigID); clearErr != nil {
			log.Warn().Err(clearErr).Str("account_id", accountID).Msg("Failed to clear rejected API refresh token")
		}
		if kind == oauth2.MailAPIFailureNone || kind == oauth2.MailAPIFailureOther {
			kind = oauth2.MailAPIFailureReauthorize
		}
		ops.emit("mail-api:reauth-required", map[string]any{"accountId": accountID, "route": route.Route, "failure": string(kind), "error": err.Error()})
	}
	log.Error().Err(err).Str("account_id", accountID).Str("failure", string(kind)).Msg("API token refresh failed")
	return fmt.Errorf("API token refresh failed (%s): %w", kind, err)
}

func (ops *mailAPIOps) status(accountID string) (MailAPIRouteStatus, error) {
	route, err := ops.accountStore.GetMailAPIRoute(accountID)
	if err != nil {
		return MailAPIRouteStatus{}, err
	}
	if route == nil {
		return MailAPIRouteStatus{Route: mailAPIProtocolRoute}, nil
	}
	status := MailAPIRouteStatus{Route: route.Route, State: route.State, Shared: route.Shared}
	tokens, err := ops.credStore.GetOAuthTokensForClientConfig(accountID, route.ClientConfigID)
	switch {
	case errors.Is(err, credentials.ErrCredentialNotFound):
		status.Failure = string(oauth2.MailAPIFailureReauthorize)
	case err != nil:
		return status, err
	case tokens.RefreshToken == "":
		status.Failure = string(oauth2.MailAPIFailureReauthorize)
	case ops.clientChanged(route):
		status.Failure = string(oauth2.MailAPIFailureReauthorize)
	default:
		operations := []string{"read", "write", "send"}
		for _, operation := range operations {
			if tokens.RequireMailAPICapability(operation, route.Shared) != nil {
				status.Failure = string(oauth2.MailAPIFailurePartialConsent)
				return status, nil
			}
		}
		status.Authorized = true
	}
	return status, nil
}

// clientChanged reports whether the registered OAuth client no longer matches
// the one the API grant was consented with.
func (ops *mailAPIOps) clientChanged(route *account.MailAPIRoute) bool {
	current, err := ops.baseProvider(route.Snapshot.Provider)
	return err != nil || current.ClientID != route.Snapshot.ClientID
}

// clear returns the account to its protocol route and deletes only the API grant.
func (ops *mailAPIOps) clear(accountID string) error {
	route, err := ops.accountStore.GetMailAPIRoute(accountID)
	if err != nil || route == nil {
		return err
	}
	unlock, err := ops.credStore.LockOAuthRefresh(accountID, route.ClientConfigID)
	if err != nil {
		return err
	}
	defer unlock()
	if err := ops.accountStore.ClearMailAPIRoute(accountID); err != nil {
		return err
	}
	return ops.credStore.DeleteOAuthTokensForClientConfig(accountID, route.ClientConfigID)
}

func (a *App) apiProviderLookup() func(string) (oauth2.ProviderConfig, error) {
	if a.apiRoutes.provider != nil {
		return a.apiRoutes.provider
	}
	return oauth2.GetProvider
}

func (a *App) mailAPIOps() *mailAPIOps {
	return &mailAPIOps{accountStore: a.accountStore, credStore: a.credStore, oauth2Manager: a.oauth2Manager,
		baseProvider: a.apiProviderLookup(),
		emit:         func(event string, data any) { eventsEmit(a.ctx, event, data) }}
}

// StartMailAPIConsent starts user-invoked consent for the account's Gmail or
// Microsoft Graph API route. Success only stages the route: sync and sending
// keep using IMAP/SMTP. Emits mail-api:started, mail-api:success and
// mail-api:error (with a failure category: policy, resource, partial-consent,
// reauthorize or other).
func (a *App) StartMailAPIConsent(accountID string, shared bool) error {
	log := logging.WithComponent("app.mailapi")
	ops := a.mailAPIOps()
	consent, err := ops.prepare(accountID, shared)
	if err != nil {
		return err
	}
	authURL, err := a.oauth2Manager.StartAuthFlowWithProvider(a.ctx, &consent.profile.Provider)
	if err != nil {
		return fmt.Errorf("failed to start API authorization: %w", err)
	}
	eventsEmit(a.ctx, "mail-api:started", map[string]any{"accountId": accountID, "route": consent.route, "authURL": authURL})
	if perr := platform.PortalOpenURI(authURL); perr != nil {
		log.Debug().Err(perr).Msg("Portal OpenURI failed, falling back to BrowserOpenURL")
		wailsRuntime.BrowserOpenURL(a.ctx, authURL)
	}
	go func() {
		defer recoverPanic("app.mailapi", "API consent callback")
		fail := func(err error) {
			log.Error().Err(err).Str("account_id", accountID).Msg("API authorization failed")
			eventsEmit(a.ctx, "mail-api:error", map[string]any{
				"accountId": accountID, "failure": string(classifyMailAPI(err)), "error": err.Error()})
		}
		response, email, err := a.oauth2Manager.WaitForCallback(a.ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			fail(err)
			return
		}
		if err := ops.finish(consent, response, email); err != nil {
			fail(err)
			return
		}
		a.apiRoutes.setPaused(accountID, false)
		a.kickAPIActions(accountID)
		eventsEmit(a.ctx, "mail-api:success", map[string]any{"accountId": accountID, "route": consent.route, "shared": shared})
	}()
	return nil
}

// CancelMailAPIConsent cancels an in-progress API consent without changing any
// route or grant.
func (a *App) CancelMailAPIConsent() {
	a.oauth2Manager.CancelAuthFlow()
	eventsEmit(a.ctx, "mail-api:cancelled", nil)
}

// GetMailAPIRouteStatus reports the account's API route and whether its stored
// grant is usable. It makes no network call.
func (a *App) GetMailAPIRouteStatus(accountID string) (MailAPIRouteStatus, error) {
	return a.mailAPIOps().status(accountID)
}

// ClearMailAPIRoute returns the account to its protocol route and deletes the
// API grant. Protocol credentials, drafts and queued mail are untouched.
func (a *App) ClearMailAPIRoute(accountID string) error {
	a.apiRoutes.transition.Lock()
	defer a.apiRoutes.transition.Unlock()
	if a.apiRouted(accountID) != nil {
		if err := a.deactivateAPIRoute(accountID); err != nil {
			return err
		}
	}
	return a.mailAPIOps().clear(accountID)
}
