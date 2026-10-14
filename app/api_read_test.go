package app

import (
	"bytes"
	"strings"
	"testing"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/message"
)

// activeAPIMessage activates the rig's route and returns the synced API row.
func activeAPIMessage(t *testing.T, r *routeRig) *message.Message {
	t.Helper()
	if err := r.app.ActivateMailAPIRoute(r.acc.ID); err != nil {
		t.Fatal(err)
	}
	_, rows, err := r.app.messageStore.GetAPIRef(r.acc.ID, message.APIProviderGmail, "m1")
	if err != nil || len(rows) != 1 {
		t.Fatalf("api rows = %v, %v", rows, err)
	}
	for _, id := range rows {
		m, err := r.app.messageStore.Get(id)
		if err != nil || m == nil {
			t.Fatalf("Get = %v, %v", m, err)
		}
		return m
	}
	return nil
}

func (r *routeRig) serve(raw []byte, gone bool) {
	r.gmail.mu.Lock()
	r.gmail.raw, r.gmail.gone = raw, gone
	r.gmail.mu.Unlock()
}

func TestAPIBodyRawComesFromProviderNotIMAP(t *testing.T) {
	r := newRouteRig(t)
	m := activeAPIMessage(t, r)
	raw, err := r.app.fetchRawMessage(r.app.ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "Subject: api") {
		t.Fatalf("raw = %q", raw)
	}
}

func TestAPIBodyFetchStoresParsedBodyAndAttachments(t *testing.T) {
	r := newRouteRig(t)
	m := activeAPIMessage(t, r)
	r.serve([]byte("From: S <s@example.com>\r\nTo: user@example.test\r\nSubject: api\r\nMessage-ID: <m1@example.com>\r\n"+
		"MIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=B\r\n\r\n"+
		"--B\r\nContent-Type: text/plain; charset=iso-8859-1\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\ncaf=E9\r\n"+
		"--B\r\nContent-Type: application/pdf; name=a.pdf\r\nContent-Disposition: attachment; filename=a.pdf\r\n"+
		"Content-Transfer-Encoding: base64\r\n\r\nUERG\r\n--B--\r\n"), false)
	got, err := r.app.fetchBody(r.app.ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	if !got.BodyFetched || !strings.Contains(got.BodyText, "café") {
		t.Fatalf("BodyFetched=%v body=%q; want decoded non-UTF-8 text", got.BodyFetched, got.BodyText)
	}
	if !got.HasAttachments {
		t.Fatal("HasAttachments not set")
	}
}

func TestAPIBodyCryptoSourceIsRawMIME(t *testing.T) {
	r := newRouteRig(t)
	m := activeAPIMessage(t, r)
	for name, body := range map[string]string{
		"signed": "Content-Type: multipart/signed; protocol=\"application/pgp-signature\"; micalg=pgp-sha256; boundary=S\r\n\r\n" +
			"--S\r\nContent-Type: text/plain\r\n\r\nhi\r\n--S\r\nContent-Type: application/pgp-signature\r\n\r\nSIG\r\n--S--\r\n",
		"encrypted": "Content-Type: multipart/encrypted; protocol=\"application/pgp-encrypted\"; boundary=E\r\n\r\n" +
			"--E\r\nContent-Type: application/pgp-encrypted\r\n\r\nVersion: 1\r\n--E\r\nContent-Type: application/octet-stream\r\n\r\nCIPHER\r\n--E--\r\n",
	} {
		src := []byte("From: S <s@example.com>\r\nSubject: " + name + "\r\nMIME-Version: 1.0\r\n" + body)
		r.serve(src, false)
		got, err := r.app.fetchRawMessage(r.app.ctx, m)
		if err != nil || !bytes.Equal(got, src) {
			t.Fatalf("%s: raw mismatch, err=%v", name, err)
		}
	}
}

func TestAPIBodyOfflineUsesCachedBody(t *testing.T) {
	r := newRouteRig(t)
	m := activeAPIMessage(t, r)
	if !m.BodyFetched {
		t.Fatal("API sync should store the body")
	}
	r.serve(nil, true) // provider unreachable for raw reads
	got, err := r.app.GetMessage(m.ID)
	if err != nil || got == nil || !strings.Contains(got.BodyText, "body") {
		t.Fatalf("GetMessage offline = %+v, %v", got, err)
	}
}

func TestAPIBodyGoneAndGuards(t *testing.T) {
	r := newRouteRig(t)
	m := activeAPIMessage(t, r)

	r.serve(nil, true)
	if _, err := r.app.fetchBody(r.app.ctx, m); err == nil || !strings.Contains(err.Error(), "no longer exists") {
		t.Fatalf("404 error = %v", err)
	}
	r.serve(nil, false)

	route, _ := r.app.accountStore.GetMailAPIRoute(r.acc.ID)
	shared := *route
	shared.Shared = true
	if _, err := r.app.fetchAPIRaw(r.app.ctx, &shared, m); err == nil || !strings.Contains(err.Error(), "shared") {
		t.Fatalf("shared error = %v", err)
	}

	if err := r.app.credStore.ClearOAuthRefreshTokenForClientConfig(r.acc.ID, route.ClientConfigID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.app.fetchRawMessage(r.app.ctx, m); err == nil {
		t.Fatal("read with cleared credentials succeeded; must not fall back to IMAP")
	}
}

func TestAPIAttachmentRawPathIgnoresSurrogateUID(t *testing.T) {
	r := newRouteRig(t)
	m := activeAPIMessage(t, r)
	if r.app.apiRouted(m.AccountID) == nil || r.app.apiRouted(m.AccountID).State != account.MailRouteStateActive {
		t.Fatal("route should be active")
	}
	// syncEngine has no IMAP pool: any IMAP attempt would panic or error out.
	if _, err := r.app.fetchRawMessage(r.app.ctx, m); err != nil {
		t.Fatalf("API raw read: %v", err)
	}
}

func TestAPIServerSearchAndFetchRefuseAPIRoutedAccount(t *testing.T) {
	r := newRouteRig(t)
	m := activeAPIMessage(t, r)
	if _, err := r.app.IMAPSearchFolder(r.acc.ID, m.FolderID, "x", 10); err == nil || !strings.Contains(err.Error(), "mail API") {
		t.Fatalf("IMAPSearchFolder err = %v", err)
	}
	if _, err := r.app.FetchServerMessage(r.acc.ID, m.FolderID, int(m.UID)); err == nil || !strings.Contains(err.Error(), "mail API") {
		t.Fatalf("FetchServerMessage err = %v", err)
	}
}
