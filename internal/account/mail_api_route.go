package account

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Mail API routes. A stored row records an explicit user selection; no
// transport reads it until the submission and sync milestones activate it.
const (
	MailRouteGmailAPI = "gmail-api"
	MailRouteGraphAPI = "graph-api"

	// MailRouteStateStaged is consented but inactive.
	MailRouteStateStaged = "staged"
	// MailRouteStateActive routes user actions through the API. Only route
	// activation (F4e-3) sets it; until then no account is active.
	MailRouteStateActive = "active"
)

// MailAPISnapshot is the non-secret profile used at consent. Refresh uses it
// instead of today's settings so a later endpoint or credential change cannot
// silently redirect the grant. The client secret is never stored here.
type MailAPISnapshot struct {
	Provider string   `json:"provider"`
	AuthURL  string   `json:"authUrl"`
	TokenURL string   `json:"tokenUrl"`
	ClientID string   `json:"clientId"`
	Scopes   []string `json:"scopes"`
	Resource string   `json:"resource"`
}

// MailAPIRoute is an account's recorded API route. No row means protocol.
type MailAPIRoute struct {
	AccountID      string `json:"accountId"`
	Route          string `json:"route"`
	State          string `json:"state"`
	ClientConfigID string `json:"clientConfigId"`
	Shared         bool   `json:"shared"`
	// MailboxAddress is the lower-cased shared mailbox a linked child route
	// addresses; empty for an account's own mailbox.
	MailboxAddress string `json:"mailboxAddress"`
	// GrantAccountID is the account whose token slot holds the API grant for
	// a linked child route; empty when the route's own account holds it.
	GrantAccountID string          `json:"grantAccountId"`
	Snapshot       MailAPISnapshot `json:"snapshot"`
}

// Linked reports whether the route addresses a shared mailbox through a grant
// owned by another account.
func (r *MailAPIRoute) Linked() bool { return r.GrantAccountID != "" }

// GrantOwnerID is the account whose token slot holds the route's API grant.
func (r *MailAPIRoute) GrantOwnerID() string {
	if r.GrantAccountID != "" {
		return r.GrantAccountID
	}
	return r.AccountID
}

// SetMailAPIRoute records or replaces the account's API route selection.
func (s *Store) SetMailAPIRoute(r MailAPIRoute) error {
	switch r.Route {
	case MailRouteGmailAPI, MailRouteGraphAPI:
	default:
		return fmt.Errorf("unsupported mail API route %q", r.Route)
	}
	if r.AccountID == "" || r.ClientConfigID == "" {
		return fmt.Errorf("mail API route needs an account and token slot")
	}
	r.MailboxAddress = strings.ToLower(strings.TrimSpace(r.MailboxAddress))
	switch {
	case r.GrantAccountID == r.AccountID && r.GrantAccountID != "":
		return fmt.Errorf("mail API route cannot name its own account as grant owner")
	case (r.GrantAccountID == "") != (r.MailboxAddress == ""):
		return fmt.Errorf("a shared mailbox API route needs both a target mailbox and a grant owner")
	case r.MailboxAddress != "" && !r.Shared:
		return fmt.Errorf("a shared mailbox target needs the shared route mode")
	}
	if r.State == "" {
		r.State = MailRouteStateStaged
	}
	snapshot, err := json.Marshal(r.Snapshot)
	if err != nil {
		return fmt.Errorf("failed to encode mail API snapshot: %w", err)
	}
	_, err = s.db.Exec(`INSERT INTO account_mail_api_routes(account_id,route,state,client_config_id,shared,snapshot,updated_at,mailbox_address,grant_account_id)
		VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(account_id) DO UPDATE SET route=excluded.route,state=excluded.state,
		client_config_id=excluded.client_config_id,shared=excluded.shared,snapshot=excluded.snapshot,updated_at=excluded.updated_at,
		mailbox_address=excluded.mailbox_address,grant_account_id=excluded.grant_account_id`,
		r.AccountID, r.Route, r.State, r.ClientConfigID, boolToInt(r.Shared), string(snapshot), time.Now().Unix(), r.MailboxAddress, r.GrantAccountID)
	if err != nil {
		return fmt.Errorf("failed to store mail API route: %w", err)
	}
	return nil
}

// GetMailAPIRoute returns the recorded route, or nil for a protocol account.
func (s *Store) GetMailAPIRoute(accountID string) (*MailAPIRoute, error) {
	r := MailAPIRoute{AccountID: accountID}
	var shared int
	var snapshot string
	err := s.db.QueryRow(`SELECT route,state,client_config_id,shared,snapshot,mailbox_address,grant_account_id FROM account_mail_api_routes WHERE account_id=?`,
		accountID).Scan(&r.Route, &r.State, &r.ClientConfigID, &shared, &snapshot, &r.MailboxAddress, &r.GrantAccountID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read mail API route: %w", err)
	}
	r.Shared = shared != 0
	if err := json.Unmarshal([]byte(snapshot), &r.Snapshot); err != nil {
		return nil, fmt.Errorf("failed to decode mail API snapshot: %w", err)
	}
	return &r, nil
}

// ClearMailAPIRoute returns the account to its protocol route.
func (s *Store) ClearMailAPIRoute(accountID string) error {
	if _, err := s.db.Exec(`DELETE FROM account_mail_api_routes WHERE account_id=?`, accountID); err != nil {
		return fmt.Errorf("failed to clear mail API route: %w", err)
	}
	return nil
}

// ListMailAPIRoutesGrantedBy returns the child routes that reference the
// account's API grant, staged or active.
func (s *Store) ListMailAPIRoutesGrantedBy(ownerID string) ([]string, error) {
	rows, err := s.db.Query(`SELECT account_id FROM account_mail_api_routes WHERE grant_account_id=? ORDER BY account_id`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("failed to list dependent mail API routes: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
