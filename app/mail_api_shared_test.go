package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/apiactions"
	"github.com/caseylabs/mailven/internal/apimigrate"
	"github.com/caseylabs/mailven/internal/credentials"
	"github.com/caseylabs/mailven/internal/folder"
	"github.com/caseylabs/mailven/internal/gmailapi"
	"github.com/caseylabs/mailven/internal/imap"
	"github.com/caseylabs/mailven/internal/message"
	"github.com/caseylabs/mailven/internal/oauth2"
	"github.com/caseylabs/mailven/internal/smtp"
	"github.com/caseylabs/mailven/internal/submission"
)

const (
	graphOwnScopes    = "https://graph.microsoft.com/Mail.ReadWrite https://graph.microsoft.com/Mail.Send offline_access openid email"
	graphSharedScopes = graphOwnScopes + " https://graph.microsoft.com/Mail.ReadWrite.Shared https://graph.microsoft.com/Mail.Send.Shared"
)

// newSharedFixture is a Microsoft work/school parent with one linked child.
func newSharedFixture(t *testing.T, tokenURL string) (*mailAPIFixture, *account.Account) {
	t.Helper()
	fx := newMailAPIFixture(t, "microsoft", tokenURL)
	tok := &credentials.OAuthTokens{Provider: "microsoft", AccessToken: "proto-access", RefreshToken: "proto-refresh",
		ExpiresAt: time.Now().Add(time.Hour), MicrosoftAccountType: "business"}
	if err := fx.ops.credStore.SetOAuthTokens(fx.acc.ID, tok); err != nil {
		t.Fatal(err)
	}
	child, err := fx.ops.accountStore.Create(&account.AccountConfig{
		Name: "Shared", DisplayName: "Shared", Email: "Shared@Example.Test", Username: "shared@example.test",
		IMAPHost: "i", IMAPPort: 993, SMTPHost: "s", SMTPPort: 587,
		AuthType: account.AuthOAuth2, SharedMailboxParentID: fx.acc.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return fx, child
}

func (fx *mailAPIFixture) stageChild(t *testing.T, child *account.Account, scopes string) error {
	t.Helper()
	c, err := fx.ops.prepare(child.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	resp := &oauth2.TokenResponse{AccessToken: "api-access", RefreshToken: "api-refresh", ExpiresIn: 3600, Scope: scopes}
	return fx.ops.finish(c, resp, "user@example.test")
}

func TestSharedConsentStagesChildUnderOwnerGrant(t *testing.T) {
	fx, child := newSharedFixture(t, "https://issuer.example.test/token")
	if err := fx.stageChild(t, child, graphSharedScopes); err != nil {
		t.Fatal(err)
	}
	owner, _ := fx.ops.accountStore.GetMailAPIRoute(fx.acc.ID)
	if owner == nil || owner.Shared || owner.Linked() || owner.State != account.MailRouteStateStaged {
		t.Fatalf("owner route must stay an own /me route: %+v", owner)
	}
	route, _ := fx.ops.accountStore.GetMailAPIRoute(child.ID)
	if route == nil || !route.Shared || route.MailboxAddress != "shared@example.test" || route.GrantAccountID != fx.acc.ID || route.State != account.MailRouteStateStaged {
		t.Fatalf("child route = %+v", route)
	}
	if _, err := fx.ops.credStore.GetOAuthTokensForClientConfig(child.ID, route.ClientConfigID); err == nil {
		t.Fatal("child holds a copy of the API grant")
	}
	for _, id := range []string{fx.acc.ID, child.ID} {
		if st, err := fx.ops.status(id); err != nil || !st.Authorized {
			t.Fatalf("status(%s) = %+v, %v", id, st, err)
		}
	}
	// Clearing the owner is refused while the child references its grant; the
	// child's own clear leaves the owner's grant alone.
	if err := fx.ops.clear(fx.acc.ID); err == nil {
		t.Fatal("owner grant cleared under a dependent child")
	}
	if err := fx.ops.clear(child.ID); err != nil {
		t.Fatal(err)
	}
	if st, err := fx.ops.status(fx.acc.ID); err != nil || !st.Authorized {
		t.Fatalf("owner grant lost with the child: %+v %v", st, err)
	}
	if err := fx.ops.clear(fx.acc.ID); err != nil {
		t.Fatal(err)
	}
}

func TestSharedConsentRejections(t *testing.T) {
	fx, child := newSharedFixture(t, "https://issuer.example.test/token")
	if _, err := fx.ops.prepare(child.ID, false); err == nil {
		t.Fatal("child without shared mode accepted")
	}
	if err := fx.stageChild(t, child, graphOwnScopes); err == nil {
		t.Fatal("grant without Shared scopes accepted")
	}
	if r, _ := fx.ops.accountStore.GetMailAPIRoute(child.ID); r != nil {
		t.Fatalf("route stored after partial consent: %+v", r)
	}
	if r, _ := fx.ops.accountStore.GetMailAPIRoute(fx.acc.ID); r != nil {
		t.Fatalf("owner route stored after partial consent: %+v", r)
	}
	for _, bad := range []string{"A Name <shared@example.test>", "../x@example.test", "a/b@example.test", "shared@example.test?x=1", "not-an-address", "a%2fb@example.test"} {
		if validateMailboxTarget(bad) == nil {
			t.Errorf("target %q accepted", bad)
		}
	}
	nested, err := fx.ops.accountStore.Create(&account.AccountConfig{
		Name: "N", DisplayName: "N", Email: "nested@example.test", Username: "n",
		IMAPHost: "i", IMAPPort: 993, SMTPHost: "s", SMTPPort: 587, AuthType: account.AuthOAuth2, SharedMailboxParentID: child.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fx.ops.prepare(nested.ID, true); err == nil {
		t.Fatal("nested link accepted")
	}
	gmail := newMailAPIFixture(t, "google", "https://issuer.example.test/token")
	gchild, _ := gmail.ops.accountStore.Create(&account.AccountConfig{
		Name: "G", DisplayName: "G", Email: "g@example.test", Username: "g",
		IMAPHost: "i", IMAPPort: 993, SMTPHost: "s", SMTPPort: 587, AuthType: account.AuthOAuth2, SharedMailboxParentID: gmail.acc.ID,
	})
	if _, err := gmail.ops.prepare(gchild.ID, true); err == nil {
		t.Fatal("Gmail delegation accepted")
	}
}

func TestSharedOwnerReauthKeepsSharedScopes(t *testing.T) {
	fx, child := newSharedFixture(t, "https://issuer.example.test/token")
	if err := fx.stageChild(t, child, graphSharedScopes); err != nil {
		t.Fatal(err)
	}
	c, err := fx.ops.prepare(fx.acc.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	resp := &oauth2.TokenResponse{AccessToken: "x", RefreshToken: "y", ExpiresIn: 3600, Scope: graphOwnScopes}
	if err := fx.ops.finish(c, resp, "user@example.test"); err == nil {
		t.Fatal("owner reauthorization dropped Shared scopes under a child")
	}
	if st, _ := fx.ops.status(child.ID); !st.Authorized {
		t.Fatalf("child lost authorization: %+v", st)
	}
	c, _ = fx.ops.prepare(fx.acc.ID, true)
	if err := fx.ops.finish(c, resp, "user@example.test"); err == nil {
		t.Fatal("grant without Shared scopes accepted")
	}
}

func TestSharedRouteFailsClosedOnBrokenOwnerLink(t *testing.T) {
	fx, child := newSharedFixture(t, "https://issuer.example.test/token")
	if err := fx.stageChild(t, child, graphSharedScopes); err != nil {
		t.Fatal(err)
	}
	db := fx.db
	for name, mutate := range map[string]struct{ do, undo string }{
		"address changed":  {`UPDATE accounts SET email='other@example.test' WHERE id='` + child.ID + `'`, `UPDATE accounts SET email='shared@example.test' WHERE id='` + child.ID + `'`},
		"link removed":     {`UPDATE accounts SET shared_mailbox_parent_id=NULL WHERE id='` + child.ID + `'`, `UPDATE accounts SET shared_mailbox_parent_id='` + fx.acc.ID + `' WHERE id='` + child.ID + `'`},
		"owner route gone": {`DELETE FROM account_mail_api_routes WHERE account_id='` + fx.acc.ID + `'`, ``},
	} {
		if _, err := db.Exec(mutate.do); err != nil {
			t.Fatal(err)
		}
		if _, err := fx.ops.status(child.ID); err == nil {
			t.Errorf("%s: status succeeded", name)
		}
		if _, err := fx.ops.validToken(child.ID); err == nil {
			t.Errorf("%s: token issued", name)
		}
		if mutate.undo != "" {
			if _, err := db.Exec(mutate.undo); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestSharedRefreshRotatesOwnerSlotOnce(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_ = r.ParseForm()
		if r.Form.Get("refresh_token") != "api-refresh" {
			http.Error(w, "bad", 400)
			return
		}
		time.Sleep(50 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "fresh", "expires_in": 3600, "token_type": "Bearer"})
	}))
	defer srv.Close()
	fx, child := newSharedFixture(t, srv.URL)
	if err := fx.stageChild(t, child, graphSharedScopes); err != nil {
		t.Fatal(err)
	}
	slot := oauth2.MicrosoftMailAPIClientConfigID
	if err := fx.ops.credStore.UpdateOAuthTokensForClientConfig(fx.acc.ID, slot, "old", "api-refresh", time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for _, id := range []string{fx.acc.ID, child.ID, child.ID, fx.acc.ID} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if tok, err := fx.ops.validToken(id); err != nil || tok.AccessToken != "fresh" {
				t.Errorf("token for %s = %+v, %v", id, tok, err)
			}
		}()
	}
	wg.Wait()
	if n := calls.Load(); n != 1 {
		t.Fatalf("refresh requests = %d, want 1", n)
	}
	if _, err := fx.ops.credStore.GetOAuthTokensForClientConfig(child.ID, slot); err == nil {
		t.Fatal("refresh wrote a child token copy")
	}
}

func TestSharedRejectedGrantPausesOwnerAndChildren(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer srv.Close()
	fx, child := newSharedFixture(t, srv.URL)
	if err := fx.stageChild(t, child, graphSharedScopes); err != nil {
		t.Fatal(err)
	}
	slot := oauth2.MicrosoftMailAPIClientConfigID
	_ = fx.ops.credStore.UpdateOAuthTokensForClientConfig(fx.acc.ID, slot, "old", "api-refresh", time.Now().Add(-time.Minute))
	var ids []string
	fx.ops.emit = func(event string, data any) {
		if m, ok := data.(map[string]any); ok && event == "mail-api:reauth-required" {
			fx.mu.Lock()
			ids = append(ids, m["accountId"].(string))
			fx.mu.Unlock()
		}
	}
	if _, err := fx.ops.validToken(child.ID); err == nil {
		t.Fatal("expected refresh failure")
	}
	if len(ids) != 2 {
		t.Fatalf("reauth events for %v, want owner and child", ids)
	}
	for _, id := range []string{fx.acc.ID, child.ID} {
		if st, _ := fx.ops.status(id); st.Authorized || st.Failure != "reauthorize" {
			t.Errorf("status(%s) = %+v", id, st)
		}
	}
	if r, _ := fx.ops.accountStore.GetMailAPIRoute(child.ID); r == nil {
		t.Fatal("child route removed by owner rejection")
	}
}

// graphLog records the requests a fake Graph receives.
type graphLog struct {
	mu    sync.Mutex
	paths []string
	auth  []string
}

func (g *graphLog) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	g.paths = append(g.paths, r.URL.EscapedPath())
	g.auth = append(g.auth, r.Header.Get("Authorization"))
	g.mu.Unlock()
	if r.Method == http.MethodPost {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	_, _ = w.Write([]byte("From: s@example.com\r\nSubject: shared\r\n\r\nbody\r\n"))
}

func (g *graphLog) last() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.paths) == 0 {
		return ""
	}
	return g.paths[len(g.paths)-1]
}

func newSharedAppRig(t *testing.T) (*mailAPIFixture, *account.Account, *App, *graphLog, *httptest.Server) {
	t.Helper()
	fx, child := newSharedFixture(t, "https://issuer.example.test/token")
	if err := fx.stageChild(t, child, graphSharedScopes); err != nil {
		t.Fatal(err)
	}
	gl := &graphLog{}
	srv := httptest.NewServer(gl)
	t.Cleanup(srv.Close)
	silentEmitOnce.Do(func() { eventsEmit = func(context.Context, string, ...interface{}) {} })
	a := &App{ctx: context.Background(), accountStore: fx.ops.accountStore, credStore: fx.ops.credStore,
		oauth2Manager: fx.ops.oauth2Manager, folderStore: folder.NewStore(fx.db), messageStore: message.NewStore(fx.db)}
	a.apiRoutes.provider = fx.ops.baseProvider
	a.apiRoutes.graphBase = srv.URL
	a.apiRoutes.http = redirectClient(srv)
	return fx, child, a, gl, srv
}

func TestSharedReadAndActionsAddressTheSharedMailbox(t *testing.T) {
	fx, child, a, gl, _ := newSharedAppRig(t)
	if _, err := fx.db.Exec(`UPDATE account_mail_api_routes SET state='active' WHERE account_id=?`, child.ID); err != nil {
		t.Fatal(err)
	}
	f := &folder.Folder{AccountID: child.ID, Name: "INBOX", Path: "INBOX", Type: folder.TypeInbox}
	if err := a.folderStore.Create(f); err != nil {
		t.Fatal(err)
	}
	m := &message.Message{ID: "m-1", AccountID: child.ID, FolderID: f.ID, UID: 1, Subject: "s"}
	if err := a.messageStore.Create(m); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO api_message_refs(account_id,provider,remote_id) VALUES('` + child.ID + `','graph','r-1')`,
		`INSERT INTO api_message_rows(message_id,account_id,provider,remote_id) VALUES('m-1','` + child.ID + `','graph','r-1')`,
	} {
		if _, err := fx.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	route := a.apiRouted(child.ID)
	if route == nil {
		t.Fatal("child route not active")
	}
	if _, err := a.fetchAPIRaw(context.Background(), route, m); err != nil {
		t.Fatal(err)
	}
	if got := gl.last(); got != "/users/shared@example.test/messages/r-1/$value" && got != "/users/shared%40example.test/messages/r-1/$value" {
		t.Fatalf("read path = %q", got)
	}
	remote, err := a.apiRemote(child.ID, message.APIProviderGraph)
	if err != nil {
		t.Fatal(err)
	}
	g := remote.(*apiactions.Graph)
	if g.Writer.Shared != "shared@example.test" || g.Reader.Shared != "shared@example.test" {
		t.Fatalf("clients not bound to the shared mailbox: %q %q", g.Writer.Shared, g.Reader.Shared)
	}
	// A changed address fails closed: no request, no remote.
	n := len(gl.paths)
	if _, err := fx.db.Exec(`UPDATE accounts SET email='other@example.test' WHERE id=?`, child.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.apiRemote(child.ID, message.APIProviderGraph); err == nil {
		t.Fatal("remote built for a changed address")
	}
	if _, err := a.fetchAPIRaw(context.Background(), route, m); err == nil {
		t.Fatal("read allowed for a changed address")
	}
	if len(gl.paths) != n {
		t.Fatalf("requests sent for an invalid route: %v", gl.paths[n:])
	}
}

func TestSharedSendUsesMailboxAndOwnerGrant(t *testing.T) {
	fx, child, _, gl, srv := newSharedAppRig(t)
	store := submission.NewStore(fx.db)
	send := func(from string) error {
		a, err := store.Begin(child.ID, "draft-"+from, smtp.ComposeMessage{Subject: "s " + from})
		if err != nil {
			t.Fatal(err)
		}
		req := submission.Request{From: from, Recipients: []string{"to@example.test"},
			Raw: []byte("Date: Mon, 02 Jan 2006 15:04:05 +0000\r\nMessage-ID: <id@example.test>\r\nFrom: " + from + "\r\nTo: to@example.test\r\nSubject: s\r\n\r\nbody\r\n")}
		_, err = fx.ops.submitViaAPI(context.Background(), store, gmailapi.NewStore(fx.db.DB), redirectClient(srv), a, child, req)
		return err
	}
	if err := send("shared@example.test"); err != nil {
		t.Fatal(err)
	}
	if got := gl.last(); got != "/v1.0/users/shared@example.test/sendMail" && got != "/v1.0/users/shared%40example.test/sendMail" {
		t.Fatalf("send path = %q", got)
	}
	n := len(gl.paths)
	for _, from := range []string{"user@example.test", "someone@example.test"} {
		if err := send(from); err == nil {
			t.Errorf("shared child sent as %s", from)
		}
	}
	if len(gl.paths) != n {
		t.Fatalf("rejected sends reached Graph: %v", gl.paths[n:])
	}
}

func TestRemoveAccountKeepsParentWhenChildRemovalFails(t *testing.T) {
	fx, child, a, _, _ := newSharedAppRig(t)
	a.db = fx.db
	a.imapPool = imap.NewPool(imap.PoolConfig{}, nil)
	if _, err := fx.db.Exec(`CREATE TRIGGER block_child BEFORE DELETE ON accounts WHEN old.id='` + child.ID + `'
		BEGIN SELECT RAISE(ABORT, 'blocked'); END`); err != nil {
		t.Fatal(err)
	}
	if err := a.RemoveAccount(fx.acc.ID); err == nil {
		t.Fatal("parent removed although a child could not be")
	}
	if _, err := a.accountStore.Get(fx.acc.ID); err != nil {
		t.Fatalf("parent deleted: %v", err)
	}
	if _, err := fx.ops.credStore.GetOAuthTokensForClientConfig(fx.acc.ID, oauth2.MicrosoftMailAPIClientConfigID); err != nil {
		t.Fatalf("parent grant lost: %v", err)
	}
	if st, err := fx.ops.status(child.ID); err != nil || !st.Authorized {
		t.Fatalf("child orphaned: %+v %v", st, err)
	}
	if _, err := fx.db.Exec(`DROP TRIGGER block_child`); err != nil {
		t.Fatal(err)
	}
	if err := a.RemoveAccount(fx.acc.ID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{fx.acc.ID, child.ID} {
		if _, err := a.accountStore.Get(id); err == nil {
			t.Fatalf("account %s survived removal", id)
		}
	}
}

func TestSharedActivationAndMigrationStayClosedWithoutLiveEvidence(t *testing.T) {
	_, child, a, gl, _ := newSharedAppRig(t)
	if sharedMailboxLiveVerified {
		t.Fatal("live gate must default closed")
	}
	if err := a.activateAPIRoute(child.ID); err == nil {
		t.Fatal("shared route activated without recorded live evidence")
	}
	if r, _ := a.accountStore.GetMailAPIRoute(child.ID); r.State != account.MailRouteStateStaged {
		t.Fatalf("state = %q", r.State)
	}
	route, _ := a.accountStore.GetMailAPIRoute(child.ID)
	blockers := a.migrationBlockers(route, apimigrate.Counts{}, 0)
	found := false
	for _, b := range blockers {
		found = found || strings.Contains(b, "live Graph checks")
	}
	if !found {
		t.Fatalf("blockers = %v", blockers)
	}
	if len(gl.paths) != 0 {
		t.Fatalf("closed gate still sent requests: %v", gl.paths)
	}
}
