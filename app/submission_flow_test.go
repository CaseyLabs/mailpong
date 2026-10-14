package app

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/certificate"
	"github.com/caseylabs/mailven/internal/credentials"
	"github.com/caseylabs/mailven/internal/database"
	"github.com/caseylabs/mailven/internal/draft"
	"github.com/caseylabs/mailven/internal/folder"
	"github.com/caseylabs/mailven/internal/logging"
	"github.com/caseylabs/mailven/internal/message"
	"github.com/caseylabs/mailven/internal/pgp"
	"github.com/caseylabs/mailven/internal/smime"
	"github.com/caseylabs/mailven/internal/smtp"
	"github.com/caseylabs/mailven/internal/submission"
	gokeyring "github.com/zalando/go-keyring"
)

type appSMTPConfig struct {
	finalReply string
	dropFinal  bool
	sizeLimit  int
}

type appSMTPServer struct {
	listener    net.Listener
	connections chan struct{}
	bodies      chan []byte
	commands    chan string
}

func startAppSMTPServer(t *testing.T, cfg appSMTPConfig) *appSMTPServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &appSMTPServer{listener: ln, connections: make(chan struct{}, 8), bodies: make(chan []byte, 8), commands: make(chan string, 128)}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			s.connections <- struct{}{}
			go s.serve(conn, cfg)
		}
	}()
	return s
}

func (s *appSMTPServer) serve(conn net.Conn, cfg appSMTPConfig) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	_, _ = fmt.Fprint(conn, "220 localhost ESMTP ready\r\n")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		s.commands <- strings.TrimSpace(line)
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(cmd, "EHLO"):
			if cfg.sizeLimit > 0 {
				_, _ = fmt.Fprintf(conn, "250-localhost\r\n250-AUTH PLAIN\r\n250-SIZE %d\r\n250 8BITMIME\r\n", cfg.sizeLimit)
			} else {
				_, _ = fmt.Fprint(conn, "250-localhost\r\n250-AUTH PLAIN\r\n250 8BITMIME\r\n")
			}
		case strings.HasPrefix(cmd, "AUTH PLAIN"):
			_, _ = fmt.Fprint(conn, "235 2.7.0 authenticated\r\n")
		case strings.HasPrefix(cmd, "MAIL FROM:"):
			_, _ = fmt.Fprint(conn, "250 2.1.0 sender accepted\r\n")
		case strings.HasPrefix(cmd, "RCPT TO:"):
			_, _ = fmt.Fprint(conn, "250 2.1.5 recipient accepted\r\n")
		case cmd == "DATA":
			_, _ = fmt.Fprint(conn, "354 send message\r\n")
			var body strings.Builder
			for {
				bodyLine, readErr := r.ReadString('\n')
				if readErr != nil {
					return
				}
				if strings.TrimSpace(bodyLine) == "." {
					s.bodies <- []byte(body.String())
					if cfg.dropFinal {
						return
					}
					_, _ = fmt.Fprint(conn, cfg.finalReply, "\r\n")
					break
				}
				body.WriteString(bodyLine)
			}
		case cmd == "QUIT":
			_, _ = fmt.Fprint(conn, "221 bye\r\n")
			return
		default:
			_, _ = fmt.Fprint(conn, "250 ok\r\n")
		}
	}
}

type appSendFixture struct {
	db         *database.DB
	accountID  string
	draft      *draft.Draft
	draftStore *draft.Store
	ops        *composeOps
	dataDir    string
}

func openSendDB(t *testing.T, path string) *database.DB {
	t.Helper()
	db, err := database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	return db
}

func makeAppSendFixture(t *testing.T, db *database.DB, server *appSMTPServer, dataDir string, createAccount bool, accountID string) appSendFixture {
	t.Helper()
	accounts := account.NewStore(db)
	credStore, err := credentials.NewStore(db.DB, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if createAccount {
		acc, err := accounts.Create(&account.AccountConfig{
			Name: "Test", DisplayName: "Sender", Email: "sender@example.test",
			IMAPHost: "imap.gmail.com", IMAPPort: 993, IMAPSecurity: account.SecurityTLS,
			SMTPHost: "localhost", SMTPPort: server.listener.Addr().(*net.TCPAddr).Port, SMTPSecurity: account.SecurityNone,
			AuthType: account.AuthPassword, Username: "sender@example.test",
		})
		if err != nil {
			t.Fatal(err)
		}
		accountID = acc.ID
		if err := credStore.SetPassword(accountID, "secret"); err != nil {
			t.Fatal(err)
		}
	}
	folders := folder.NewStore(db)
	messages := message.NewStore(db)
	draftStore := draft.NewStore(db)
	d := &draft.Draft{ID: "draft-send-flow", AccountID: accountID, Subject: "durable outcomes", BodyText: "message body"}
	if createAccount {
		if err := draftStore.Create(d); err != nil {
			t.Fatal(err)
		}
	} else {
		d, err = draftStore.Get("draft-send-flow")
		if err != nil || d == nil {
			t.Fatalf("load retained draft: %v, %+v", err, d)
		}
	}
	certStore := certificate.NewStore(db.DB)
	smimeStore := smime.NewStore(db.DB, logging.WithComponent("test"))
	pgpStore := pgp.NewStore(db.DB, logging.WithComponent("test"))
	draftOps := &draftOps{accountStore: accounts, folderStore: folders, messageStore: messages, draftStore: draftStore}
	ops := &composeOps{
		submissions: submission.NewStore(db), accountStore: accounts, folderStore: folders,
		credStore: credStore, certStore: certStore, smimeStore: smimeStore,
		smimeSigner:    smime.NewSigner(smimeStore, credStore, logging.WithComponent("test")),
		smimeEncryptor: smime.NewEncryptor(smimeStore, credStore, logging.WithComponent("test")),
		pgpStore:       pgpStore,
		pgpSigner:      pgp.NewSigner(pgpStore, credStore, logging.WithComponent("test")),
		pgpEncryptor:   pgp.NewEncryptor(pgpStore, credStore, logging.WithComponent("test")),
		draftOps:       draftOps,
	}
	return appSendFixture{db: db, accountID: accountID, draft: d, draftStore: draftStore, ops: ops, dataDir: dataDir}
}

func appSendMessage() smtp.ComposeMessage {
	return smtp.ComposeMessage{
		From:    smtp.Address{Name: "Sender", Address: "sender@example.test"},
		To:      []smtp.Address{{Name: "Recipient", Address: "recipient@example.test"}},
		Subject: "durable outcomes", TextBody: "message body",
	}
}

func initAppSendFixture(t *testing.T, cfg appSMTPConfig) (appSendFixture, *appSMTPServer, string) {
	t.Helper()
	gokeyring.MockInit()
	server := startAppSMTPServer(t, cfg)
	dbPath := filepath.Join(t.TempDir(), "send-flow.db")
	dataDir := filepath.Join(t.TempDir(), "data")
	db := openSendDB(t, dbPath)
	t.Cleanup(func() { _ = db.Close() })
	fixture := makeAppSendFixture(t, db, server, dataDir, true, "")
	return fixture, server, dbPath
}

func getAttempt(t *testing.T, f appSendFixture) *submission.Attempt {
	t.Helper()
	attempts, err := f.ops.submissions.List(f.accountID)
	if err != nil || len(attempts) != 1 {
		t.Fatalf("attempts = %v, err = %v", attempts, err)
	}
	return attempts[0]
}

func assertDraftRemains(t *testing.T, f appSendFixture) {
	t.Helper()
	d, err := f.draftStore.Get(f.draft.ID)
	if err != nil || d == nil {
		t.Fatalf("draft not retained: %v, %+v", err, d)
	}
}

func readAppSMTP(t *testing.T, s *appSMTPServer) []byte {
	t.Helper()
	select {
	case body := <-s.bodies:
		return body
	case <-time.After(3 * time.Second):
		t.Fatal("SMTP server did not receive message body")
		return nil
	}
}

func TestSendMessageAcceptedCleanupFailureKeepsJournalAndDraft(t *testing.T) {
	f, server, _ := initAppSendFixture(t, appSMTPConfig{finalReply: "250 2.0.0 queued"})
	msg := appSendMessage()
	_, err := f.ops.sendMessage(context.Background(), f.accountID, msg, f.draft)
	if err != nil {
		t.Fatalf("accepted send returned an error after cleanup panic: %v", err)
	}
	attempt := getAttempt(t, f)
	if attempt.Outcome != "accepted" || attempt.Warnings == "" {
		t.Fatalf("attempt = %+v; want accepted with cleanup warning", attempt)
	}
	body := readAppSMTP(t, server)
	if !strings.Contains(string(body), "Message-ID: "+attempt.MessageID) {
		t.Fatalf("SMTP body has no journal Message-ID %q:\n%s", attempt.MessageID, body)
	}
	assertDraftRemains(t, f)
}

func TestSendMessageUnknownSurvivesRestartAndBlocksReplay(t *testing.T) {
	f, server, dbPath := initAppSendFixture(t, appSMTPConfig{dropFinal: true})
	msg := appSendMessage()
	_, sendErr := f.ops.sendMessage(context.Background(), f.accountID, msg, f.draft)
	if sendErr == nil || !strings.Contains(sendErr.Error(), "duplicates") {
		t.Fatalf("send error = %v; want duplicate-risk review", sendErr)
	}
	attempt := getAttempt(t, f)
	if attempt.Outcome != "unknown" || attempt.Stage != "data_final_reply" {
		t.Fatalf("attempt = %+v; want unknown final-reply outcome", attempt)
	}
	body := readAppSMTP(t, server)
	if !strings.Contains(string(body), "Message-ID: "+attempt.MessageID) {
		t.Fatalf("SMTP body Message-ID does not match journal %q", attempt.MessageID)
	}
	select {
	case <-server.connections:
	case <-time.After(time.Second):
		t.Fatal("initial SMTP connection was not observed")
	}
	if err := f.db.Close(); err != nil {
		t.Fatal(err)
	}

	db := openSendDB(t, dbPath)
	t.Cleanup(func() { _ = db.Close() })
	restarted := makeAppSendFixture(t, db, server, f.dataDir, false, f.accountID)
	_, retryErr := restarted.ops.sendMessage(context.Background(), restarted.accountID, msg, restarted.draft)
	if retryErr == nil || !strings.Contains(retryErr.Error(), "duplicates") {
		t.Fatalf("post-restart send = %v; want journal replay block", retryErr)
	}
	select {
	case <-server.connections:
		t.Fatal("post-restart review opened a new SMTP connection")
	case <-time.After(150 * time.Millisecond):
	}
	assertDraftRemains(t, restarted)
}

func TestSendMessageAcceptanceJournalFailureLeavesUnknownAndDraft(t *testing.T) {
	f, server, _ := initAppSendFixture(t, appSMTPConfig{finalReply: "250 2.0.0 queued"})
	_, err := f.db.Exec(`CREATE TRIGGER fail_acceptance BEFORE UPDATE OF outcome ON submission_attempts WHEN NEW.outcome='accepted' BEGIN SELECT RAISE(ABORT, 'forced acceptance journal failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	_, sendErr := f.ops.sendMessage(context.Background(), f.accountID, appSendMessage(), f.draft)
	if sendErr == nil || !strings.Contains(sendErr.Error(), "duplicates") {
		t.Fatalf("send error = %v; want unknown-outcome review after acceptance journal failure", sendErr)
	}
	attempt := getAttempt(t, f)
	if attempt.Outcome != "unknown" {
		t.Fatalf("attempt = %+v; want unknown after journal write failure", attempt)
	}
	body := readAppSMTP(t, server)
	if !strings.Contains(string(body), "Message-ID: "+attempt.MessageID) {
		t.Fatalf("SMTP body Message-ID does not match journal %q", attempt.MessageID)
	}
	assertDraftRemains(t, f)
}

func TestSendMessageDefiniteDATARejectionKeepsDraft(t *testing.T) {
	f, _, _ := initAppSendFixture(t, appSMTPConfig{finalReply: "554 5.5.0 DATA rejected"})
	_, err := f.ops.sendMessage(context.Background(), f.accountID, appSendMessage(), f.draft)
	if err == nil {
		t.Fatal("send succeeded after permanent DATA rejection")
	}
	attempt := getAttempt(t, f)
	if attempt.Outcome != "rejected_permanent" || attempt.Stage != "data_final_reply" {
		t.Fatalf("attempt = %+v; want permanent final DATA rejection", attempt)
	}
	assertDraftRemains(t, f)
}

func TestReadReceiptDoesNotReplayAfterUnknownOrCleanupFailure(t *testing.T) {
	for _, unknown := range []bool{true, false} {
		t.Run(fmt.Sprintf("unknown=%t", unknown), func(t *testing.T) {
			f, server, _ := initAppSendFixture(t, appSMTPConfig{finalReply: "250 queued", dropFinal: unknown})
			if err := f.ops.folderStore.Create(&folder.Folder{ID: "inbox", AccountID: f.accountID, Name: "Inbox", Path: "INBOX", Type: folder.TypeInbox}); err != nil {
				t.Fatal(err)
			}
			messages := message.NewStore(f.db)
			if err := messages.Create(&message.Message{ID: "original", AccountID: f.accountID, FolderID: "inbox", UID: 1, MessageID: "<original@example.test>", FromEmail: "recipient@example.test", ReadReceiptTo: "recipient@example.test", Date: time.Now()}); err != nil {
				t.Fatal(err)
			}
			if !unknown {
				_, err := f.db.Exec(`CREATE TRIGGER fail_receipt_cleanup BEFORE UPDATE OF read_receipt_handled ON messages BEGIN SELECT RAISE(ABORT,'receipt cleanup failed'); END`)
				if err != nil {
					t.Fatal(err)
				}
			}
			a := &App{accountStore: f.ops.accountStore, credStore: f.ops.credStore, certStore: f.ops.certStore, messageStore: messages, composeOps: *f.ops}
			err := a.SendReadReceipt(f.accountID, "original")
			if (err != nil) != unknown {
				t.Fatalf("receipt result %v, unknown=%t", err, unknown)
			}
			attempt := getAttempt(t, f)
			want := "accepted"
			if unknown {
				want = "unknown"
			}
			if attempt.Outcome != want {
				t.Fatalf("got %+v", attempt)
			}
			if !unknown && attempt.Warnings == "" {
				t.Fatal("receipt cleanup warning lost")
			}
			body := readAppSMTP(t, server)
			if !strings.Contains(string(body), "Message-ID: "+attempt.MessageID) {
				t.Fatal("receipt ID did not match journal")
			}
			<-server.connections
			if err := a.SendReadReceipt(f.accountID, "original"); err == nil {
				t.Fatal("receipt replay was not blocked")
			}
			select {
			case <-server.connections:
				t.Fatal("receipt replay opened SMTP")
			case <-time.After(150 * time.Millisecond):
			}
		})
	}
}

func TestRecoveryDraftRejectsEncryptionFallback(t *testing.T) {
	for _, pgp := range []bool{false, true} {
		t.Run(fmt.Sprintf("pgp=%t", pgp), func(t *testing.T) {
			f, _, _ := initAppSendFixture(t, appSMTPConfig{finalReply: "250 queued"})
			f.ops.draftOps.smimeEncryptor = f.ops.smimeEncryptor
			f.ops.draftOps.pgpEncryptor = f.ops.pgpEncryptor
			msg := appSendMessage()
			msg.EncryptMessage = !pgp
			msg.PGPEncryptMessage = pgp
			if _, err := f.ops.draftOps.persistRecoveryDraft(f.accountID, msg, f.draft); err == nil {
				t.Fatal("recovery write silently downgraded encryption")
			}
			d, err := f.draftStore.Get(f.draft.ID)
			if err != nil || d == nil || d.BodyText != f.draft.BodyText {
				t.Fatalf("existing draft was overwritten: %+v %v", d, err)
			}
		})
	}
}
