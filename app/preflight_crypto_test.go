package app

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/pem"
	"fmt"
	"github.com/caseylabs/mailven/internal/logging"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/caseylabs/mailven/internal/pgp"
	"github.com/caseylabs/mailven/internal/smime"
	"github.com/caseylabs/mailven/internal/smtp"
)

func installAppPreflightPGPKey(t *testing.T, f appSendFixture) {
	t.Helper()
	entity, err := openpgp.NewEntity("Sender", "", "sender@example.test", nil)
	if err != nil {
		t.Fatal(err)
	}
	privateKey, err := pgp.ArmorPrivateKey(entity)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := pgp.ArmorPublicKey(entity)
	if err != nil {
		t.Fatal(err)
	}
	key := pgp.ExtractKeyMetadata(entity)
	key.AccountID = f.accountID
	key.IsDefault = true
	if err := f.ops.pgpStore.SaveKey(key, publicKey); err != nil {
		t.Fatal(err)
	}
	if err := f.ops.pgpStore.SetDefaultKey(f.accountID, key.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.ops.credStore.SetPGPPrivateKey(key.ID, []byte(privateKey)); err != nil {
		t.Fatal(err)
	}
	if err := f.ops.pgpStore.CacheSenderKey("recipient@example.test", publicKey, "manual"); err != nil {
		t.Fatal(err)
	}
}

func installAppPreflightSMIMECertificate(t *testing.T, f appSendFixture) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(42),
		Subject:      pkix.Name{CommonName: "Sender"},
		NotBefore:    now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour),
		KeyUsage:       x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:    []x509.ExtKeyUsage{x509.ExtKeyUsageEmailProtection},
		EmailAddresses: []string{"sender@example.test"},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	chainPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})
	certRecord := &smime.Certificate{
		AccountID: f.accountID, Email: "sender@example.test", Subject: cert.Subject.String(),
		Issuer: cert.Issuer.String(), SerialNumber: cert.SerialNumber.String(),
		Fingerprint: fmt.Sprintf("%X", sha256.Sum256(cert.Raw)),
		NotBefore:   cert.NotBefore, NotAfter: cert.NotAfter, IsDefault: true,
	}
	if err := f.ops.smimeStore.SaveCertificate(certRecord, chainPEM); err != nil {
		t.Fatal(err)
	}
	if err := f.ops.smimeStore.SetDefaultCertificate(f.accountID, certRecord.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.ops.credStore.SetSMIMEPrivateKey(certRecord.ID, privatePEM); err != nil {
		t.Fatal(err)
	}
	recipientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	recipientTemplate := *template
	recipientTemplate.SerialNumber = big.NewInt(43)
	recipientTemplate.Subject = pkix.Name{CommonName: "Recipient"}
	recipientTemplate.EmailAddresses = []string{"recipient@example.test"}
	recipientDER, err := x509.CreateCertificate(rand.Reader, &recipientTemplate, &recipientTemplate, &recipientKey.PublicKey, recipientKey)
	if err != nil {
		t.Fatal(err)
	}
	recipientPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: recipientDER}))
	if err := f.ops.smimeStore.CacheSenderCert("recipient@example.test", recipientPEM, true); err != nil {
		t.Fatal(err)
	}
}

func appPreflightCryptoBytes(t *testing.T, f appSendFixture, msg smtp.ComposeMessage, protocol string) []byte {
	t.Helper()
	wire, err := msg.ToRFC822()
	if err != nil {
		t.Fatal(err)
	}
	if protocol == "smime" {
		if msg.SignMessage {
			wire, err = f.ops.smimeSigner.SignMessage(f.accountID, msg.From.Address, wire)
			if err != nil {
				t.Fatal(err)
			}
		}
		if msg.EncryptMessage {
			wire, err = f.ops.smimeEncryptor.EncryptMessage(f.accountID, msg.From.Address, msg.AllRecipients(), wire)
			if err != nil {
				t.Fatal(err)
			}
		}
		return wire
	}
	if msg.PGPSignMessage {
		wire, err = f.ops.pgpSigner.SignMessage(f.accountID, msg.From.Address, wire)
		if err != nil {
			t.Fatal(err)
		}
	}
	if msg.PGPEncryptMessage {
		wire, err = f.ops.pgpEncryptor.EncryptMessage(f.accountID, msg.From.Address, msg.AllRecipients(), wire)
		if err != nil {
			t.Fatal(err)
		}
	}
	return wire
}

func assertAppSMTPNeverSubmitted(t *testing.T, server *appSMTPServer) {
	t.Helper()
	for {
		select {
		case command := <-server.commands:
			upper := strings.ToUpper(command)
			if strings.HasPrefix(upper, "MAIL FROM:") || strings.HasPrefix(upper, "RCPT TO:") || upper == "DATA" {
				t.Fatalf("oversized message reached SMTP submission command %q", command)
			}
		default:
			return
		}
	}
}

func appSMTPWireSize(msg []byte) int {
	size := len(msg)
	for i, b := range msg {
		if b == '\n' && (i == 0 || msg[i-1] != '\r') {
			size++
		}
	}
	if len(msg) == 0 || msg[len(msg)-1] != '\n' {
		size += 2
	}
	return size
}

func assertAppSMTPSizeBoundary(t *testing.T, wire []byte) {
	t.Helper()
	size := appSMTPWireSize(wire)
	for _, tc := range []struct {
		name      string
		limit     int
		wantError bool
	}{
		{name: "at limit", limit: size},
		{name: "one byte over", limit: size - 1, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := startAppSMTPServer(t, appSMTPConfig{finalReply: "250 queued", sizeLimit: tc.limit})
			config := smtp.DefaultConfig()
			config.Host = "localhost"
			config.Port = server.listener.Addr().(*net.TCPAddr).Port
			config.Security = smtp.SecurityNone
			config.Username = "sender@example.test"
			config.Password = "secret"
			client := smtp.NewClient(config)
			if err := client.Connect(); err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			if err := client.Login(); err != nil {
				t.Fatal(err)
			}
			_, err := client.SendMailTracked("sender@example.test", []string{"recipient@example.test"}, wire, nil)
			if tc.wantError {
				if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("encoded message is %d bytes, exceeding the server SIZE limit of %d bytes", size, tc.limit)) {
					t.Fatalf("SendMailTracked error = %v; want %d-byte wire rejected at %d", err, size, tc.limit)
				}
				assertAppSMTPNeverSubmitted(t, server)
				return
			}
			if err != nil {
				t.Fatalf("SendMailTracked at exact SIZE boundary: %v", err)
			}
			_ = readAppSMTP(t, server)
		})
	}
}

func TestSendMessageCryptoWireSizePreflight(t *testing.T) {
	tests := []struct {
		name, protocol, format string
		sign, encrypt          bool
	}{
		{name: "PGP signed", protocol: "pgp", format: "multipart/signed", sign: true},
		{name: "PGP encrypted", protocol: "pgp", format: "multipart/encrypted", encrypt: true},
		{name: "PGP sign then encrypt", protocol: "pgp", format: "multipart/encrypted", sign: true, encrypt: true},
		{name: "S MIME signed", protocol: "smime", format: "multipart/signed", sign: true},
		{name: "S MIME encrypted", protocol: "smime", format: "application/pkcs7-mime", encrypt: true},
		{name: "S MIME sign then encrypt", protocol: "smime", format: "application/pkcs7-mime", sign: true, encrypt: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, server, _ := initAppSendFixture(t, appSMTPConfig{finalReply: "250 queued", sizeLimit: 1})
			if tt.protocol == "pgp" {
				installAppPreflightPGPKey(t, f)
			} else {
				installAppPreflightSMIMECertificate(t, f)
			}
			msg := appSendMessage()
			msg.SignMessage, msg.EncryptMessage = tt.sign && tt.protocol == "smime", tt.encrypt && tt.protocol == "smime"
			msg.PGPSignMessage, msg.PGPEncryptMessage = tt.sign && tt.protocol == "pgp", tt.encrypt && tt.protocol == "pgp"
			attempt, err := f.ops.prepareSubmission(f.accountID, msg, f.draft.ID)
			if err != nil {
				t.Fatal(err)
			}
			msg.MessageID = attempt.MessageID
			plain, err := msg.ToRFC822()
			if err != nil {
				t.Fatal(err)
			}
			wire := appPreflightCryptoBytes(t, f, msg, tt.protocol)
			if !strings.Contains(strings.ToLower(string(wire)), tt.format) {
				t.Fatalf("crypto output does not contain %q: %q", tt.format, wire[:min(len(wire), 256)])
			}
			_, err = f.ops.sendPrepared(context.Background(), f.accountID, msg, f.draft, attempt)
			if err == nil || !strings.Contains(err.Error(), "exceeding the server SIZE limit of 1 bytes") {
				t.Fatalf("send error = %v; want final crypto bytes rejected by SIZE", err)
			}
			var sentSize int
			if _, scanErr := fmt.Sscanf(err.Error(), "failed to send message: encoded message is %d bytes", &sentSize); scanErr != nil {
				t.Fatalf("send error %q does not report transformed wire size", err)
			}
			if sentSize <= len(plain) || sentSize < len(wire)-256 || sentSize > len(wire)+256 {
				t.Fatalf("SIZE preflight counted %d bytes; plain=%d, independently generated crypto wire=%d", sentSize, len(plain), len(wire))
			}
			assertAppSMTPSizeBoundary(t, wire)
			assertAppSMTPNeverSubmitted(t, server)
			assertDraftRemains(t, f)
			if attempt.Outcome != "not_submitted" {
				t.Fatalf("attempt outcome = %q; want not_submitted", attempt.Outcome)
			}
		})
	}
}

func TestSendMessageRejectsLongSubjectAndRetainsDraft(t *testing.T) {
	f, server, _ := initAppSendFixture(t, appSMTPConfig{finalReply: "250 queued", sizeLimit: 1})
	msg := appSendMessage()
	msg.To[0].Address = "recipient@gmail.com"
	msg.Subject = strings.Repeat("subject ", 130)
	msg.TextBody = "content saved before oversized subject rejection"
	f.draft.Subject = msg.Subject
	f.draft.BodyText = msg.TextBody
	if err := f.draftStore.Update(f.draft); err != nil {
		t.Fatal(err)
	}
	attempt, err := f.ops.prepareSubmission(f.accountID, msg, f.draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	msg.MessageID = attempt.MessageID
	_, err = f.ops.sendPrepared(context.Background(), f.accountID, msg, f.draft, attempt)
	if err == nil || !strings.Contains(err.Error(), "Gmail Subject header exceeds 998 characters") {
		t.Fatalf("send error = %v; want long raw Subject rejected by header preflight", err)
	}
	assertAppSMTPNeverSubmitted(t, server)
	assertDraftRemains(t, f)
	retained, err := f.draftStore.Get(f.draft.ID)
	if err != nil || retained == nil || retained.Subject != msg.Subject || retained.BodyText != msg.TextBody {
		t.Fatalf("retained draft = %+v, err=%v; want original subject and body", retained, err)
	}
	retained.BodyText += " edited after rejection"
	if err := f.draftStore.Update(retained); err != nil {
		t.Fatalf("edit retained draft after rejection: %v", err)
	}
	retained, err = f.draftStore.Get(f.draft.ID)
	if err != nil || retained == nil || retained.BodyText != msg.TextBody+" edited after rejection" {
		t.Fatalf("edited draft = %+v, err=%v", retained, err)
	}
	if attempt.Outcome != "not_submitted" {
		t.Fatalf("attempt outcome = %q; want not_submitted", attempt.Outcome)
	}
}

func TestSendMessageMissingEncryptionKeyNeverFallsBack(t *testing.T) {
	for _, protocol := range []string{"smime", "pgp"} {
		t.Run(protocol, func(t *testing.T) {
			f, server, _ := initAppSendFixture(t, appSMTPConfig{finalReply: "250 queued"})
			msg := appSendMessage()
			msg.EncryptMessage = protocol == "smime"
			msg.PGPEncryptMessage = protocol == "pgp"
			_, err := f.ops.sendMessage(context.Background(), f.accountID, msg, f.draft)
			if err == nil || !strings.Contains(err.Error(), "encryption is unavailable") {
				t.Fatalf("err=%v", err)
			}
			assertAppSMTPNeverSubmitted(t, server)
			assertDraftRemains(t, f)
			if attempt := getAttempt(t, f); attempt.Outcome != "not_submitted" {
				t.Fatalf("attempt=%+v", attempt)
			}
		})
	}
}

func TestSendMessageMissingPolicyEncryptionKeyNeverFallsBack(t *testing.T) {
	for _, protocol := range []string{"smime", "pgp"} {
		t.Run(protocol, func(t *testing.T) {
			f, server, _ := initAppSendFixture(t, appSMTPConfig{finalReply: "250 queued"})
			var err error
			if protocol == "smime" {
				err = f.ops.smimeStore.SetEncryptPolicy(f.accountID, "always")
			} else {
				err = f.ops.pgpStore.SetEncryptPolicy(f.accountID, "always")
			}
			if err != nil {
				t.Fatal(err)
			}
			_, err = f.ops.sendMessage(context.Background(), f.accountID, appSendMessage(), f.draft)
			if err == nil || !strings.Contains(err.Error(), "encryption is unavailable") {
				t.Fatalf("err=%v", err)
			}
			assertAppSMTPNeverSubmitted(t, server)
			assertDraftRemains(t, f)
		})
	}
}

func TestSendMessageEncryptionPolicyReadFailureStopsSubmission(t *testing.T) {
	for _, protocol := range []string{"smime", "pgp"} {
		t.Run(protocol, func(t *testing.T) {
			f, server, _ := initAppSendFixture(t, appSMTPConfig{finalReply: "250 queued"})
			brokenDB, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Fatal(err)
			}
			defer brokenDB.Close()
			// No accounts table: policy lookup fails, while the journal/draft DB works.
			if protocol == "smime" {
				f.ops.smimeStore = smime.NewStore(brokenDB, logging.WithComponent("test"))
			} else {
				f.ops.pgpStore = pgp.NewStore(brokenDB, logging.WithComponent("test"))
			}
			_, err = f.ops.sendMessage(context.Background(), f.accountID, appSendMessage(), f.draft)
			if err == nil || !strings.Contains(err.Error(), "encryption policy") {
				t.Fatalf("err=%v", err)
			}
			assertAppSMTPNeverSubmitted(t, server)
			assertDraftRemains(t, f)
		})
	}
}
