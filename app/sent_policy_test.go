package app

import (
	"context"
	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/folder"
	"github.com/caseylabs/mailven/internal/submission"
	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"
	"net"
	"strings"
	"testing"
)

func TestSentPoliciesUseActualRouteAndAcceptedCopyEvidence(t *testing.T) {
	for _, mode := range []string{"unreviewed", "provider", "append", "unresolved"} {
		t.Run(mode, func(t *testing.T) {
			f, server, _ := initAppSendFixture(t, appSMTPConfig{finalReply: "250 queued"})
			// The receive provider is Gmail, while actual submission is disposable SMTP.
			// APPEND failure is forced locally, never by contacting the real provider.
			if _, err := f.db.Exec(`UPDATE accounts SET imap_host='localhost',imap_port=1,imap_security='none' WHERE id=?`, f.accountID); err != nil {
				t.Fatal(err)
			}
			if err := f.ops.folderStore.Create(&folder.Folder{ID: "custom-sent", AccountID: f.accountID, Name: "Custom", Path: "Custom/Sent", Type: folder.TypeSent}); err != nil {
				t.Fatal(err)
			}
			a := &App{composeOps: *f.ops, folderStore: f.ops.folderStore}
			p, err := a.GetSentPolicy(f.accountID)
			if err != nil {
				t.Fatal(err)
			}
			if mode != "unreviewed" {
				p.Destinations = []submission.Destination{{AccountID: f.accountID, FolderPath: "Custom/Sent", Mode: mode}}
				if err := a.SetSentPolicy(f.accountID, *p); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := f.ops.sendMessage(context.Background(), f.accountID, appSendMessage(), f.draft); err != nil {
				t.Fatalf("accepted send became failure: %v", err)
			}
			_ = readAppSMTP(t, server)
			attempt := getAttempt(t, f)
			if attempt.Outcome != "accepted" {
				t.Fatalf("lost acceptance: %+v", attempt)
			}
			copies, err := a.GetSubmissionCopies(f.accountID)
			if err != nil || len(copies) != 1 {
				t.Fatalf("copy evidence: %+v %v", copies, err)
			}
			if mode == "append" {
				if copies[0].State != "unknown" || !strings.Contains(attempt.Warnings, "APPEND unconfirmed") {
					t.Fatalf("failed append invited retry: %+v %+v", copies, attempt)
				}
			} else {
				if copies[0].State != "pending" || !strings.Contains(attempt.Warnings, "Sent copy unresolved") {
					t.Fatalf("provider copy assumed: %+v %+v", copies, attempt)
				}
			}
			if mode != "unreviewed" {
				if _, err := f.db.Exec(`UPDATE accounts SET smtp_username='different-auth' WHERE id=?`, f.accountID); err != nil {
					t.Fatal(err)
				}
				changed, err := a.GetSentPolicy(f.accountID)
				if err != nil || len(changed.Destinations) != 0 {
					t.Fatalf("old policy applied to changed auth: %+v %v", changed, err)
				}
				if err := a.SetSentPolicy(f.accountID, *p); err == nil {
					t.Fatal("stale UI route accepted")
				}
			}
		})
	}
}

func TestSentSnapshotMatchesSubmittedRouteDespiteSettingsEdit(t *testing.T) {
	f, _, _ := initAppSendFixture(t, appSMTPConfig{finalReply: "250 queued"})
	acc, err := f.ops.accountStore.Get(f.accountID)
	if err != nil {
		t.Fatal(err)
	}
	config, err := f.ops.smtpRouteConfig(acc)
	if err != nil {
		t.Fatal(err)
	}
	oldRoute := sentRouteSnapshot(acc, config)
	if err := f.ops.submissions.SetSentPolicy(acc.ID, submission.SentPolicy{Route: oldRoute, Destinations: []submission.Destination{{AccountID: acc.ID, FolderPath: "Old/Sent", Mode: "provider"}}}); err != nil {
		t.Fatal(err)
	}
	// Settings change after SMTP configuration has been captured.
	if _, err := f.db.Exec(`UPDATE accounts SET smtp_host='another.example.test' WHERE id=?`, acc.ID); err != nil {
		t.Fatal(err)
	}
	newRoute, err := f.ops.sentRoute(acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if newRoute == oldRoute {
		t.Fatal("settings edit did not change route")
	}
	if err := f.ops.submissions.SetSentPolicy(acc.ID, submission.SentPolicy{Route: newRoute, Destinations: []submission.Destination{{AccountID: acc.ID, FolderPath: "New/Sent", Mode: "provider"}}}); err != nil {
		t.Fatal(err)
	}
	attempt, err := f.ops.prepareSubmission(acc.ID, appSendMessage(), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.ops.planSentCopies(attempt, acc, config); err != nil {
		t.Fatal(err)
	}
	attempt.Outcome = "accepted"
	if err := f.ops.submissions.Update(attempt); err != nil {
		t.Fatal(err)
	}
	copies, err := f.ops.submissions.ReconcileCopies(acc.ID)
	if err != nil || len(copies) != 1 || copies[0].Mode != "unresolved" || copies[0].FolderPath != "" {
		t.Fatalf("new route's reviewed policy applied to old-route submission: %+v %v", copies, err)
	}
	if copies[0].Evidence != oldRoute {
		t.Fatalf("journal route differs from submitted route: %+v", copies[0])
	}
}

func TestReviewedClientAppendAcknowledgedAndNeverRepeated(t *testing.T) {
	f, smtpServer, _ := initAppSendFixture(t, appSMTPConfig{finalReply: "250 queued"})
	user := imapmemserver.NewUser("sender@example.test", "secret")
	if err := user.Create("Custom/Sent", nil); err != nil {
		t.Fatal(err)
	}
	memory := imapmemserver.New()
	memory.AddUser(user)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := imapserver.New(&imapserver.Options{InsecureAuth: true, NewSession: func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
		return memory.NewSession(), nil, nil
	}})
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close(); <-done })
	if _, err := f.db.Exec(`UPDATE accounts SET imap_host='localhost',imap_port=?,imap_security='none' WHERE id=?`, listener.Addr().(*net.TCPAddr).Port, f.accountID); err != nil {
		t.Fatal(err)
	}
	if err := f.ops.folderStore.Create(&folder.Folder{ID: "sent", AccountID: f.accountID, Name: "Custom", Path: "Custom/Sent", Type: folder.TypeSent}); err != nil {
		t.Fatal(err)
	}
	a := &App{composeOps: *f.ops, folderStore: f.ops.folderStore}
	p, err := a.GetSentPolicy(f.accountID)
	if err != nil {
		t.Fatal(err)
	}
	p.Destinations = []submission.Destination{{AccountID: f.accountID, FolderPath: "Custom/Sent", Mode: "append"}}
	if err := a.SetSentPolicy(f.accountID, *p); err != nil {
		t.Fatal(err)
	}
	if _, err := f.ops.sendMessage(context.Background(), f.accountID, appSendMessage(), nil); err != nil {
		t.Fatal(err)
	}
	raw := readAppSMTP(t, smtpServer)
	attempt := getAttempt(t, f)
	copies, err := a.GetSubmissionCopies(f.accountID)
	if err != nil || len(copies) != 1 || copies[0].State != "appended" {
		t.Fatalf("APPEND not acknowledged: %+v %v", copies, err)
	}
	// Even another cleanup pass with the raw message cannot replay APPEND.
	f.ops.saveSentCopies(context.Background(), attempt, raw)
	status, err := user.Status("Custom/Sent", &imap.StatusOptions{NumMessages: true})
	if err != nil {
		t.Fatal(err)
	}
	if status.NumMessages == nil || *status.NumMessages != 1 {
		t.Fatalf("APPEND duplicated or missing: %+v", status)
	}
}

func TestSentDestinationConnectionChangeRequiresReview(t *testing.T) {
	f, _, _ := initAppSendFixture(t, appSMTPConfig{finalReply: "250 queued"})
	target, err := f.ops.accountStore.Create(&account.AccountConfig{Name: "Custom", DisplayName: "Custom", Email: "copy@example.test", Username: "copy@example.test", IMAPHost: "old-target.example.test", SMTPHost: "smtp.example.test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.ops.folderStore.Create(&folder.Folder{ID: "target-sent", AccountID: target.ID, Name: "Custom", Path: "Custom/Sent", Type: folder.TypeSent}); err != nil {
		t.Fatal(err)
	}
	a := &App{composeOps: *f.ops, folderStore: f.ops.folderStore}
	p, err := a.GetSentPolicy(f.accountID)
	if err != nil {
		t.Fatal(err)
	}
	p.Destinations = []submission.Destination{{AccountID: target.ID, FolderPath: "Custom/Sent", Mode: "append"}}
	if err := a.SetSentPolicy(f.accountID, *p); err != nil {
		t.Fatal(err)
	}
	reviewed, err := a.GetSentPolicy(f.accountID)
	if err != nil {
		t.Fatal(err)
	}
	oldTarget := reviewed.Destinations[0].MailboxRoute
	if _, err := f.db.Exec(`UPDATE accounts SET imap_host='new-target.example.test' WHERE id=?`, target.ID); err != nil {
		t.Fatal(err)
	}
	changed, err := a.GetSentPolicy(f.accountID)
	if err != nil || changed.Route != reviewed.Route || changed.Destinations[0].Mode != "unresolved" {
		t.Fatalf("changed target inherited APPEND policy: %+v %v", changed, err)
	}
	copy := submission.Copy{AccountID: target.ID, FolderPath: "Custom/Sent", MailboxRoute: oldTarget}
	if err := f.ops.appendSentCopy(context.Background(), copy, nil); err == nil || !strings.Contains(err.Error(), "connection changed") {
		t.Fatalf("changed target not blocked before IMAP: %v", err)
	}
	changed.Destinations[0].Mode = "append"
	if err := a.SetSentPolicy(f.accountID, *changed); err != nil {
		t.Fatal(err)
	}
	reviewed, err = a.GetSentPolicy(f.accountID)
	if err != nil || reviewed.Destinations[0].Mode != "append" || reviewed.Destinations[0].MailboxRoute == oldTarget {
		t.Fatalf("explicit target re-review failed: %+v %v", reviewed, err)
	}
}

func TestIMAPCopyConnectionUsesValidatedAccountSnapshot(t *testing.T) {
	f, _, _ := initAppSendFixture(t, appSMTPConfig{finalReply: "250 queued"})
	acc, err := f.ops.accountStore.Get(f.accountID)
	if err != nil {
		t.Fatal(err)
	}
	reviewedRoute := sentMailboxRouteSnapshot(acc)
	// A UI settings write happens after this destination was validated.
	if _, err := f.db.Exec(`UPDATE accounts SET imap_host='other.example.test',imap_port=144,username='other@example.test' WHERE id=?`, acc.ID); err != nil {
		t.Fatal(err)
	}
	config, err := f.ops.getIMAPCredentialsForAccount(context.Background(), acc)
	if err != nil {
		t.Fatal(err)
	}
	if config.Host != acc.IMAPHost || config.Port != acc.IMAPPort || config.Username != acc.Username || sentMailboxRouteSnapshot(acc) != reviewedRoute {
		t.Fatalf("validated destination and connection diverged: %+v", config)
	}
	currentRoute, err := f.ops.sentMailboxRoute(acc.ID)
	if err != nil || currentRoute == reviewedRoute {
		t.Fatalf("fixture did not change destination: %s %v", currentRoute, err)
	}
}
