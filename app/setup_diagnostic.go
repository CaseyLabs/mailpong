package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/credentials"
	"github.com/caseylabs/mailven/internal/platform"
	"github.com/caseylabs/mailven/internal/setupdiagnostic"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type SetupDiagnosticRequest struct {
	RunID         string   `json:"runId"`
	AccountID     string   `json:"accountId"`
	From          string   `json:"from"`
	Route         string   `json:"route"`
	Domain        string   `json:"domain"`
	OwnedDomain   bool     `json:"ownedDomain"`
	DKIMSelectors []string `json:"dkimSelectors"`
}
type SetupDiagnosticReport struct {
	RunID            string                    `json:"runId"`
	AccountID        string                    `json:"accountId"`
	Mailbox          string                    `json:"mailbox"`
	From             string                    `json:"from"`
	Route            string                    `json:"route"`
	StartedAt        string                    `json:"startedAt"`
	FinishedAt       string                    `json:"finishedAt"`
	Endpoints        []string                  `json:"endpoints"`
	OAuthResource    string                    `json:"oauthResource"`
	OAuthExpiresAt   string                    `json:"oauthExpiresAt"`
	OAuthScopes      []string                  `json:"oauthScopes"`
	OAuthScopesKnown bool                      `json:"oauthScopesKnown"`
	Checks           []setupdiagnostic.Check   `json:"checks"`
	DNS              setupdiagnostic.DNSResult `json:"dns"`
	Limitations      string                    `json:"limitations"`
}
type setupDiagnosticState struct {
	mu          sync.Mutex
	id          string
	cancel      context.CancelFunc
	cancelledID string
	report      *SetupDiagnosticReport
}

var diagnosticRunID = regexp.MustCompile(`^[a-zA-Z0-9-]{1,80}$`)
var diagnosticScope = regexp.MustCompile(`^[a-zA-Z0-9:/._-]+$`)
var diagnosticHost = regexp.MustCompile(`^[a-zA-Z0-9.-]{1,253}$`)

func safeDiagnosticHost(host string) bool {
	return diagnosticHost.MatchString(host) || net.ParseIP(host) != nil
}

// RunSetupDiagnostic is called only by the settings Start button. There is no
// startup hook, send dependency, reauthorization event, or background scheduler.
func (a *App) RunSetupDiagnostic(req SetupDiagnosticRequest) (*SetupDiagnosticReport, error) {
	if !diagnosticRunID.MatchString(req.RunID) || req.Route != "imap-smtp" {
		return nil, fmt.Errorf("choose the existing IMAP/SMTP route and a valid run ID")
	}
	acc, err := a.accountStore.Get(req.AccountID)
	if err != nil || acc == nil {
		return nil, fmt.Errorf("account unavailable")
	}
	identities, err := a.accountStore.GetIdentities(acc.ID)
	if err != nil {
		return nil, fmt.Errorf("identities unavailable")
	}
	found := false
	for _, id := range identities {
		if id.Email == req.From {
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("choose an existing From identity")
	}
	if !safeDiagnosticHost(acc.IMAPHost) || (!acc.NoOutgoingServer && !safeDiagnosticHost(acc.SMTPHost)) {
		return nil, fmt.Errorf("review the saved server hostnames")
	}
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 35*time.Second)
	state := &a.setupDiagnostic
	state.mu.Lock()
	if state.cancel != nil {
		state.mu.Unlock()
		cancel()
		return nil, fmt.Errorf("a diagnostic is already running")
	}
	state.id, state.cancel, state.report = req.RunID, cancel, nil
	if state.cancelledID == req.RunID {
		cancel()
	}
	state.mu.Unlock()
	defer func() { cancel(); state.mu.Lock(); state.cancel = nil; state.mu.Unlock() }()
	r := a.runSetupDiagnostic(ctx, acc, req)
	state.mu.Lock()
	state.report = r
	state.mu.Unlock()
	return r, nil
}
func (a *App) CancelSetupDiagnostic(runID string) {
	s := &a.setupDiagnostic
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancelledID = runID
	if s.id == runID && s.cancel != nil {
		s.cancel()
	}
}

// ExportSetupDiagnostic saves only after the user chooses a local file. Copy the
// redacted snapshot before showing the dialog; never hold the run mutex in UI.
func (a *App) ExportSetupDiagnostic(runID string) (bool, error) {
	return a.exportSetupDiagnostic(runID, func() (string, error) {
		if platform.IsFlatpak() {
			return platform.PortalSaveFile("Export redacted account setup report", "mailven-account-setup.json", "")
		}
		return wailsRuntime.SaveFileDialog(a.ctx, wailsRuntime.SaveDialogOptions{Title: "Export redacted account setup report", DefaultFilename: "mailven-account-setup.json", Filters: []wailsRuntime.FileFilter{{DisplayName: "JSON report", Pattern: "*.json"}}})
	})
}
func (a *App) exportSetupDiagnostic(runID string, choose func() (string, error)) (bool, error) {
	raw, err := a.redactedSetupDiagnostic(runID)
	if err != nil {
		return false, err
	}
	path, err := choose()
	if err != nil || path == "" {
		return false, err
	}
	if err = os.WriteFile(path, []byte(raw), 0600); err != nil {
		return false, fmt.Errorf("could not save diagnostic report")
	}
	return true, nil
}
func (a *App) redactedSetupDiagnostic(runID string) (string, error) {
	s := &a.setupDiagnostic
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.id != runID || s.report == nil || s.cancel != nil {
		return "", fmt.Errorf("wait for the diagnostic to finish")
	}
	// Positive projection: internal IDs, mailbox/From addresses, usernames and
	// credential structures are never included. Free-form provider errors are
	// discarded at the probe boundary rather than scrubbed after collection.
	r := *s.report
	r.RunID = ""
	r.AccountID = ""
	r.Mailbox = "selected mailbox (redacted)"
	r.From = "selected identity (redacted)"
	b, err := json.MarshalIndent(r, "", "  ")
	return string(b), err
}
func (a *App) runSetupDiagnostic(ctx context.Context, acc *account.Account, req SetupDiagnosticRequest) *SetupDiagnosticReport {
	r := &SetupDiagnosticReport{RunID: req.RunID, AccountID: acc.ID, Mailbox: acc.Email, From: req.From, Route: "imap-smtp", StartedAt: time.Now().UTC().Format(time.RFC3339Nano), OAuthScopes: []string{}, Endpoints: []string{}, Checks: []setupdiagnostic.Check{}, Limitations: "A passed probe records only the operation performed at the stated UTC time. Login never proves Send As, Send on Behalf, tenant policy, signing, SPF/DKIM/DMARC pass or alignment, delivery or inbox placement. DNS presence is not proof of signing or alignment. No test mail, mail reads, send API, SMTP MAIL/RCPT/DATA or DNS writes. No delivery score. Raw errors and request IDs are omitted for privacy. No automatic retry; wait at least 60 seconds after a timeout or temporary failure. Only the existing configured IMAP/SMTP route is supported."}
	im := setupdiagnostic.Config{Protocol: "imap", Host: acc.IMAPHost, Port: acc.IMAPPort, Security: string(acc.IMAPSecurity), Username: acc.Username, Mechanism: string(acc.IMAPAuthMechanism)}
	sc, err := a.composeOps.smtpRouteConfig(acc)
	sm := setupdiagnostic.Config{Protocol: "smtp", Host: acc.SMTPHost, Port: acc.SMTPPort, Security: string(acc.SMTPSecurity), Username: sc.Username, Mechanism: sc.AuthMechanism}
	if err != nil {
		sm.AuthSkip = "Authentication owner unavailable. Review the linked account."
	}
	var grant *credentials.OAuthTokens
	if acc.AuthType == account.AuthOAuth2 {
		im.OAuth, sm.OAuth = true, true
		// Read the shared owner's secure slot, but do not initiate refresh or consent.
		// Existing refresh locks can block without a caller deadline. Keeping this
		// diagnostic read-only makes start/cancel bounded and avoids reauth events.
		owner, ownerErr := a.credStore.OAuthTokenOwnerID(acc.ID)
		tokens, tokenErr := a.credStore.GetOAuthTokens(owner)
		if ownerErr != nil {
			tokenErr = ownerErr
		}
		skip := ""
		if tokenErr != nil || tokens == nil {
			skip = "Stored OAuth authorization unavailable. Use the account's explicit Re-authorize action; no consent was started."
		} else {
			grant = tokens
			recordDiagnosticGrant(r, tokens)
			if tokens.AccessToken == "" {
				skip = "Stored access token unavailable. Use the account’s explicit Re-authorize action, then retry."
			}
			if tokens.IsExpiringSoon(35 * time.Second) {
				skip = "Stored token expires before this probe can finish. Refresh through the normal account connection or explicitly reauthorize, then retry. No token refresh or consent was started."
			}
		}
		for _, c := range []*setupdiagnostic.Config{&im, &sm} {
			c.AuthSkip = skip
			if tokens != nil && skip == "" {
				if e := tokens.RequireMailCapability(c.Protocol); e != nil {
					c.AuthSkip = "Actual OAuth grant lacks this protocol permission. Explicitly reauthorize the account to grant it; no consent was started."
				} else {
					c.Token = tokens.AccessToken
				}
			}
		}
	} else {
		im.Password, err = a.credStore.GetPassword(acc.ID)
		if err != nil {
			im.AuthSkip = "Saved IMAP credential unavailable. Review account credentials."
		}
		if acc.SMTPUsername != "" {
			sm.Password, err = a.credStore.GetSMTPPassword(acc.ID)
		} else {
			sm.Password, err = a.credStore.GetPassword(acc.ID)
		}
		if err != nil {
			sm.AuthSkip = "Saved SMTP credential unavailable. Review account credentials."
		}
	}
	for _, c := range []setupdiagnostic.Config{im, sm} {
		if c.Protocol == "smtp" && acc.NoOutgoingServer {
			r.Checks = append(r.Checks, setupdiagnostic.NewCheck("smtp transport", "skipped", "No outgoing server configured."))
			continue
		}
		r.Endpoints = append(r.Endpoints, fmt.Sprintf("%s %s (%s)", c.Protocol, net.JoinHostPort(c.Host, fmt.Sprint(c.Port)), c.Security))
		r.Checks = append(r.Checks, setupdiagnostic.Probe(ctx, c)...)
		permission := setupdiagnostic.NewCheck(c.Protocol+" permission", "unknown", "Protocol permission and administrator policy are not established by login. Confirm restrictions with your provider/administrator.")
		if c.AuthSkip != "" {
			permission.Detail = c.AuthSkip
		}
		if grant != nil && grant.ScopesKnown && (grant.Provider == "google" || grant.Provider == "microsoft") {
			permission.Name = c.Protocol + " OAuth protocol scope permission"
			if grant.RequireMailCapability(c.Protocol) != nil {
				permission.Status, permission.Detail = "failed", "Actual recorded grant lacks this protocol scope. Use explicit account reauthorization to request the missing permission. No consent was started."
			} else {
				permission.Status, permission.Detail = "passed", "Actual recorded grant includes this protocol scope. This is grant metadata only; administrator policy, delegation and current server access remain independently enforced."
			}
		}
		r.Checks = append(r.Checks, permission)
	}
	e, eErr := a.composeOps.senderEvidenceForAccount(acc, req.From)
	check := setupdiagnostic.NewCheck("cached sender permission", "unknown", "Cached sender evidence unavailable. Confirm this From with the provider; login never establishes Send As.")
	if eErr == nil && e != nil {
		check.Detail = e.Source + ": " + e.State + " (last successful state: " + e.ObservedState + "). " + e.Guidance
		if e.Action == "block" {
			check.Status = "failed"
		}
		if t, parseErr := time.Parse(time.RFC3339Nano, e.ObservedAt); parseErr == nil {
			check.At = t.UTC().Format(time.RFC3339Nano)
			check.EvidenceAgeSeconds = max(0, int64(time.Since(t).Seconds()))
		} else {
			check.EvidenceAgeSeconds = -1
		}
	}
	r.Checks = append(r.Checks, check, setupdiagnostic.NewCheck("sending policy", "unknown", "No sending operation was attempted. Send As/Send on Behalf, protocol restrictions and provider/tenant sending policy need separate administrator evidence. Send-only API consent cannot be proven using mail reads."))
	r.DNS = setupdiagnostic.RunDNS(ctx, setupdiagnostic.DNSRequest{Domain: req.Domain, Owned: req.OwnedDomain, Selectors: req.DKIMSelectors})
	r.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return r
}
func recordDiagnosticGrant(r *SetupDiagnosticReport, t *credentials.OAuthTokens) {
	r.OAuthExpiresAt = t.ExpiresAt.UTC().Format(time.RFC3339Nano)
	r.OAuthScopesKnown = t.ScopesKnown
	// Only public, recognized scope namespaces are exportable. Opaque custom
	// grants/resource URLs may contain secrets and are intentionally redacted.
	safe := func(s string) string {
		if s == "openid" || s == "profile" || s == "email" || s == "offline_access" {
			return s
		}
		for _, prefix := range []string{"https://mail.google.com/", "https://www.googleapis.com/auth/", "https://outlook.office.com", "https://graph.microsoft.com"} {
			if (s == prefix || strings.HasPrefix(s, prefix+"/") || strings.HasSuffix(prefix, "/") && strings.HasPrefix(s, prefix)) && diagnosticScope.MatchString(s) {
				return s
			}
		}
		return "[custom metadata redacted]"
	}
	r.OAuthResource = safe(t.ScopeResource)
	if t.ScopesKnown {
		for _, scope := range t.EffectiveScopes() {
			r.OAuthScopes = append(r.OAuthScopes, safe(scope))
		}
	}
	r.Checks = append(r.Checks, setupdiagnostic.NewCheck("OAuth grant metadata", "unknown", "Scopes are actual recorded grants when known; requested scopes are never treated as consent. Expiry is local token metadata, not a policy or sending-rights check. Opaque custom scope/resource metadata is redacted. No refresh or consent is started by this diagnostic."))
}
