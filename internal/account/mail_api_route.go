package account

import (
	"database/sql"
	"encoding/json"
	"fmt"
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
	AccountID      string          `json:"accountId"`
	Route          string          `json:"route"`
	State          string          `json:"state"`
	ClientConfigID string          `json:"clientConfigId"`
	Shared         bool            `json:"shared"`
	Snapshot       MailAPISnapshot `json:"snapshot"`
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
	if r.State == "" {
		r.State = MailRouteStateStaged
	}
	snapshot, err := json.Marshal(r.Snapshot)
	if err != nil {
		return fmt.Errorf("failed to encode mail API snapshot: %w", err)
	}
	_, err = s.db.Exec(`INSERT INTO account_mail_api_routes(account_id,route,state,client_config_id,shared,snapshot,updated_at)
		VALUES(?,?,?,?,?,?,?) ON CONFLICT(account_id) DO UPDATE SET route=excluded.route,state=excluded.state,
		client_config_id=excluded.client_config_id,shared=excluded.shared,snapshot=excluded.snapshot,updated_at=excluded.updated_at`,
		r.AccountID, r.Route, r.State, r.ClientConfigID, boolToInt(r.Shared), string(snapshot), time.Now().Unix())
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
	err := s.db.QueryRow(`SELECT route,state,client_config_id,shared,snapshot FROM account_mail_api_routes WHERE account_id=?`,
		accountID).Scan(&r.Route, &r.State, &r.ClientConfigID, &shared, &snapshot)
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
