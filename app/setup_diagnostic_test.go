package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/credentials"
	"github.com/caseylabs/mailven/internal/setupdiagnostic"
)

func TestDiagnosticGrantAndExportRedaction(t *testing.T) {
	r := &SetupDiagnosticReport{RunID: "run", AccountID: "private-id", Mailbox: "private@example.test", From: "alias@example.test", Route: "imap-smtp", Checks: []setupdiagnostic.Check{}}
	recordDiagnosticGrant(r, &credentials.OAuthTokens{AccessToken: "secret-access", RefreshToken: "secret-refresh", RequestedScopes: []string{"requested-secret"}, ScopesKnown: true, Scopes: []string{"https://outlook.office.com/SMTP.Send", "https://secret.example/token?private=secret", "opaque-secret"}, ScopeResource: "https://outlook.office.com", ExpiresAt: time.Now().Add(time.Hour)})
	a := &App{}
	a.setupDiagnostic.id = "run"
	a.setupDiagnostic.report = r
	raw, err := a.redactedSetupDiagnostic("run")
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"secret-access", "secret-refresh", "requested-secret", "opaque-secret", "private-id", "private@example.test", "alias@example.test", "secret.example"} {
		if strings.Contains(raw, secret) {
			t.Fatalf("export leaked %s", secret)
		}
	}
	if !strings.Contains(raw, "SMTP.Send") || !strings.Contains(raw, "selected mailbox (redacted)") {
		t.Fatal(raw)
	}
	if _, err = a.redactedSetupDiagnostic("different"); err == nil {
		t.Fatal("exported unrelated run")
	}
	if r.Mailbox != "private@example.test" {
		t.Fatal("export mutated local results")
	}
}
func TestDiagnosticUsesOwnerGrantsWithoutRefreshOrConsent(t *testing.T) {
	ops, parent, child := linkedOAuthOps(t, "https://must-not-refresh.invalid/token")
	if err := ops.credStore.SetOAuthTokens(parent.ID, &credentials.OAuthTokens{Provider: "microsoft", AccessToken: "parent-token", RefreshToken: "refresh-token", ScopesKnown: true, Scopes: []string{"SMTP.Send"}, ScopeResource: "https://outlook.office.com", ExpiresAt: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := ops.credStore.SetOAuthTokens(child.ID, &credentials.OAuthTokens{Provider: "custom", AccessToken: "stale-child-token", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	// Plaintext configurations must never dial, even with valid credentials.
	child.IMAPSecurity = account.SecurityNone
	child.SMTPSecurity = account.SecurityNone
	a := &App{accountStore: ops.accountStore, credStore: ops.credStore, composeOps: *ops}
	r := a.runSetupDiagnostic(context.Background(), child, SetupDiagnosticRequest{RunID: "run", From: child.Email})
	if r.OAuthResource != "https://outlook.office.com" || !r.OAuthScopesKnown {
		t.Fatalf("used child rather than owner grant: %+v", r)
	}
	raw, _ := json.Marshal(r)
	if strings.Contains(string(raw), "parent-token") || strings.Contains(string(raw), "stale-child-token") {
		t.Fatal("token leaked")
	}
	found := false
	for _, c := range r.Checks {
		if strings.Contains(c.Detail, "token expires") {
			found = true
		}
	}
	if !found {
		t.Fatal("expiry guidance missing")
	}
	got, err := ops.credStore.GetOAuthTokens(parent.ID)
	if err != nil || got.AccessToken != "parent-token" {
		t.Fatal("diagnostic changed tokens", err)
	}
}
func TestDiagnosticStartCancelAndNoBackgroundProbe(t *testing.T) {
	ops, _, _ := linkedOAuthOps(t, "https://must-not-refresh.invalid/token")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	accepted := make(chan struct{})
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		c, e := ln.Accept()
		if e != nil {
			return
		}
		defer c.Close()
		close(accepted)
		var b [4096]byte
		for {
			if _, e = c.Read(b[:]); e != nil {
				return
			}
		}
	}()
	acc, err := ops.accountStore.Create(&account.AccountConfig{Name: "Diagnostic", Email: "owned@example.test", DisplayName: "Owned", Username: "owned@example.test", IMAPHost: "127.0.0.1", IMAPPort: ln.Addr().(*net.TCPAddr).Port, IMAPSecurity: account.SecurityTLS, NoOutgoingServer: true})
	if err != nil {
		t.Fatal(err)
	}
	a := &App{ctx: context.Background(), accountStore: ops.accountStore, credStore: ops.credStore, composeOps: *ops}
	select {
	case <-accepted:
		t.Fatal("background execution")
	default:
	}
	request := SetupDiagnosticRequest{RunID: "run", AccountID: acc.ID, From: acc.Email, Route: "imap-smtp"}
	invalid := request
	invalid.From = "not-configured@example.test"
	if _, e := a.RunSetupDiagnostic(invalid); e == nil {
		t.Fatal("accepted arbitrary From")
	}
	result := make(chan *SetupDiagnosticReport, 1)
	fail := make(chan error, 1)
	go func() {
		r, e := a.RunSetupDiagnostic(request)
		if e != nil {
			fail <- e
			return
		}
		result <- r
	}()
	select {
	case <-accepted:
	case e := <-fail:
		t.Fatal(e)
	case <-time.After(3 * time.Second):
		t.Fatal("did not start")
	}
	if _, e := a.RunSetupDiagnostic(request); e == nil {
		t.Fatal("allowed parallel probes")
	}
	a.CancelSetupDiagnostic("wrong-run")
	select {
	case <-closed:
		t.Fatal("wrong cancellation ID cancelled")
	default:
	}
	a.CancelSetupDiagnostic("run")
	select {
	case r := <-result:
		if r.Checks[0].Status != "cancelled" {
			t.Fatalf("%+v", r.Checks)
		}
	case e := <-fail:
		t.Fatal(e)
	case <-time.After(time.Second):
		t.Fatal("cancel not bounded")
	}
	<-closed
	if _, e := a.redactedSetupDiagnostic("run"); e != nil {
		t.Fatal(e)
	}
	a.CancelSetupDiagnostic("early")
	request.RunID = "early"
	r, e := a.RunSetupDiagnostic(request)
	if e != nil {
		t.Fatal(e)
	}
	if r.Checks[0].Status != "cancelled" {
		t.Fatal(fmt.Sprint(r.Checks))
	}
}

func TestDiagnosticExportSavesOnlyRedactedUserChosenFile(t *testing.T) {
	a := &App{}
	a.setupDiagnostic.id = "run"
	a.setupDiagnostic.report = &SetupDiagnosticReport{Mailbox: "private@example.test", From: "alias@example.test", AccountID: "private-id"}
	path := filepath.Join(t.TempDir(), "report.json")
	saved, err := a.exportSetupDiagnostic("run", func() (string, error) { return "", nil })
	if err != nil || saved {
		t.Fatal("cancelled export wrote a file", err)
	}
	saved, err = a.exportSetupDiagnostic("run", func() (string, error) { return path, nil })
	if err != nil || !saved {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "private-id") || strings.Contains(string(raw), "@") {
		t.Fatal("saved private identifiers")
	}
	if !strings.Contains(string(raw), "redacted") {
		t.Fatal("missing redacted evidence")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		t.Fatal("export readable by others")
	}
	called := false
	if _, err = a.exportSetupDiagnostic("wrong", func() (string, error) { called = true; return path, nil }); err == nil || called {
		t.Fatal("opened save dialog for unrelated report")
	}
}
