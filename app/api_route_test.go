package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/folder"
	"github.com/caseylabs/mailven/internal/message"
	"github.com/caseylabs/mailven/internal/oauth2"
	"github.com/caseylabs/mailven/internal/sync"
)

// routeGmail serves the Gmail read endpoints for one INBOX message. fail makes
// every request answer 500, as a provider outage during activation.
type routeGmail struct {
	mu   gosync.Mutex
	fail bool
	// raw replaces the served message source; gone answers 404 for it.
	raw  []byte
	gone bool
}

func (g *routeGmail) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.fail {
		w.WriteHeader(500)
		return
	}
	out := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	switch p := r.URL.Path; {
	case p == "/profile":
		out(map[string]string{"historyId": "100"})
	case p == "/labels":
		out(map[string]any{"labels": []map[string]string{{"id": "INBOX", "name": "INBOX", "type": "system"}}})
	case p == "/messages":
		out(map[string]any{"messages": []map[string]string{{"id": "m1"}}})
	case p == "/history":
		out(map[string]any{"historyId": "100"})
	default:
		if g.gone {
			w.WriteHeader(404)
			return
		}
		raw := "From: S <s@example.com>\r\nTo: user@example.test\r\nSubject: api\r\nMessage-ID: <m1@example.com>\r\nDate: Mon, 02 Jan 2026 10:00:00 +0000\r\n\r\nbody\r\n"
		if g.raw != nil {
			raw = string(g.raw)
		}
		out(map[string]any{"id": "m1", "threadId": "T1", "labelIds": []string{"INBOX"}, "historyId": "50",
			"raw": base64.RawURLEncoding.EncodeToString([]byte(raw))})
	}
}

type routeRig struct {
	*mailAPIFixture
	app    *App
	gmail  *routeGmail
	imapID string // the IMAP INBOX folder
}

// newRouteRig stages a Gmail API route on an account with an IMAP INBOX that
// holds one IMAP message.
func newRouteRig(t *testing.T) *routeRig {
	t.Helper()
	fx := newMailAPIFixture(t, "google", "https://issuer.example.test/token")
	stageGmailRoute(t, fx, time.Now().Add(time.Hour))
	silentEmitOnce.Do(func() { eventsEmit = func(context.Context, string, ...interface{}) {} })
	gm := &routeGmail{}
	srv := httptest.NewServer(gm)
	t.Cleanup(srv.Close)
	a := &App{
		ctx: context.Background(), accountStore: fx.ops.accountStore, credStore: fx.ops.credStore,
		oauth2Manager: fx.ops.oauth2Manager, folderStore: folder.NewStore(fx.db), messageStore: message.NewStore(fx.db),
	}
	a.syncEngine = sync.NewEngine(nil, a.accountStore, a.folderStore, a.messageStore, message.NewAttachmentStore(fx.db))
	a.apiRoutes.provider = fx.ops.baseProvider
	a.apiRoutes.gmailBase = srv.URL
	a.apiRoutes.sleep = func(context.Context, time.Duration) error { return nil }
	f := &folder.Folder{AccountID: fx.acc.ID, Name: "INBOX", Path: "INBOX", Type: folder.TypeInbox}
	if err := a.folderStore.Create(f); err != nil {
		t.Fatal(err)
	}
	if err := a.messageStore.Create(&message.Message{ID: "imap-1", AccountID: fx.acc.ID, FolderID: f.ID, UID: 7, Subject: "imap"}); err != nil {
		t.Fatal(err)
	}
	return &routeRig{mailAPIFixture: fx, app: a, gmail: gm, imapID: f.ID}
}

func (r *routeRig) state(t *testing.T) string {
	t.Helper()
	route, err := r.app.accountStore.GetMailAPIRoute(r.acc.ID)
	if err != nil || route == nil {
		t.Fatalf("route = %v, %v", route, err)
	}
	return route.State
}

// visibleSubjects lists the subjects the IMAP views would show for the account.
func (r *routeRig) visibleSubjects(t *testing.T) []string {
	t.Helper()
	var out []string
	folders, err := r.app.folderStore.List(r.acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range folders {
		hs, err := r.app.messageStore.ListByFolder(f.ID, 0, 100)
		if err != nil {
			t.Fatal(err)
		}
		for _, h := range hs {
			out = append(out, h.Subject)
		}
	}
	return out
}

func TestMailAPIRouteActivationAndDeactivationSwapRows(t *testing.T) {
	r := newRouteRig(t)
	a := r.app
	if got := r.visibleSubjects(t); len(got) != 1 || got[0] != "imap" {
		t.Fatalf("before = %v", got)
	}
	if err := a.ActivateMailAPIRoute(r.acc.ID); err != nil {
		t.Fatal(err)
	}
	if r.state(t) != account.MailRouteStateActive {
		t.Fatal("route not active")
	}
	if got := r.visibleSubjects(t); len(got) != 1 || got[0] != "api" {
		t.Fatalf("active views = %v; want only the API row", got)
	}
	if err := a.DeactivateMailAPIRoute(r.acc.ID); err != nil {
		t.Fatal(err)
	}
	if r.state(t) != account.MailRouteStateStaged {
		t.Fatal("route not staged after deactivation")
	}
	if got := r.visibleSubjects(t); len(got) != 1 || got[0] != "imap" {
		t.Fatalf("deactivated views = %v; want only the IMAP row", got)
	}
	// API rows survive hidden and return on reactivation without a refetch.
	r.gmail.mu.Lock()
	r.gmail.fail = true
	r.gmail.mu.Unlock()
	if err := a.ActivateMailAPIRoute(r.acc.ID); err == nil {
		t.Fatal("reactivation with a failing provider succeeded")
	}
	if got := r.visibleSubjects(t); len(got) != 1 || got[0] != "imap" {
		t.Fatalf("after failed reactivation = %v", got)
	}
}

func TestMailAPIRouteActivationFailureStaysOnIMAP(t *testing.T) {
	r := newRouteRig(t)
	r.gmail.fail = true
	err := r.app.ActivateMailAPIRoute(r.acc.ID)
	if err == nil || !strings.Contains(err.Error(), "stays on IMAP") {
		t.Fatalf("Activate = %v", err)
	}
	if r.state(t) != account.MailRouteStateStaged {
		t.Fatal("failed activation left the route active")
	}
	if got := r.visibleSubjects(t); len(got) != 1 || got[0] != "imap" {
		t.Fatalf("views = %v; IMAP data must be intact", got)
	}
	if r.app.apiRouted(r.acc.ID) != nil {
		t.Fatal("account still routed through the API")
	}
}

func TestMailAPIRouteActivationPreconditions(t *testing.T) {
	t.Run("grant must pass the gates", func(t *testing.T) {
		r := newRouteRig(t)
		route, _ := r.app.accountStore.GetMailAPIRoute(r.acc.ID)
		if err := r.app.credStore.ClearOAuthRefreshTokenForClientConfig(r.acc.ID, route.ClientConfigID); err != nil {
			t.Fatal(err)
		}
		if err := r.app.ActivateMailAPIRoute(r.acc.ID); err == nil || !strings.Contains(err.Error(), "reauthorize") {
			t.Fatalf("Activate = %v", err)
		}
		if r.state(t) != account.MailRouteStateStaged {
			t.Fatal("route changed")
		}
	})
	t.Run("IMAP sync in flight", func(t *testing.T) {
		r := newRouteRig(t)
		r.app.syncContexts = map[string]*syncSlot{r.acc.ID + ":" + r.imapID: {}}
		if err := r.app.ActivateMailAPIRoute(r.acc.ID); err == nil || !strings.Contains(err.Error(), "IMAP sync") {
			t.Fatalf("Activate = %v", err)
		}
		if r.state(t) != account.MailRouteStateStaged || len(r.visibleSubjects(t)) != 1 {
			t.Fatal("blocked activation changed state")
		}
	})
	t.Run("queue action in flight", func(t *testing.T) {
		r := newRouteRig(t)
		queueAPIFlagAction(t, r, message.APIActionInFlight)
		if err := r.app.ActivateMailAPIRoute(r.acc.ID); err == nil || !strings.Contains(err.Error(), "in flight") {
			t.Fatalf("Activate = %v", err)
		}
	})
	t.Run("shared route", func(t *testing.T) {
		r := newRouteRig(t)
		route, _ := r.app.accountStore.GetMailAPIRoute(r.acc.ID)
		route.Shared = true
		if err := r.app.accountStore.SetMailAPIRoute(*route); err != nil {
			t.Fatal(err)
		}
		if err := r.app.ActivateMailAPIRoute(r.acc.ID); err == nil {
			t.Fatal("shared route activated")
		}
	})
	t.Run("no route and already active", func(t *testing.T) {
		r := newRouteRig(t)
		if err := r.app.ActivateMailAPIRoute("missing"); err == nil {
			t.Fatal("activated an account with no route")
		}
		if err := r.app.ActivateMailAPIRoute(r.acc.ID); err != nil {
			t.Fatal(err)
		}
		if err := r.app.ActivateMailAPIRoute(r.acc.ID); err == nil {
			t.Fatal("double activation succeeded")
		}
	})
}

// queueAPIFlagAction queues a read-flag action on an API row and sets its status.
func queueAPIFlagAction(t *testing.T, r *routeRig, status string) *message.APIAction {
	t.Helper()
	a := r.app
	inbox := &folder.Folder{AccountID: r.acc.ID, Name: "api-inbox", Path: "api-inbox", Type: folder.Type("")}
	if err := a.folderStore.Create(inbox); err != nil {
		t.Fatal(err)
	}
	err := a.messageStore.CommitAPIPage(message.APIPage{AccountID: r.acc.ID, Provider: message.APIProviderGmail, Scope: "m",
		Cursor: "c", CursorKind: "k", Upserts: []message.APIUpsert{{
			Ref:  message.APIRef{RemoteID: "q1", RemoteThreadID: "t", Containers: []string{"INBOX"}, Version: "v"},
			Rows: []message.APIRow{{Message: message.Message{FolderID: inbox.ID, MessageID: "<q1@x>", Subject: "q", Date: time.Unix(1, 0).UTC()}}}}}})
	if err != nil {
		t.Fatal(err)
	}
	_, rows, err := a.messageStore.GetAPIRef(r.acc.ID, message.APIProviderGmail, "q1")
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows = %v, %v", rows, err)
	}
	var id string
	for _, v := range rows {
		id = v
	}
	if err := a.messageStore.EnqueueAPIFlag(r.acc.ID, message.APIProviderGmail, message.APIActionRead, []string{id}, true); err != nil {
		t.Fatal(err)
	}
	acts, err := a.messageStore.ListAPIActions(r.acc.ID)
	if err != nil || len(acts) != 1 {
		t.Fatalf("actions = %v, %v", acts, err)
	}
	if status != message.APIActionPending {
		if err := a.messageStore.SetAPIActionStatus(acts[0].ID, status, ""); err != nil {
			t.Fatal(err)
		}
	}
	return acts[0]
}

func TestMailAPIRouteDeactivationMidQueueKeepsActions(t *testing.T) {
	r := newRouteRig(t)
	if err := r.app.ActivateMailAPIRoute(r.acc.ID); err != nil {
		t.Fatal(err)
	}
	act := queueAPIFlagAction(t, r, message.APIActionPending)
	if err := r.app.DeactivateMailAPIRoute(r.acc.ID); err != nil {
		t.Fatal(err)
	}
	acts, err := r.app.messageStore.ListAPIActions(r.acc.ID, message.APIActionPending)
	if err != nil || len(acts) != 1 || acts[0].ID != act.ID {
		t.Fatalf("pending after deactivation = %v, %v", acts, err)
	}
	if r.app.apiActionWorker().Active(r.acc.ID) {
		t.Fatal("worker would drain a deactivated account")
	}
}

func TestMailAPIRouteClearDeactivatesFirst(t *testing.T) {
	r := newRouteRig(t)
	if err := r.app.ActivateMailAPIRoute(r.acc.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.app.ClearMailAPIRoute(r.acc.ID); err != nil {
		t.Fatal(err)
	}
	if route, _ := r.app.accountStore.GetMailAPIRoute(r.acc.ID); route != nil {
		t.Fatalf("route = %+v", route)
	}
	if got := r.visibleSubjects(t); len(got) != 1 || got[0] != "imap" {
		t.Fatalf("views after clear = %v", got)
	}
}

func TestMailAPIRoutePausesOnReauthWithoutIMAPFallback(t *testing.T) {
	r := newRouteRig(t)
	if err := r.app.ActivateMailAPIRoute(r.acc.ID); err != nil {
		t.Fatal(err)
	}
	queueAPIFlagAction(t, r, message.APIActionPending)
	route, _ := r.app.accountStore.GetMailAPIRoute(r.acc.ID)
	if err := r.app.credStore.ClearOAuthRefreshTokenForClientConfig(r.acc.ID, route.ClientConfigID); err != nil {
		t.Fatal(err)
	}
	acc, _ := r.app.accountStore.Get(r.acc.ID)
	handled, err := r.app.syncAPIAccount(context.Background(), acc)
	if !handled || err != errAPIPaused {
		t.Fatalf("syncAPIAccount = %v, %v; want handled with pause (never IMAP)", handled, err)
	}
	if r.app.apiActionWorker().Active(r.acc.ID) {
		t.Fatal("queue keeps draining while paused")
	}
	if got := r.visibleSubjects(t); len(got) < 1 || strings.Contains(strings.Join(got, ","), "imap") {
		t.Fatalf("local API data changed or IMAP rows leaked: %v", got)
	}
	if r.state(t) != account.MailRouteStateActive {
		t.Fatal("pause must not deactivate the route")
	}
}

func TestMailAPIRouteAPISyncHookIgnoresUnroutedAccounts(t *testing.T) {
	r := newRouteRig(t)
	acc, _ := r.app.accountStore.Get(r.acc.ID)
	if handled, err := r.app.syncAPIAccount(context.Background(), acc); handled || err != nil {
		t.Fatalf("staged route handled = %v, %v", handled, err)
	}
}

func TestMailAPIRouteReconsentKeepsActive(t *testing.T) {
	r := newRouteRig(t)
	if err := r.app.ActivateMailAPIRoute(r.acc.ID); err != nil {
		t.Fatal(err)
	}
	c := r.consent(t, false)
	resp := &oauth2.TokenResponse{AccessToken: "n", RefreshToken: "r2", ExpiresIn: 3600, Scope: gmailScopes}
	if err := r.ops.finish(c, resp, "user@example.test"); err != nil {
		t.Fatal(err)
	}
	if r.state(t) != account.MailRouteStateActive {
		t.Fatal("reconsent flipped an active route back to staged")
	}
}

func TestMailAPIRouteSyncFolderBypassesIMAP(t *testing.T) {
	r := newRouteRig(t)
	if err := r.app.ActivateMailAPIRoute(r.acc.ID); err != nil {
		t.Fatal(err)
	}
	// imapPool/syncEngine IMAP paths would panic or dial; the routed branch must not reach them.
	if err := r.app.SyncFolder(r.acc.ID, "any"); err != nil {
		t.Fatal(err)
	}
	if err := r.app.SyncFolders(r.acc.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.app.SyncAccountComplete(r.acc.ID); err != nil {
		t.Fatal(err)
	}
}

func TestSendMessageOnActiveAPIRouteNeverFallsBackToSMTP(t *testing.T) {
	f, server, _ := initAppSendFixture(t, appSMTPConfig{finalReply: "250 2.0.0 queued"})
	err := f.ops.accountStore.SetMailAPIRoute(account.MailAPIRoute{
		AccountID: f.accountID, Route: account.MailRouteGmailAPI, State: account.MailRouteStateActive,
		ClientConfigID: "shipped",
	})
	if err != nil {
		t.Fatal(err)
	}
	// No API tokens exist, so the API send must fail rather than use SMTP.
	_, err = f.ops.sendMessage(context.Background(), f.accountID, appSendMessage(), f.draft)
	if err == nil || !strings.Contains(err.Error(), "via API") {
		t.Fatalf("err = %v; want an API send failure", err)
	}
	assertAppSMTPNeverSubmitted(t, server)
	assertDraftRemains(t, f)
}
