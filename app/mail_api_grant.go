package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/caseylabs/mailven/internal/account"
)

// errAPIGrantOwner marks a route whose grant owner cannot be established. The
// route then sends no request and keeps its local data.
var errAPIGrantOwner = errors.New("shared mailbox API route has no valid grant owner")

// apiGrant is a route resolved to the account whose token slot holds its API
// grant. For an own-mailbox route the owner is the route itself. For a linked
// shared-mailbox child the owner is the validated parent and its route is the
// authoritative snapshot; the child never holds a copy of the grant.
type apiGrant struct {
	route      *account.MailAPIRoute
	ownerID    string
	ownerRoute *account.MailAPIRoute
}

// slot is the token slot of the grant.
func (g *apiGrant) slot() string { return g.ownerRoute.ClientConfigID }

// grantFor resolves the account's stored route to its grant owner.
func (ops *mailAPIOps) grantFor(accountID string) (*apiGrant, error) {
	route, err := ops.accountStore.GetMailAPIRoute(accountID)
	if err != nil {
		return nil, err
	}
	if route == nil {
		return nil, fmt.Errorf("account has no API route")
	}
	return ops.grantForRoute(route)
}

// grantForRoute validates a route's owner and target on every use: the link,
// the owner's own staged or active Graph route, and the target address, so a
// legacy orphan or a changed address fails closed.
func (ops *mailAPIOps) grantForRoute(route *account.MailAPIRoute) (*apiGrant, error) {
	if !route.Linked() {
		if route.MailboxAddress != "" {
			return nil, fmt.Errorf("%w: target without owner", errAPIGrantOwner)
		}
		return &apiGrant{route: route, ownerID: route.AccountID, ownerRoute: route}, nil
	}
	if route.Route != account.MailRouteGraphAPI || !route.Shared || route.MailboxAddress == "" {
		return nil, fmt.Errorf("%w: incomplete route", errAPIGrantOwner)
	}
	child, err := ops.accountStore.Get(route.AccountID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errAPIGrantOwner, err)
	}
	if child.SharedMailboxParentID == "" || child.SharedMailboxParentID != route.GrantAccountID {
		return nil, fmt.Errorf("%w: account is not linked to the grant owner", errAPIGrantOwner)
	}
	if !strings.EqualFold(strings.TrimSpace(child.Email), route.MailboxAddress) {
		return nil, fmt.Errorf("%w: mailbox address changed", errAPIGrantOwner)
	}
	owner, err := ops.accountStore.Get(route.GrantAccountID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errAPIGrantOwner, err)
	}
	if owner.SharedMailboxParentID != "" || owner.AuthType != account.AuthOAuth2 {
		return nil, fmt.Errorf("%w: invalid owner account", errAPIGrantOwner)
	}
	ownerRoute, err := ops.accountStore.GetMailAPIRoute(owner.ID)
	switch {
	case err != nil:
		return nil, err
	case ownerRoute == nil || ownerRoute.Linked() || ownerRoute.Route != route.Route || ownerRoute.ClientConfigID != route.ClientConfigID:
		return nil, fmt.Errorf("%w: owner has no matching API authorization", errAPIGrantOwner)
	}
	return &apiGrant{route: route, ownerID: owner.ID, ownerRoute: ownerRoute}, nil
}

// dependents lists the child accounts whose routes reference the owner's grant.
func (ops *mailAPIOps) dependents(ownerID string) ([]string, error) {
	return ops.accountStore.ListMailAPIRoutesGrantedBy(ownerID)
}

// sharedTarget is the shared mailbox a route addresses ("" for the signed-in
// user's own mailbox).
func sharedTarget(route *account.MailAPIRoute) string {
	if !route.Shared {
		return ""
	}
	return route.MailboxAddress
}

// sharedMailboxLiveVerified is the recorded-evidence gate for shared mailbox
// migration and activation. It stays false until the live checks of
// docs/SHARED_MAILBOX_API_DESIGN.md (Full Access, Send As / Send on Behalf,
// Sent copy, Bcc privacy) are recorded in docs/validation/; tests flip it.
var sharedMailboxLiveVerified = false

var errSharedMailboxGate = errors.New("shared mailbox API routes stay on IMAP until the live Graph checks are recorded")

// requireSharedCapabilities checks the owner's grant for the shared read,
// write and send permissions a linked mailbox route needs.
func (ops *mailAPIOps) requireSharedCapabilities(accountID string) error {
	grant, err := ops.grantFor(accountID)
	if err != nil {
		return err
	}
	tokens, err := ops.credStore.GetOAuthTokensForClientConfig(grant.ownerID, grant.slot())
	if err != nil {
		return fmt.Errorf("failed to get API authorization: %w", err)
	}
	for _, op := range []string{"read", "write", "send"} {
		if err := tokens.RequireMailAPICapability(op, true); err != nil {
			return err
		}
	}
	return nil
}

// cancelAPISync stops a running API pass of the account, if any.
func (a *App) cancelAPISync(accountID string) {
	rs := &a.apiRoutes
	rs.mu.Lock()
	cancel := rs.cancels[accountID]
	rs.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}
