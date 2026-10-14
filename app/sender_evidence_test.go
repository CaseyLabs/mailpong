package app

import (
	"github.com/caseylabs/mailven/internal/credentials"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/folder"
	"github.com/caseylabs/mailven/internal/gmailapi"
	"github.com/caseylabs/mailven/internal/message"
	"github.com/caseylabs/mailven/internal/smtp"
)

func TestReadReceiptRequiresSenderEvidence(t *testing.T) {
	for _, state := range []string{"unknown", "pending"} {
		t.Run(state, func(t *testing.T) {
			f, server, _ := initAppSendFixture(t, appSMTPConfig{finalReply: "250 queued"})
			acc, err := f.ops.accountStore.Get(f.accountID)
			if err != nil {
				t.Fatal(err)
			}
			// Retain the disposable SMTP endpoint. A different primary From needs
			// review; Gmail's fresh pending observation must block even that review.
			cfg := account.AccountConfig{Name: acc.Name, DisplayName: "Sender", Email: acc.Email, Username: acc.Username,
				IMAPHost: acc.IMAPHost, SMTPHost: acc.SMTPHost, SMTPPort: acc.SMTPPort, SMTPSecurity: acc.SMTPSecurity, AuthType: acc.AuthType}
			want := "sender review required"
			if state == "pending" {
				cfg.AuthType, cfg.SMTPHost = account.AuthOAuth2, "smtp.gmail.com"
				want = "sender blocked"
				f.ops.sendAsStore = gmailapi.NewStore(f.db.DB)
				if err := f.ops.sendAsStore.Replace(acc.ID, []gmailapi.SendAs{{Email: acc.Email, VerificationStatus: "pending"}}, time.Now()); err != nil {
					t.Fatal(err)
				}
			} else {
				cfg.Email = "different@example.test"
			}
			if _, err := f.ops.accountStore.Update(acc.ID, &cfg); err != nil {
				t.Fatal(err)
			}
			if err := f.ops.folderStore.Create(&folder.Folder{ID: "inbox", AccountID: f.accountID, Name: "Inbox", Path: "INBOX", Type: folder.TypeInbox}); err != nil {
				t.Fatal(err)
			}
			messages := message.NewStore(f.db)
			if err := messages.Create(&message.Message{ID: "receipt-original", AccountID: f.accountID, FolderID: "inbox", UID: 1,
				MessageID: "<original@example.test>", FromEmail: "recipient@example.test", ReadReceiptTo: "recipient@example.test", Date: time.Now()}); err != nil {
				t.Fatal(err)
			}
			a := &App{accountStore: f.ops.accountStore, credStore: f.ops.credStore, messageStore: messages, composeOps: *f.ops}
			if err := a.SendReadReceipt(f.accountID, "receipt-original"); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("receipt bypassed sender gate: %v", err)
			}
			select {
			case <-server.connections:
				t.Fatal("unreviewed receipt connected to SMTP")
			default:
			}
			original, err := messages.Get("receipt-original")
			if err != nil || original == nil || original.ReadReceiptHandled {
				t.Fatalf("blocked receipt was marked handled: %+v, %v", original, err)
			}
		})
	}
}

func TestSenderEvidenceGmail(t *testing.T) {
	for _, tt := range []struct {
		name, email, verification, state, action string
		primary                                  bool
		age                                      time.Duration
		fetched                                  bool
	}{
		{"primary", "me@gmail.com", "", "verified", "allow", true, 0, true},
		{"accepted", "alias@example.com", "accepted", "verified", "allow", false, 0, true},
		{"pending", "alias@example.com", "pending", "pending", "block", false, 0, true},
		{"blank", "alias@example.com", "", "unknown", "review", false, 0, true},
		{"missing", "missing@example.com", "accepted", "not-configured", "block", false, 0, true},
		{"denied or never fetched", "alias@example.com", "", "unknown", "review", false, 0, false},
		{"stale missing", "missing@example.com", "accepted", "stale", "block", false, 25 * time.Hour, true},
		{"stale accepted", "alias@example.com", "accepted", "stale", "review", false, 25 * time.Hour, true},
		{"stale pending", "alias@example.com", "pending", "stale", "block", false, 25 * time.Hour, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ops, acc := newSendAsOps(t, "imap.gmail.com", account.AuthOAuth2)
			if _, err := ops.accountStore.Update(acc.ID, &account.AccountConfig{Name: acc.Name, DisplayName: "Me", Email: acc.Email, IMAPHost: acc.IMAPHost, SMTPHost: "smtp.gmail.com", SMTPPort: 587, AuthType: acc.AuthType, Username: acc.Username}); err != nil {
				t.Fatal(err)
			}
			if tt.fetched {
				if err := ops.sendAsStore.Replace(acc.ID, []gmailapi.SendAs{{Email: "me@gmail.com", IsPrimary: true}, {Email: "alias@example.com", VerificationStatus: tt.verification, IsPrimary: tt.primary}}, time.Now().Add(-tt.age)); err != nil {
					t.Fatal(err)
				}
			}
			e, err := ops.senderEvidence(acc.ID, tt.email)
			if err != nil {
				t.Fatal(err)
			}
			if e.State != tt.state || e.Action != tt.action || e.Source != "Gmail send-as settings" || !strings.Contains(e.Route, "smtp.gmail.com:587") || (e.ObservedAt != "") != tt.fetched {
				t.Fatalf("evidence = %+v", e)
			}
			msg := smtp.ComposeMessage{From: smtp.Address{Address: tt.email}, Bcc: []smtp.Address{{Address: "hidden@example.com"}}}
			err = ops.requireSenderEvidence(acc.ID, msg)
			if (err == nil) != (tt.action == "allow") {
				t.Fatalf("gate = %v", err)
			}
			msg.SenderReviewToken = e.ReviewToken
			err = ops.requireSenderEvidence(acc.ID, msg)
			if (err == nil) != (tt.action != "block") {
				t.Fatalf("reviewed gate = %v", err)
			}
			if msg.From.Address != tt.email || msg.Bcc[0].Address != "hidden@example.com" {
				t.Fatal("gate changed message")
			}
			if tt.action == "review" {
				msg.From.Address = "other@example.com"
				if err := ops.requireSenderEvidence(acc.ID, msg); err == nil {
					t.Fatal("review acknowledged a different From")
				}
			}
		})
	}
}

func TestSenderEvidenceMicrosoftAndCustomRoute(t *testing.T) {
	ops, acc := newSendAsOps(t, "outlook.office365.com", account.AuthOAuth2)
	e, err := ops.senderEvidence(acc.ID, "shared@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if e.SendAs != "unknown" || e.SendOnBehalf != "unknown" || e.SharedRead != "unknown" || e.Action != "review" {
		t.Fatalf("login inferred rights: %+v", e)
	}
	// Shared read configuration provides no sending grant or exhaustive list.
	child, err := ops.accountStore.Create(&account.AccountConfig{Name: "Shared", DisplayName: "Shared", Email: "shared@example.com", IMAPHost: "outlook.office365.com", SMTPHost: "smtp.office365.com", SMTPPort: 587, AuthType: account.AuthOAuth2, Username: "shared@example.com", SharedMailboxParentID: acc.ID})
	if err != nil {
		t.Fatal(err)
	}
	e, err = ops.senderEvidence(child.ID, child.Email)
	if err != nil {
		t.Fatal(err)
	}
	if e.SharedRead != "configured" || e.SendAs != "unknown" || e.SendOnBehalf != "unknown" || e.Action != "review" || !strings.Contains(e.Guidance, "needs Send As") {
		t.Fatalf("read implied delegation: %+v", e)
	}
	msg := smtp.ComposeMessage{From: smtp.Address{Address: child.Email}, SenderReviewToken: e.ReviewToken}
	if err := ops.requireSenderEvidence(child.ID, msg); err != nil {
		t.Fatal(err)
	}
	// Review for the parent cannot acknowledge the child route.
	if err := ops.requireSenderEvidence(acc.ID, msg); err == nil {
		t.Fatal("review crossed accounts")
	}
	ops, acc = newSendAsOps(t, "imap.gmail.com", account.AuthOAuth2)
	if err := ops.sendAsStore.Replace(acc.ID, []gmailapi.SendAs{{Email: "alias@example.com", VerificationStatus: "accepted"}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	e, err = ops.senderEvidence(acc.ID, "alias@example.com")
	if err != nil || e.State != "unknown" || e.Action != "review" {
		t.Fatalf("Gmail settings authorized custom SMTP: %+v, %v", e, err)
	}
}

func TestSenderGateBeforeTransportPreservesDraft(t *testing.T) {
	f, server, _ := initAppSendFixture(t, appSMTPConfig{})
	msg := appSendMessage()
	msg.From.Address = "custom@example.test"
	_, err := f.ops.sendMessage(t.Context(), f.accountID, msg, f.draft)
	if err == nil || !strings.Contains(err.Error(), "sender review required") {
		t.Fatalf("gate: %v", err)
	}
	select {
	case <-server.connections:
		t.Fatal("unreviewed sender connected to SMTP")
	default:
	}
	d, err := f.draftStore.Get(f.draft.ID)
	if err != nil || d == nil || d.Subject != "durable outcomes" {
		t.Fatalf("draft lost: %+v, %v", d, err)
	}
}

func TestSenderEvidenceFailedLookupAndChangedRoute(t *testing.T) {
	ops, acc := newSendAsOps(t, "imap.gmail.com", account.AuthOAuth2)
	cfg := account.AccountConfig{Name: acc.Name, DisplayName: "Me", Email: acc.Email, IMAPHost: acc.IMAPHost, SMTPHost: "smtp.gmail.com", SMTPPort: 587, AuthType: acc.AuthType, Username: acc.Username}
	if _, err := ops.accountStore.Update(acc.ID, &cfg); err != nil {
		t.Fatal(err)
	}
	observed := time.Now().Add(-time.Hour)
	if err := ops.sendAsStore.Replace(acc.ID, []gmailapi.SendAs{{Email: "alias@example.com", VerificationStatus: "accepted"}}, observed); err != nil {
		t.Fatal(err)
	}
	if err := ops.sendAsStore.MarkLookupFailed(acc.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	e, err := ops.senderEvidence(acc.ID, "alias@example.com")
	if err != nil || e.State != "unknown" || e.Action != "review" || e.ObservedAt == "" || !strings.Contains(e.Source, "failed") {
		t.Fatalf("failed lookup became success: %+v, %v", e, err)
	}
	msg := smtp.ComposeMessage{From: smtp.Address{Address: e.Email}, SenderReviewToken: e.ReviewToken}
	if err := ops.requireSenderEvidence(acc.ID, msg); err != nil {
		t.Fatal(err)
	}
	cfg.SMTPHost = "smtp.custom.example"
	if _, err := ops.accountStore.Update(acc.ID, &cfg); err != nil {
		t.Fatal(err)
	}
	if err := ops.requireSenderEvidence(acc.ID, msg); err == nil {
		t.Fatal("review survived route change")
	}
	cfg.SMTPHost = "smtp.gmail.com"
	if _, err := ops.accountStore.Update(acc.ID, &cfg); err != nil {
		t.Fatal(err)
	}
	if err := ops.sendAsStore.Replace(acc.ID, []gmailapi.SendAs{{Email: e.Email, VerificationStatus: "pending"}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := ops.requireSenderEvidence(acc.ID, msg); err == nil || !strings.Contains(err.Error(), "sender blocked") {
		t.Fatalf("review bypassed refreshed known-invalid evidence: %v", err)
	}
	if err := ops.sendAsStore.MarkLookupFailed(acc.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	e, err = ops.senderEvidence(acc.ID, msg.From.Address)
	if err != nil || e.State != "unknown" || e.ObservedState != "pending" || e.Action != "block" {
		t.Fatalf("lookup failure bypassed last negative evidence: %+v, %v", e, err)
	}
	msg.SenderReviewToken = "forced-review"
	if err := ops.requireSenderEvidence(acc.ID, msg); err == nil {
		t.Fatal("review bypassed negative evidence after lookup failure")
	}
}

type senderRoundTripper func(*http.Request) (*http.Response, error)

func (f senderRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSenderEvidenceRefreshDeniedAndPartialGrant(t *testing.T) {
	f, _, _ := initAppSendFixture(t, appSMTPConfig{})
	acc, err := f.ops.accountStore.Get(f.accountID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.ops.accountStore.Update(acc.ID, &account.AccountConfig{Name: acc.Name, DisplayName: "Me", Email: acc.Email, Username: acc.Username, IMAPHost: "imap.gmail.com", SMTPHost: "smtp.gmail.com", SMTPPort: 587, AuthType: account.AuthOAuth2}); err != nil {
		t.Fatal(err)
	}
	f.ops.sendAsStore = gmailapi.NewStore(f.db.DB)
	if err := f.ops.sendAsStore.Replace(acc.ID, []gmailapi.SendAs{{Email: acc.Email, IsPrimary: true}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	tokens := &credentials.OAuthTokens{Provider: "google", AccessToken: "mock-token", RefreshToken: "mock-refresh", ExpiresAt: time.Now().Add(time.Hour), ScopesKnown: true, Scopes: []string{"openid"}}
	if err := f.ops.credStore.SetOAuthTokens(acc.ID, tokens); err != nil {
		t.Fatal(err)
	}
	original := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = original })
	http.DefaultClient = &http.Client{Transport: senderRoundTripper(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "gmail.googleapis.com" || r.Header.Get("Authorization") != "Bearer mock-token" {
			t.Fatalf("unexpected request: %s", r.URL)
		}
		return &http.Response{StatusCode: 403, Body: io.NopCloser(strings.NewReader(`{"error":{"errors":[{"reason":"insufficientPermissions"}]}}`))}, nil
	})}
	if err := f.ops.refreshSendAs(t.Context(), acc.ID); err == nil || !strings.Contains(err.Error(), "Gmail denied access") {
		t.Fatalf("lookup did not exercise denial: %v", err)
	}
	e, err := f.ops.senderEvidence(acc.ID, acc.Email)
	if err != nil || e.State != "unknown" || e.Action != "review" {
		t.Fatalf("denied lookup: %+v, %v", e, err)
	}
	msg := appSendMessage()
	msg.SenderReviewToken = e.ReviewToken
	// Reviewing identity evidence never grants the missing SMTP OAuth scope.
	_, err = f.ops.sendMessage(t.Context(), acc.ID, msg, f.draft)
	if err == nil || !strings.Contains(err.Error(), "authorization does not include smtp") {
		t.Fatalf("partial scope bypassed: %v", err)
	}
}

func TestSenderEvidencePreservesIDNAFrom(t *testing.T) {
	ops, acc := newSendAsOps(t, "imap.gmail.com", account.AuthOAuth2)
	if _, err := ops.accountStore.Update(acc.ID, &account.AccountConfig{Name: acc.Name, DisplayName: "Me", Email: acc.Email, IMAPHost: acc.IMAPHost, SMTPHost: "smtp.gmail.com", SMTPPort: 587, AuthType: acc.AuthType, Username: acc.Username}); err != nil {
		t.Fatal(err)
	}
	if err := ops.sendAsStore.Replace(acc.ID, []gmailapi.SendAs{{Email: "alias@xn--bcher-kva.example", VerificationStatus: "accepted"}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	msg := smtp.ComposeMessage{From: smtp.Address{Address: "alias@bücher.example"}}
	e, err := ops.senderEvidence(acc.ID, msg.From.Address)
	if err != nil || e.State != "verified" || e.Email != msg.From.Address {
		t.Fatalf("equivalent IDN alias lost: %+v, %v", e, err)
	}
	if err := ops.requireSenderEvidence(acc.ID, msg); err != nil {
		t.Fatal(err)
	}
	if msg.From.Address != "alias@bücher.example" {
		t.Fatal("selected From changed")
	}
}
