package app

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/database"
	"github.com/caseylabs/mailven/internal/draft"
	"github.com/caseylabs/mailven/internal/logging"
	"github.com/caseylabs/mailven/internal/pgp"
	"github.com/caseylabs/mailven/internal/smime"
	"github.com/caseylabs/mailven/internal/smtp"
)

func TestAutosaveEncryptionFailurePreservesStorage(t *testing.T) {
	for _, protocol := range []string{"S/MIME", "PGP"} {
		for _, failure := range []string{"missing key", "unusable key"} {
			for _, composer := range []string{"main", "detached"} {
				for _, existing := range []bool{false, true} {
					t.Run(protocol+"/"+failure+"/"+composer+"/existing="+map[bool]string{false: "false", true: "true"}[existing], func(t *testing.T) {
						db, err := database.Open(filepath.Join(t.TempDir(), "draft.db"))
						if err != nil {
							t.Fatal(err)
						}
						t.Cleanup(func() { db.Close() })
						if err := db.Migrate(); err != nil {
							t.Fatal(err)
						}
						accounts := account.NewStore(db)
						acc, err := accounts.Create(&account.AccountConfig{Name: "Fixture", DisplayName: "Sender", IMAPHost: "imap.example.test", IMAPPort: 993, IMAPSecurity: account.SecurityTLS, SMTPHost: "smtp.example.test", SMTPPort: 587, SMTPSecurity: account.SecurityStartTLS, Username: "sender@example.test", Email: "sender@example.test", AuthType: account.AuthPassword})
						if err != nil {
							t.Fatal(err)
						}
						store := draft.NewStore(db)
						smimeStore := smime.NewStore(db.DB, logging.WithComponent("test"))
						pgpStore := pgp.NewStore(db.DB, logging.WithComponent("test"))
						ops := &draftOps{accountStore: accounts, draftStore: store,
							smimeEncryptor: smime.NewEncryptor(smimeStore, nil, logging.WithComponent("test")),
							pgpEncryptor:   pgp.NewEncryptor(pgpStore, nil, logging.WithComponent("test"))}
						if failure == "unusable key" {
							if protocol == "S/MIME" {
								err = smimeStore.SaveCertificate(&smime.Certificate{AccountID: acc.ID, Email: acc.Email, Fingerprint: "fixture", NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour), IsDefault: true}, "invalid certificate")
							} else {
								err = pgpStore.SaveKey(&pgp.Key{AccountID: acc.ID, Email: acc.Email, Fingerprint: "fixture", IsDefault: true}, "invalid public key")
							}
							if err != nil {
								t.Fatal(err)
							}
						}
						var before *draft.Draft
						id := ""
						if existing {
							// An opaque encrypted prior revision must survive even when its key is unavailable.
							d := &draft.Draft{AccountID: acc.ID, Subject: "previous revision", Encrypted: true, EncryptedBody: []byte("previous ciphertext"), SyncStatus: draft.SyncStatusSynced, IMAPUID: 42}
							if err := store.Create(d); err != nil {
								t.Fatal(err)
							}
							id = d.ID
							before, err = store.Get(id)
							if err != nil {
								t.Fatal(err)
							}
						}
						msg := smtp.ComposeMessage{From: smtp.Address{Address: acc.Email}, Subject: "new revision", HTMLBody: "<p>private html sentinel</p>", TextBody: "private text sentinel", EncryptMessage: protocol == "S/MIME", PGPEncryptMessage: protocol == "PGP", Attachments: []smtp.Attachment{{Filename: "secret.txt", ContentBase64: "cHJpdmF0ZSBhdHRhY2htZW50IHNlbnRpbmVs"}}}
						enc, err := ops.encryptDraftBody(acc.ID, acc.Email, msg)
						if err == nil || enc != nil {
							t.Fatalf("encryption result=%+v err=%v", enc, err)
						}
						if composer == "main" {
							a := &App{draftOps: *ops, draftStore: store}
							result, saveErr := a.SaveDraft(acc.ID, msg, id)
							err = saveErr
							if result != nil {
								t.Fatalf("unexpected save result %+v", result)
							}
						} else {
							c := &ComposerApp{draftOps: *ops, draftStore: store, currentDraft: before}
							result, saveErr := c.SaveDraft(acc.ID, msg, id)
							err = saveErr
							if result != nil {
								t.Fatalf("unexpected save result %+v", result)
							}
							if !reflect.DeepEqual(c.currentDraft, before) {
								t.Fatal("detached current draft changed")
							}
						}
						if err == nil || !strings.Contains(err.Error(), "draft not saved: "+protocol+" encryption failed") {
							t.Fatalf("save error=%v", err)
						}
						rows, err := store.ListByAccount(acc.ID)
						if err != nil {
							t.Fatal(err)
						}
						if !existing {
							if len(rows) != 0 {
								t.Fatalf("failed create persisted %+v", rows)
							}
							return
						}
						after, err := store.Get(id)
						if err != nil {
							t.Fatal(err)
						}
						if len(rows) != 1 || !reflect.DeepEqual(before, after) {
							t.Fatalf("failed update changed stored draft: before=%+v after=%+v", before, after)
						}
					})
				}
			}
		}
	}
}

func TestDraftUnencryptedPayloadPreserved(t *testing.T) {
	msg := smtp.ComposeMessage{HTMLBody: "<p>body</p>", TextBody: "body", Attachments: []smtp.Attachment{{Filename: "plain.txt", ContentBase64: "YXR0YWNobWVudA=="}}}
	enc, err := (&draftOps{}).encryptDraftBody("", "", msg)
	if err != nil {
		t.Fatal(err)
	}
	if enc.bodyHTML != msg.HTMLBody || enc.bodyText != msg.TextBody || enc.encrypted || enc.pgpEncrypted || !strings.Contains(string(enc.attachmentsData), "YXR0YWNobWVudA==") {
		t.Fatalf("plaintext payload changed: %+v", enc)
	}
}

func TestDraftEncryptedPayloadRoundTrip(t *testing.T) {
	for _, protocol := range []string{"S/MIME", "PGP"} {
		t.Run(protocol, func(t *testing.T) {
			// Existing disposable crypto fixture; the local SMTP stub is never submitted to.
			f, _, _ := initAppSendFixture(t, appSMTPConfig{finalReply: "250 queued"})
			ops := f.ops.draftOps
			ops.smimeEncryptor = f.ops.smimeEncryptor
			ops.pgpEncryptor = f.ops.pgpEncryptor
			ops.smimeDecryptor = smime.NewDecryptor(f.ops.smimeStore, f.ops.credStore, logging.WithComponent("test"))
			ops.pgpDecryptor = pgp.NewDecryptor(f.ops.pgpStore, f.ops.credStore, logging.WithComponent("test"))
			if protocol == "S/MIME" {
				installAppPreflightSMIMECertificate(t, f)
			} else {
				installAppPreflightPGPKey(t, f)
			}
			msg := appSendMessage()
			msg.HTMLBody = "<p>private html</p>"
			msg.TextBody = "private text"
			msg.Attachments = []smtp.Attachment{{Filename: "secret.txt", ContentBase64: "YXR0YWNobWVudA=="}}
			msg.EncryptMessage = protocol == "S/MIME"
			msg.PGPEncryptMessage = protocol == "PGP"
			enc, err := ops.encryptDraftBody(f.accountID, msg.From.Address, msg)
			if err != nil {
				t.Fatal(err)
			}
			d, err := ops.saveDraftToDB(f.accountID, f.draft, msg, enc)
			if err != nil {
				t.Fatal(err)
			}
			stored, err := f.draftStore.Get(d.ID)
			if err != nil {
				t.Fatal(err)
			}
			if stored.BodyHTML != "" || stored.BodyText != "" || len(stored.AttachmentsData) != 0 || stored.Encrypted != msg.EncryptMessage || stored.PGPEncrypted != msg.PGPEncryptMessage {
				t.Fatalf("plaintext or wrong flags persisted: %+v", stored)
			}
			restored := ops.toComposeMessage(stored)
			if restored.HTMLBody != msg.HTMLBody || restored.TextBody != msg.TextBody || len(restored.Attachments) != 1 || string(restored.Attachments[0].Content) != "attachment" {
				t.Fatalf("encrypted round trip lost content: %+v", restored)
			}
		})
	}
}
