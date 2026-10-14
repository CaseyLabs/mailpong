package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/gmailapi"
	"github.com/caseylabs/mailven/internal/smtp"
)

// SenderEvidence describes setup evidence for one exact SMTP route, not domain
// authentication, mailbox enumeration, delivery, or an enduring permission grant.
type SenderEvidence struct {
	Email         string `json:"email"`
	State         string `json:"state"`
	ObservedState string `json:"observedState"`
	Source        string `json:"source"`
	Route         string `json:"route"`
	ObservedAt    string `json:"observedAt"`
	Action        string `json:"action"` // allow, review, block
	Guidance      string `json:"guidance"`
	ReviewToken   string `json:"reviewToken"`
	// Microsoft permissions are independent. Configured shared read access is
	// never evidence of Send As or Send on Behalf, nor proof of current read rights.
	SendAs       string `json:"sendAs"`
	SendOnBehalf string `json:"sendOnBehalf"`
	SharedRead   string `json:"sharedRead"`
}

func (ops *composeOps) senderEvidence(accountID, email string) (*SenderEvidence, error) {
	acc, err := ops.accountStore.Get(accountID)
	if err != nil {
		return nil, err
	}
	if acc == nil {
		return nil, fmt.Errorf("account not found")
	}
	return ops.senderEvidenceForAccount(acc, email)
}

func (ops *composeOps) senderEvidenceForAccount(acc *account.Account, email string) (*SenderEvidence, error) {
	e := &SenderEvidence{Email: email, State: "unknown", ObservedState: "unknown", Source: "local account configuration", Route: fmt.Sprintf("smtp://%s:%d (%s)", acc.SMTPHost, acc.SMTPPort, acc.SMTPSecurity), Action: "review", SendAs: "unknown", SendOnBehalf: "unknown", SharedRead: "unknown"}
	e.Guidance = "Sender authorization is unknown for this route. Confirm this From address with your provider or administrator. Review before sending; the provider enforces permission."
	if acc.SharedMailboxParentID != "" {
		e.SharedRead = "configured"
		e.Source = "local shared mailbox configuration"
		e.Guidance = "Shared mailbox read access does not grant Send As or Send on Behalf. This SMTP route submits as the selected From and needs Send As. Confirm delegation with your administrator; sending rights are unknown."
	}
	microsoft := acc.IMAPHost == "outlook.office365.com" || acc.IMAPHost == "imap-mail.outlook.com" || acc.SMTPHost == "smtp.office365.com" || acc.SMTPHost == "smtp-mail.outlook.com"
	if microsoft {
		e.Source = "Microsoft delegation lookup unavailable; local configuration"
		if acc.SharedMailboxParentID == "" {
			e.Guidance = "Microsoft Send As and Send on Behalf rights are unknown. Login and OAuth protocol scopes do not enumerate sendable mailboxes. Confirm this From address; the provider enforces permission."
		}
	} else if isGoogleOAuth(acc) && strings.EqualFold(acc.SMTPHost, "smtp.gmail.com") {
		e.Source = "Gmail send-as settings"
		if ops.sendAsStore == nil {
			return nil, fmt.Errorf("sender evidence store unavailable")
		}
		cached, err := ops.sendAsStore.Get(acc.ID)
		if err != nil {
			return nil, fmt.Errorf("cannot read sender evidence: %w", err)
		}
		e.ObservedState = cached.StatusFor(email)
		e.State = e.ObservedState
		if !cached.FetchedAt.IsZero() {
			e.ObservedAt = cached.FetchedAt.UTC().Format(time.RFC3339Nano)
			if !freshSendAs(cached) {
				e.State = "stale"
			}
		}
		if cached.LookupFailed {
			e.State = "unknown"
			e.Source = "Gmail cached send-as settings; latest lookup failed"
		}
		switch e.State {
		case gmailapi.StatusVerified:
			e.Action = "allow"
			e.Guidance = "Gmail reports this sender configured and verified. This does not prove domain alignment, delivery, or inbox placement."
		case gmailapi.StatusPending, gmailapi.StatusNotConfigured:
			e.Action = "block"
			e.Guidance = "Gmail reports this sender pending verification or absent from its send-as settings. Verify/add this address in Gmail, refresh account sender evidence, or choose another From. The selected From is preserved."
		case "stale":
			e.Guidance = "Gmail sender evidence is older than 24 hours. Refresh it in account identities, or explicitly review this From before sending. Old evidence does not prove current permission."
		default:
			e.Guidance = "Gmail sender authorization is unknown. Refresh sender evidence in account identities; check OAuth permission and Gmail API access if lookup fails. Review before sending."
		}
		if e.Action == "review" && (e.ObservedState == gmailapi.StatusPending || e.ObservedState == gmailapi.StatusNotConfigured) {
			e.Action = "block"
			e.Guidance = "The last successful Gmail observation found this sender pending verification or absent. Evidence is now stale or lookup failed. Verify/add this address and refresh account sender evidence before sending, or choose another From. The selected From is preserved."
		}
	} else if !microsoft && acc.SharedMailboxParentID == "" && acc.AuthType != account.AuthOAuth2 && strings.EqualFold(email, acc.Email) {
		e.State, e.ObservedState, e.Action = "configured", "configured", "allow"
		e.Guidance = "This is the locally configured account address. Provider authorization and domain alignment are unverified; the server enforces permission."
	}
	// Review is tied to the account, exact From, SMTP route/auth configuration,
	// and observation. It is an acknowledgement, never a provider credential.
	if e.Action == "review" {
		b, _ := json.Marshal(struct {
			AccountID    string
			Evidence     *SenderEvidence
			Username     string
			SMTPUsername string
			Parent       string
			Auth         account.AuthType
		}{acc.ID, e, acc.Username, acc.SMTPUsername, acc.SharedMailboxParentID, acc.AuthType})
		sum := sha256.Sum256(b)
		e.ReviewToken = hex.EncodeToString(sum[:])
	}
	return e, nil
}

func (ops *composeOps) requireSenderEvidence(accountID string, msg smtp.ComposeMessage) error {
	e, err := ops.senderEvidence(accountID, msg.From.Address)
	if err != nil {
		return err
	}
	return checkSenderEvidence(e, msg)
}

func checkSenderEvidence(e *SenderEvidence, msg smtp.ComposeMessage) error {
	switch e.Action {
	case "block":
		return fmt.Errorf("sender blocked: %s", e.Guidance)
	case "review":
		if msg.SenderReviewToken == "" || msg.SenderReviewToken != e.ReviewToken {
			return fmt.Errorf("sender review required: %s Open the composer and review the selected From before sending", e.Guidance)
		}
	}
	return nil
}

// GetSenderEvidence reads cached/local evidence without contacting providers.
func (a *App) GetSenderEvidence(accountID, email string) (*SenderEvidence, error) {
	return a.composeOps.senderEvidence(accountID, email)
}
func (c *ComposerApp) GetSenderEvidence(accountID, email string) (*SenderEvidence, error) {
	return c.composeOps.senderEvidence(accountID, email)
}
