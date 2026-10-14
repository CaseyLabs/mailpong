package app

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/folder"
	"github.com/caseylabs/mailven/internal/message"
)

var silentEmitOnce sync.Once

// newAPIActionApp returns an App with one Gmail-API account whose route has
// the given state, an Inbox, a Trash and a Spam folder named by their Gmail
// label IDs, and one API message "m1" in the Inbox. The App's context is
// already cancelled, so queued work is recorded but never drained.
func newAPIActionApp(t *testing.T, state string) (*App, string, map[folder.Type]*folder.Folder) {
	t.Helper()
	a, folders := newArchiveTestApp(t)
	// Count goroutines outlive the test, so the silent emitter stays installed.
	silentEmitOnce.Do(func() { eventsEmit = func(context.Context, string, ...interface{}) {} })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a.ctx = ctx

	accs, err := a.accountStore.List()
	if err != nil || len(accs) != 1 {
		t.Fatalf("accounts = %v, %v", accs, err)
	}
	acct := accs[0].ID
	for typ, path := range map[folder.Type]string{folder.TypeInbox: "INBOX", folder.TypeTrash: "TRASH", folder.TypeSpam: "SPAM"} {
		f := &folder.Folder{AccountID: acct, Name: path, Path: path, Type: typ}
		if err := a.folderStore.Create(f); err != nil {
			t.Fatal(err)
		}
		folders[typ] = f
	}
	if err := a.accountStore.SetMailAPIRoute(account.MailAPIRoute{AccountID: acct, Route: account.MailRouteGmailAPI,
		State: state, ClientConfigID: "cfg"}); err != nil {
		t.Fatal(err)
	}
	err = a.messageStore.CommitAPIPage(message.APIPage{AccountID: acct, Provider: message.APIProviderGmail, Scope: "m",
		Cursor: "c", CursorKind: "k", Upserts: []message.APIUpsert{{
			Ref: message.APIRef{RemoteID: "r1", RemoteThreadID: "t1", Containers: []string{"INBOX", "UNREAD"}, Version: "v1"},
			Rows: []message.APIRow{{Message: message.Message{FolderID: folders[folder.TypeInbox].ID,
				MessageID: "<r1@x>", Subject: "s", Date: time.Unix(1, 0).UTC()}}}}}})
	if err != nil {
		t.Fatal(err)
	}
	return a, acct, folders
}

func apiRowID(t *testing.T, a *App, acct string) string {
	t.Helper()
	_, rows, err := a.messageStore.GetAPIRef(acct, message.APIProviderGmail, "r1")
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows = %v, %v", rows, err)
	}
	for _, id := range rows {
		return id
	}
	return ""
}

func TestAPIActionRoutedReadStarAndMoveQueueLocally(t *testing.T) {
	a, acct, folders := newAPIActionApp(t, account.MailRouteStateActive)
	id := apiRowID(t, a, acct)

	if err := a.MarkAsRead([]string{id}); err != nil {
		t.Fatal(err)
	}
	if err := a.Star([]string{id}); err != nil {
		t.Fatal(err)
	}
	if ok, err := a.Trash([]string{id}); err != nil || !ok {
		t.Fatalf("Trash = %v, %v", ok, err)
	}
	m, err := a.messageStore.Get(id)
	if err != nil || m == nil || !m.IsRead || !m.IsStarred || m.FolderID != folders[folder.TypeTrash].ID {
		t.Fatalf("local message = %+v, %v", m, err)
	}
	acts, err := a.messageStore.ListAPIActions(acct)
	if err != nil || len(acts) != 3 {
		t.Fatalf("actions = %+v, %v", acts, err)
	}
	mv := acts[2]
	if mv.Op != message.APIActionMove || !mv.Payload.Trash || mv.Payload.Source != "INBOX" || mv.Payload.Dest != "TRASH" {
		t.Fatalf("move action = %+v", mv)
	}
	if c, _ := a.messageStore.APIRefContainers(acct, message.APIProviderGmail, "r1"); len(c) != 1 || c[0] != "TRASH" {
		t.Fatalf("containers = %v", c)
	}
}

func TestAPIActionUnsupportedOperationsFailExplicitly(t *testing.T) {
	a, acct, folders := newAPIActionApp(t, account.MailRouteStateActive)
	id := apiRowID(t, a, acct)
	if err := a.DeletePermanently([]string{id}); err == nil || !strings.Contains(err.Error(), "mail API") {
		t.Fatalf("DeletePermanently = %v", err)
	}
	if err := a.CopyToFolder([]string{id}, folders[folder.TypeSpam].ID); err == nil || !strings.Contains(err.Error(), "mail API") {
		t.Fatalf("CopyToFolder = %v", err)
	}
	if m, _ := a.messageStore.Get(id); m == nil {
		t.Fatal("rejected delete removed the message")
	}
	if acts, _ := a.messageStore.ListAPIActions(acct); len(acts) != 0 {
		t.Fatalf("unsupported operations queued %d actions", len(acts))
	}
}

func TestAPIActionStagedRouteStaysOnIMAP(t *testing.T) {
	a, acct, _ := newAPIActionApp(t, account.MailRouteStateStaged)
	if a.apiRouted(acct) != nil {
		t.Fatal("a staged route must not take over actions")
	}
	if msgs, other := a.splitAPIMessages(nil); msgs != nil || other != nil {
		t.Fatal("empty split")
	}
	id := apiRowID(t, a, acct)
	m, _ := a.messageStore.Get(id)
	api, other := a.splitAPIMessages([]*message.Message{m})
	if len(api) != 0 || len(other) != 1 {
		t.Fatalf("split = %v, %d", api, len(other))
	}
}

func TestAPIMoveBetweenAccountsKeepsExplicitError(t *testing.T) {
	a, acct, folders := newAPIActionApp(t, account.MailRouteStateActive)
	other := &account.AccountConfig{Name: "o", DisplayName: "o", Email: "o@example.com",
		IMAPHost: "h", IMAPPort: 993, IMAPSecurity: account.SecurityTLS,
		SMTPHost: "h", SMTPPort: 587, SMTPSecurity: account.SecurityStartTLS, AuthType: account.AuthPassword, Username: "o"}
	oa, err := a.accountStore.Create(other)
	if err != nil {
		t.Fatal(err)
	}
	dest := &folder.Folder{AccountID: oa.ID, Name: "INBOX", Path: "INBOX", Type: folder.TypeInbox}
	if err := a.folderStore.Create(dest); err != nil {
		t.Fatal(err)
	}
	_ = folders
	err = a.MoveToFolder([]string{apiRowID(t, a, acct)}, dest.ID)
	if err == nil || !strings.Contains(err.Error(), "between accounts is not available") {
		t.Fatalf("MoveToFolder = %v", err)
	}
}
