package app

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/smtp"
	"github.com/caseylabs/mailven/internal/submission"
)

func (ops *composeOps) sentRoute(accountID string) (string, error) {
	acc, err := ops.accountStore.Get(accountID)
	if err != nil {
		return "", err
	}
	if acc == nil {
		return "", fmt.Errorf("account not found")
	}
	config, err := ops.smtpRouteConfig(acc)
	if err != nil {
		return "", err
	}
	return sentRouteSnapshot(acc, config), nil
}

// smtpRouteConfig is shared by policy review and actual submission.
func (ops *composeOps) smtpRouteConfig(acc *account.Account) (smtp.ClientConfig, error) {
	config := smtp.DefaultConfig()
	config.Host, config.Port = acc.SMTPHost, acc.SMTPPort
	config.Security = smtp.SecurityType(acc.SMTPSecurity)
	config.AuthMechanism = string(acc.EffectiveSMTPAuthMechanism())
	config.Username = acc.Username
	if acc.SharedMailboxParentID != "" {
		parent, err := ops.accountStore.Get(acc.SharedMailboxParentID)
		if err != nil {
			return config, err
		}
		if parent == nil {
			return config, fmt.Errorf("linked authentication owner not found")
		}
		config.Username = parent.Username
	}
	if acc.AuthType != account.AuthOAuth2 && acc.SMTPUsername != "" {
		config.Username = acc.SMTPUsername
	}
	return config, nil
}
func sentRouteSnapshot(acc *account.Account, config smtp.ClientConfig) string {
	owner := acc.ID
	if acc.SharedMailboxParentID != "" {
		owner = acc.SharedMailboxParentID
	}
	// Never include tokens/passwords. This is the configuration actually used by
	// SMTP, not an independent settings read that could race with account edits.
	b, _ := json.Marshal([]any{"smtp", config.Host, config.Port, config.Security, config.AuthMechanism, acc.AuthType, owner, config.Username, acc.ID, acc.IMAPHost, acc.IMAPPort, acc.IMAPSecurity, acc.Username, acc.SentFolderPath})
	return string(b)
}

func (ops *composeOps) sentMailboxRoute(accountID string) (string, error) {
	acc, err := ops.accountStore.Get(accountID)
	if err != nil {
		return "", err
	}
	if acc == nil {
		return "", fmt.Errorf("destination account not found")
	}
	return sentMailboxRouteSnapshot(acc), nil
}
func sentMailboxRouteSnapshot(acc *account.Account) string {
	b, _ := json.Marshal([]any{acc.ID, acc.IMAPHost, acc.IMAPPort, acc.IMAPSecurity, acc.IMAPAuthMechanism, acc.AuthType, acc.Username, acc.SharedMailboxParentID})
	return string(b)
}
func (ops *composeOps) currentSentPolicy(accountID, route string) (*submission.SentPolicy, error) {
	p, err := ops.submissions.SentPolicy(accountID, route)
	if err != nil {
		return nil, err
	}
	for i, d := range p.Destinations {
		target, err := ops.sentMailboxRoute(d.AccountID)
		if err != nil || target != d.MailboxRoute {
			p.Destinations[i].Mode = "unresolved"
		}
	}
	return p, nil
}
func (a *App) GetSentPolicy(accountID string) (*submission.SentPolicy, error) {
	route, err := a.composeOps.sentRoute(accountID)
	if err != nil {
		return nil, err
	}
	return a.composeOps.currentSentPolicy(accountID, route)
}
func (a *App) SetSentPolicy(accountID string, p submission.SentPolicy) error {
	route, err := a.composeOps.sentRoute(accountID)
	if err != nil {
		return err
	}
	if p.Route != route {
		return fmt.Errorf("account route changed; reload and review Sent policy")
	}
	for i, d := range p.Destinations {
		f, err := a.folderStore.GetByPath(d.AccountID, d.FolderPath)
		if err != nil {
			return err
		}
		if f == nil {
			return fmt.Errorf("Sent destination folder not found")
		}
		target, err := a.composeOps.sentMailboxRoute(d.AccountID)
		if err != nil {
			return err
		}
		p.Destinations[i].MailboxRoute = target
	}
	return a.composeOps.submissions.SetSentPolicy(accountID, p)
}
func (a *App) GetSubmissionCopies(accountID string) ([]submission.Copy, error) {
	return a.composeOps.submissions.ReconcileCopies(accountID)
}
func (ops *composeOps) planSentCopies(attempt *submission.Attempt, acc *account.Account, config smtp.ClientConfig) error {
	route := sentRouteSnapshot(acc, config)
	p, err := ops.currentSentPolicy(attempt.AccountID, route)
	if err != nil {
		return err
	}
	return ops.submissions.PlanCopies(attempt, *p)
}
func (ops *composeOps) saveSentCopies(ctx context.Context, a *submission.Attempt, raw []byte) {
	copies, err := ops.submissions.ReconcileCopies(a.AccountID)
	if err != nil {
		a.Warnings += "Sent copy reconciliation failed: " + err.Error() + "\n"
		return
	}
	for _, c := range copies {
		if c.AttemptID != a.ID || c.State == "observed" {
			continue
		}
		if c.Mode == "append" {
			claimed, err := ops.submissions.ClaimAppend(c)
			if err == nil && claimed {
				err = ops.appendSentCopy(ctx, c, raw)
				if err == nil {
					err = ops.submissions.SetCopyState(c, "appended", "IMAP APPEND acknowledged; sync discovery pending")
				}
			}
			if err != nil {
				a.Warnings += "Sent APPEND unconfirmed; do not repeat automatically: " + err.Error() + "\n"
			}
			continue
		}
		a.Warnings += "Sent copy unresolved: " + c.AccountID + " / " + c.FolderPath + " (" + c.Mode + "); sync and review copy evidence. No automatic APPEND.\n"
	}
}
