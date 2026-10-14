package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/credentials"
	"github.com/caseylabs/mailven/internal/gmailapi"
	"github.com/caseylabs/mailven/internal/logging"
)

// sendAsTTL is how long a fetched Gmail send-as list stays fresh before the
// automatic refresh fetches it again.
const sendAsTTL = 24 * time.Hour

// A failed lookup or implausible future observation cannot be current evidence.
func freshSendAs(cached gmailapi.Cached) bool {
	now := time.Now()
	return !cached.LookupFailed && !cached.FetchedAt.IsZero() && !cached.FetchedAt.After(now) && now.Sub(cached.FetchedAt) < sendAsTTL
}

// SendAsAlias is a verified Gmail send-as address that is not yet an identity.
type SendAsAlias struct {
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	ReplyTo     string `json:"replyTo"`
}

// SendAsStatus is the Gmail send-as state of an account. Supported is false
// for anything but a Google OAuth account, and the UI shows nothing then.
type SendAsStatus struct {
	Supported    bool              `json:"supported"`
	AccountEmail string            `json:"accountEmail"` // the address Gmail rewrites unverified senders to
	Fetched      bool              `json:"fetched"`      // a send-as list has been cached
	Identities   map[string]string `json:"identities"`   // identity ID -> verified|pending|unknown|not-configured
	Available    []SendAsAlias     `json:"available"`    // verified aliases without an identity
}

// isGoogleOAuth reports whether the account is a Gmail account signed in
// through OAuth; only those have a send-as list to read.
func isGoogleOAuth(acc *account.Account) bool {
	return acc != nil && acc.AuthType == account.AuthOAuth2 && acc.IsGmail()
}

// errSendAsUnsupported is returned for send-as actions on non-Gmail accounts.
var errSendAsUnsupported = errors.New("send-as addresses are only available for Gmail accounts")

// sendAsStatus reads the cached state; it makes no network calls.
func (ops *composeOps) sendAsStatus(accountID string) (*SendAsStatus, error) {
	acc, err := ops.accountStore.Get(accountID)
	if err != nil {
		return nil, err
	}
	st := &SendAsStatus{Identities: map[string]string{}, Available: []SendAsAlias{}}
	if !isGoogleOAuth(acc) {
		return st, nil
	}
	st.Supported = true
	st.AccountEmail = acc.Email

	cached, err := ops.sendAsStore.Get(accountID)
	if err != nil {
		return nil, err
	}
	st.Fetched = !cached.FetchedAt.IsZero()
	fresh := freshSendAs(cached)

	identities, err := ops.accountStore.GetIdentities(accountID)
	if err != nil {
		return nil, err
	}
	have := make(map[string]bool, len(identities))
	for _, id := range identities {
		st.Identities[id.ID] = cached.StatusFor(id.Email)
		if cached.LookupFailed {
			st.Identities[id.ID] = "unknown"
		} else if !cached.FetchedAt.IsZero() && !fresh {
			st.Identities[id.ID] = "stale"
		}
		have[strings.ToLower(id.Email)] = true
	}
	for _, e := range cached.Entries {
		if have[strings.ToLower(e.Email)] || !e.Verified() || !fresh {
			continue
		}
		st.Available = append(st.Available, SendAsAlias{Email: e.Email, DisplayName: e.DisplayName, ReplyTo: e.ReplyTo})
	}
	return st, nil
}

// refreshSendAs caches successful Gmail lookups. Failed refreshes retain old
// observations for context but invalidate them as current sender evidence.
func (ops *composeOps) refreshSendAs(ctx context.Context, accountID string) (refreshErr error) {
	log := logging.WithComponent("sendas")

	acc, err := ops.accountStore.Get(accountID)
	if err != nil {
		return err
	}
	if !isGoogleOAuth(acc) {
		return errSendAsUnsupported
	}
	defer func() {
		if refreshErr != nil {
			if err := ops.sendAsStore.MarkLookupFailed(accountID, time.Now().UTC()); err != nil {
				refreshErr = fmt.Errorf("%w; could not record lookup failure: %v", refreshErr, err)
			}
		}
	}()
	route, err := ops.accountStore.GetMailAPIRoute(accountID)
	if err != nil {
		return err
	}
	var tokens *credentials.OAuthTokens
	if route != nil && route.State == account.MailRouteStateActive && route.Route == account.MailRouteGmailAPI {
		tokens, err = ops.apiOps().validToken(accountID)
		if err == nil {
			err = tokens.RequireMailAPICapability("aliases", false)
		}
	} else {
		tokens, err = ops.getValidOAuthToken(ctx, accountID)
	}
	if err != nil {
		return fmt.Errorf("failed to get OAuth token: %w", err)
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	list, err := gmailapi.ListSendAs(fetchCtx, tokens.AccessToken)
	if err != nil {
		if errors.Is(err, gmailapi.ErrInsufficientScope) {
			log.Warn().Str("account_id", accountID).Msg("Gmail send-as list denied; the token lacks the Gmail scope")
			return errors.New("Gmail denied access to the send-as list; the account may need to be re-authorized, or the Gmail API enabled for the OAuth client")
		}
		log.Warn().Err(err).Str("account_id", accountID).Msg("Failed to fetch Gmail send-as list")
		return err
	}
	if err := ops.sendAsStore.Replace(accountID, list, time.Now().UTC()); err != nil {
		return err
	}
	log.Info().Str("account_id", accountID).Int("count", len(list)).Msg("Gmail send-as list refreshed")
	return nil
}

// refreshStaleSendAs refreshes every enabled Gmail account whose cache is
// older than sendAsTTL.
func (ops *composeOps) refreshStaleSendAs(ctx context.Context) {
	log := logging.WithComponent("sendas")
	accounts, err := ops.accountStore.List()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to list accounts for send-as refresh")
		return
	}
	for _, acc := range accounts {
		if !acc.Enabled || !isGoogleOAuth(acc) {
			continue
		}
		cached, err := ops.sendAsStore.Get(acc.ID)
		if err == nil && freshSendAs(cached) {
			continue
		}
		_ = ops.refreshSendAs(ctx, acc.ID) // logged inside
	}
}

// importSendAs creates identities from cached verified aliases.
func (ops *composeOps) importSendAs(accountID string, emails []string, withSignature bool) ([]*account.Identity, error) {
	acc, err := ops.accountStore.Get(accountID)
	if err != nil {
		return nil, err
	}
	if !isGoogleOAuth(acc) {
		return nil, errSendAsUnsupported
	}
	cached, err := ops.sendAsStore.Get(accountID)
	if err != nil {
		return nil, err
	}
	if !freshSendAs(cached) {
		return nil, errors.New("refresh Gmail sender evidence before importing aliases")
	}
	existing, err := ops.accountStore.GetIdentities(accountID)
	if err != nil {
		return nil, err
	}
	have := make(map[string]bool, len(existing))
	fallbackName := acc.Name
	for _, id := range existing {
		have[strings.ToLower(id.Email)] = true
		if id.IsDefault && id.Name != "" {
			fallbackName = id.Name
		}
	}

	verified := make(map[string]gmailapi.SendAs, len(cached.Entries))
	for _, e := range cached.Entries {
		if e.Verified() {
			verified[strings.ToLower(e.Email)] = e
		}
	}

	var created []*account.Identity
	for _, want := range emails {
		key := strings.ToLower(want)
		e, ok := verified[key]
		if !ok || have[key] {
			continue
		}
		cfg := account.IdentityConfig{
			Email:               e.Email,
			Name:                e.DisplayName,
			SignatureEnabled:    true,
			SignatureForNew:     true,
			SignatureForReply:   true,
			SignatureForForward: true,
			SignaturePlacement:  "above",
		}
		if cfg.Name == "" {
			cfg.Name = fallbackName
		}
		if account.IsPlainAddress(e.ReplyTo) {
			cfg.ReplyTo = e.ReplyTo
		}
		if withSignature {
			cfg.SignatureHTML = e.Signature
		}
		id, err := ops.accountStore.CreateIdentity(accountID, &cfg)
		if err != nil {
			return created, fmt.Errorf("import %s: %w", e.Email, err)
		}
		have[key] = true
		created = append(created, id)
	}
	return created, nil
}

// GetSendAsStatus returns the cached Gmail send-as state of an account's
// identities. It never calls Gmail.
func (a *App) GetSendAsStatus(accountID string) (*SendAsStatus, error) {
	return a.composeOps.sendAsStatus(accountID)
}

// RefreshSendAs fetches the account's Gmail send-as list now.
func (a *App) RefreshSendAs(accountID string) (*SendAsStatus, error) {
	if err := a.composeOps.refreshSendAs(a.ctx, accountID); err != nil {
		return nil, err
	}
	return a.composeOps.sendAsStatus(accountID)
}

// ImportSendAs adds the chosen verified Gmail aliases as identities. Gmail's
// signature is imported only when withSignature is set.
func (a *App) ImportSendAs(accountID string, emails []string, withSignature bool) ([]*account.Identity, error) {
	return a.composeOps.importSendAs(accountID, emails, withSignature)
}

// GetSendAsStatus returns the cached Gmail send-as state for the composer's
// rewrite warning.
func (c *ComposerApp) GetSendAsStatus(accountID string) (*SendAsStatus, error) {
	return c.composeOps.sendAsStatus(accountID)
}
