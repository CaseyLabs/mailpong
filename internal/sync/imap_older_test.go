package sync

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/database"
	"github.com/caseylabs/mailven/internal/folder"
	imapPkg "github.com/caseylabs/mailven/internal/imap"
	"github.com/caseylabs/mailven/internal/message"
)

// imapRig is an Engine on a real database against an in-memory IMAP server.
type imapRig struct {
	t        *testing.T
	eng      *Engine
	store    *message.Store
	acc      *account.Account
	folder   *folder.Folder
	addr     string
	nextTime time.Time
}

const rigWindowDays = 30

func newIMAPRig(t *testing.T) *imapRig {
	t.Helper()
	mem := imapmemserver.New()
	user := imapmemserver.NewUser("u", "p")
	if err := user.Create("INBOX", nil); err != nil {
		t.Fatal(err)
	}
	mem.AddUser(user)
	srv := imapserver.New(&imapserver.Options{
		NewSession: func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			return mem.NewSession(), nil, nil
		},
		InsecureAuth: true,
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	var port int
	_, _ = fmt.Sscan(portStr, &port)

	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	accounts := account.NewStore(db)
	acc, err := accounts.Create(&account.AccountConfig{Name: "A", DisplayName: "A", Email: "u@example.test", Username: "u",
		IMAPHost: host, IMAPPort: port, SMTPHost: "smtp.example.test", SMTPPort: 587, AuthType: account.AuthPassword,
		SyncPeriodDays: rigWindowDays})
	if err != nil {
		t.Fatal(err)
	}
	folders := folder.NewStore(db)
	inbox := &folder.Folder{AccountID: acc.ID, Name: "INBOX", Path: "INBOX", Type: folder.TypeInbox}
	if err := folders.Create(inbox); err != nil {
		t.Fatal(err)
	}
	pool := imapPkg.NewPool(imapPkg.DefaultPoolConfig(), func(string) (*imapPkg.ClientConfig, error) {
		return &imapPkg.ClientConfig{Host: host, Port: port, Security: imapPkg.SecurityNone, Username: "u", Password: "p",
			ConnectTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second}, nil
	})
	t.Cleanup(pool.CloseAll)
	store := message.NewStore(db)
	return &imapRig{t: t, eng: NewEngine(pool, accounts, folders, store, message.NewAttachmentStore(db)), store: store,
		acc: acc, folder: inbox, addr: ln.Addr().String(), nextTime: time.Now()}
}

// client dials the server directly, as the "other client" that changes mail.
func (r *imapRig) client() *imapclient.Client {
	c, err := imapclient.DialInsecure(r.addr, nil)
	if err != nil {
		r.t.Fatal(err)
	}
	r.t.Cleanup(func() { _ = c.Close() })
	if err := c.Login("u", "p").Wait(); err != nil {
		r.t.Fatal(err)
	}
	if _, err := c.Select("INBOX", nil).Wait(); err != nil {
		r.t.Fatal(err)
	}
	return c
}

// add appends a message received daysAgo and returns its UID.
func (r *imapRig) add(subject string, daysAgo int) uint32 {
	r.t.Helper()
	when := time.Now().AddDate(0, 0, -daysAgo).UTC().Truncate(time.Second)
	body := fmt.Sprintf("From: Ann <ann@example.test>\r\nTo: u@example.test\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: <%s@example.test>\r\n\r\nbody of %s\r\n",
		subject, when.Format(time.RFC1123Z), strings.ReplaceAll(subject, " ", "-"), subject)
	c := r.client()
	cmd := c.Append("INBOX", int64(len(body)), &imap.AppendOptions{Time: when})
	if _, err := cmd.Write([]byte(body)); err != nil {
		r.t.Fatal(err)
	}
	if err := cmd.Close(); err != nil {
		r.t.Fatal(err)
	}
	data, err := cmd.Wait()
	if err != nil {
		r.t.Fatal(err)
	}
	return uint32(data.UID)
}

// expunge deletes a message on the server.
func (r *imapRig) expunge(uid uint32) {
	r.t.Helper()
	c := r.client()
	set := imap.UIDSetNum(imap.UID(uid))
	if err := c.Store(set, &imap.StoreFlags{Op: imap.StoreFlagsAdd, Silent: true, Flags: []imap.Flag{imap.FlagDeleted}}, nil).Close(); err != nil {
		r.t.Fatal(err)
	}
	if err := c.Expunge().Close(); err != nil {
		r.t.Fatal(err)
	}
}

func (r *imapRig) sync() {
	r.t.Helper()
	if err := r.eng.SyncMessages(context.Background(), r.acc.ID, r.folder.ID, rigWindowDays, false); err != nil {
		r.t.Fatalf("SyncMessages: %v", err)
	}
}

func (r *imapRig) uids() map[uint32]bool {
	r.t.Helper()
	uids, err := r.store.GetAllUIDs(r.folder.ID)
	if err != nil {
		r.t.Fatal(err)
	}
	out := map[uint32]bool{}
	for _, u := range uids {
		out[u] = true
	}
	return out
}

func TestIMAPSearchServerPinsOlderMail(t *testing.T) {
	r := newIMAPRig(t)
	oldMatch := r.add("quarterly invoice", 200)
	oldOther := r.add("holiday photos", 210)
	recent := r.add("invoice reminder", 3)
	r.sync()
	if got := r.uids(); !got[recent] || got[oldMatch] || got[oldOther] {
		t.Fatalf("after the first sync only the recent message is local, got %v", got)
	}

	found, added, err := r.eng.SearchServer(context.Background(), r.acc.ID, "invoice", 50)
	if err != nil {
		t.Fatal(err)
	}
	if found != 2 || added != 1 {
		t.Fatalf("found/added = %d/%d, want 2/1 (the recent match is already local)", found, added)
	}
	if got := r.uids(); !got[oldMatch] || got[oldOther] {
		t.Fatalf("only the matching old message is stored, got %v", got)
	}

	// The window's deletions leave the pinned message alone, across syncs.
	r.sync()
	r.sync()
	if !r.uids()[oldMatch] {
		t.Fatal("a sync removed the pinned message")
	}

	// A search that finds it already local stores nothing more.
	if _, added, err = r.eng.SearchServer(context.Background(), r.acc.ID, "invoice", 50); err != nil || added != 0 {
		t.Fatalf("repeat search added %d (err %v), want 0", added, err)
	}

	// Deleted on the server: the next sync notices.
	r.expunge(oldMatch)
	r.sync()
	if got := r.uids(); got[oldMatch] || !got[recent] {
		t.Fatalf("pinned message deleted on the server should go, recent stays; got %v", got)
	}
}

func TestIMAPSearchServerLimit(t *testing.T) {
	r := newIMAPRig(t)
	for i := range 4 {
		r.add(fmt.Sprintf("receipt %d", i), 100+i)
	}
	r.sync()
	found, added, err := r.eng.SearchServer(context.Background(), r.acc.ID, "receipt", 3)
	if err != nil || found != 4 || added != 3 {
		t.Fatalf("found/added = %d/%d (err %v), want 4/3", found, added, err)
	}
}

func TestIMAPLoadOlderPagesBackFromTheWindow(t *testing.T) {
	r := newIMAPRig(t)
	var old []uint32
	for i := range 5 {
		old = append(old, r.add(fmt.Sprintf("old %d", i), 100+i))
	}
	recent := r.add("recent", 2)
	r.sync()

	var addedPerStep []int
	for step := 0; step < 6; step++ {
		_, added, done, err := r.eng.LoadOlder(context.Background(), r.acc.ID, rigWindowDays, 2)
		if err != nil {
			t.Fatal(err)
		}
		addedPerStep = append(addedPerStep, added)
		r.sync() // a reconcile between steps keeps what was loaded
		if done {
			break
		}
	}
	if fmt.Sprint(addedPerStep) != "[2 2 1]" {
		t.Fatalf("pages = %v, want [2 2 1]", addedPerStep)
	}
	got := r.uids()
	for _, uid := range append(old, recent) {
		if !got[uid] {
			t.Fatalf("uid %d missing after loading everything: %v", uid, got)
		}
	}

	r.expunge(old[0])
	r.sync()
	if r.uids()[old[0]] {
		t.Fatal("a loaded message deleted on the server survived the sync")
	}

	// No sync period: everything is already local.
	if _, _, done, err := r.eng.LoadOlder(context.Background(), r.acc.ID, 0, 2); err != nil || !done {
		t.Fatalf("no sync period: done=%v err=%v, want done", done, err)
	}
}
