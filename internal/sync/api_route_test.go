package sync

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/database"
	"github.com/caseylabs/mailven/internal/folder"
	"github.com/caseylabs/mailven/internal/message"
)

func newAPISyncScheduler(t *testing.T) (*Scheduler, *account.Account) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	accounts := account.NewStore(db)
	acc, err := accounts.Create(&account.AccountConfig{Name: "A", DisplayName: "A", Email: "a@example.test", Username: "a",
		IMAPHost: "imap.example.test", IMAPPort: 993, SMTPHost: "smtp.example.test", SMTPPort: 587, AuthType: account.AuthPassword})
	if err != nil {
		t.Fatal(err)
	}
	folders := folder.NewStore(db)
	// A nil IMAP pool: any IMAP use by the routed paths would panic.
	eng := NewEngine(nil, accounts, folders, message.NewStore(db), message.NewAttachmentStore(db))
	s := NewScheduler(eng, accounts, folders)
	s.ctx = context.Background()
	return s, acc
}

func TestAPISyncHookReplacesIMAPForBlockingSync(t *testing.T) {
	s, acc := newAPISyncScheduler(t)
	want := errors.New("provider outage")
	calls := 0
	s.SetAPISync(func(_ context.Context, a *account.Account) (bool, error) {
		calls++
		if a.ID != acc.ID {
			t.Errorf("hook account = %s", a.ID)
		}
		return true, want
	})
	if _, err := s.SyncAccountInboxBlocking(acc.ID); !errors.Is(err, want) {
		t.Fatalf("err = %v; the API error must surface with no IMAP fallback", err)
	}
	if calls != 1 {
		t.Fatalf("hook calls = %d", calls)
	}
	if s.IsSyncing(acc.ID) {
		t.Fatal("account left marked as syncing")
	}
}

func TestAPISyncHookUnroutedAccountIsNotHandled(t *testing.T) {
	s, acc := newAPISyncScheduler(t)
	if handled, _, err := s.syncViaAPI(context.Background(), acc); handled || err != nil {
		t.Fatalf("no hook: handled=%v err=%v", handled, err)
	}
	s.SetAPISync(func(context.Context, *account.Account) (bool, error) { return false, nil })
	if handled, _, err := s.syncViaAPI(context.Background(), acc); handled || err != nil {
		t.Fatalf("unrouted: handled=%v err=%v", handled, err)
	}
}

func TestAPISyncScheduledCompletionReportsError(t *testing.T) {
	s, acc := newAPISyncScheduler(t)
	want := errors.New("throttled")
	s.SetAPISync(func(context.Context, *account.Account) (bool, error) { return true, want })
	var got error
	s.SetSyncCompletedCallback(func(_, _ string, err error) { got = err })
	s.syncAccountInbox(acc)
	if !errors.Is(got, want) {
		t.Fatalf("completion error = %v", got)
	}
}
