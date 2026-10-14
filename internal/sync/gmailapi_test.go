package sync

import (
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/credentials"
	"github.com/caseylabs/mailven/internal/database"
	"github.com/caseylabs/mailven/internal/folder"
	"github.com/caseylabs/mailven/internal/gmailapi"
	"github.com/caseylabs/mailven/internal/message"
	"github.com/caseylabs/mailven/internal/oauth2"
	"github.com/caseylabs/mailven/internal/submission"
)

type fakeGmailMsg struct {
	raw     string
	labels  []string
	thread  string
	history string
}

// fakeGmail serves the read endpoints from memory. Hooks let a test inject
// failures per request.
type fakeGmail struct {
	mu      gosync.Mutex
	msgs    map[string]*fakeGmailMsg
	profile string
	history []map[string]any // history records served for any start ID
	expired bool             // history.list answers 404
	hook    func(r *http.Request) (status int, body string, header map[string]string)
	label1  string // display name of Label_1; "Receipts" when empty
	rawGets []string
	events  []string // profile/list/history ordering
}

// setHook swaps the hook under the lock: a cancelled fan-out request can still
// be in ServeHTTP when Run returns.
func (f *fakeGmail) setHook(h func(r *http.Request) (status int, body string, header map[string]string)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hook = h
}

func (f *fakeGmail) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.hook != nil {
		if status, body, hdr := f.hook(r); status != 0 {
			for k, v := range hdr {
				w.Header().Set(k, v)
			}
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
			return
		}
	}
	out := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	switch p := r.URL.Path; {
	case p == "/profile":
		f.events = append(f.events, "profile")
		out(map[string]string{"historyId": f.profile})
	case p == "/labels":
		out(map[string]any{"labels": []map[string]string{
			{"id": "INBOX", "name": "INBOX", "type": "system"},
			{"id": "Label_1", "name": cmp.Or(f.label1, "Receipts"), "type": "user"}}})
	case p == "/messages":
		f.events = append(f.events, "list")
		var ids []map[string]string
		keys := make([]string, 0, len(f.msgs))
		for id := range f.msgs {
			keys = append(keys, id)
		}
		sort.Strings(keys)
		for _, id := range keys {
			ids = append(ids, map[string]string{"id": id})
		}
		out(map[string]any{"messages": ids})
	case p == "/history":
		f.events = append(f.events, "history")
		if f.expired {
			w.WriteHeader(404)
			return
		}
		out(map[string]any{"history": f.history, "historyId": f.profile})
	default:
		id := p[len("/messages/"):]
		m, ok := f.msgs[id]
		if !ok {
			w.WriteHeader(404)
			return
		}
		resp := map[string]any{"id": id, "threadId": m.thread, "labelIds": m.labels, "historyId": m.history}
		if r.URL.Query().Get("format") == "raw" {
			f.rawGets = append(f.rawGets, id)
			resp["raw"] = base64.RawURLEncoding.EncodeToString([]byte(m.raw))
		}
		out(resp)
	}
}

type gmailRig struct {
	t      *testing.T
	fake   *fakeGmail
	srv    *httptest.Server
	g      *GmailSync
	store  *message.Store
	db     *database.DB
	sleeps []time.Duration
	fails  []oauth2.MailAPIFailure
}

func newGmailRig(t *testing.T) *gmailRig {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO accounts (id, name, email, imap_host, smtp_host, username)
		VALUES ('acct-1', 'Test', 'me@example.com', 'imap.example.com', 'smtp.example.com', 'me')`); err != nil {
		t.Fatal(err)
	}
	r := &gmailRig{t: t, fake: &fakeGmail{msgs: map[string]*fakeGmailMsg{}, profile: "100"}, db: db}
	r.srv = httptest.NewServer(r.fake)
	t.Cleanup(r.srv.Close)
	r.store = message.NewStore(db)
	eng := NewEngine(nil, account.NewStore(db), folder.NewStore(db), r.store, message.NewAttachmentStore(db))
	r.g = &GmailSync{
		Engine: eng, AccountID: "acct-1", Concurrency: 3,
		Reader: &gmailapi.Reader{Tokens: staticTokens{}, Base: r.srv.URL},
		Tokens: func() (*credentials.OAuthTokens, error) {
			return &credentials.OAuthTokens{Provider: "google", ScopeResource: "https://www.googleapis.com", ScopesKnown: true,
				Scopes: []string{"https://www.googleapis.com/auth/gmail.readonly"}}, nil
		},
		Sleep:     func(_ context.Context, d time.Duration) error { r.sleeps = append(r.sleeps, d); return nil },
		OnFailure: func(f oauth2.MailAPIFailure, _ error) { r.fails = append(r.fails, f) },
	}
	return r
}

type staticTokens struct{}

func (staticTokens) Token(context.Context) (string, error)   { return "tok", nil }
func (staticTokens) Refresh(context.Context) (string, error) { return "tok2", nil }

var _ submission.TokenSource = staticTokens{}

func plainMail(id, subject, body string) string {
	return fmt.Sprintf("From: Sender <s@example.com>\r\nTo: me@example.com\r\nSubject: %s\r\nMessage-ID: <%s@example.com>\r\nDate: Mon, 02 Jan 2026 10:00:00 +0000\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n", subject, id, body)
}

func (r *gmailRig) add(id string, labels []string, raw, history string) {
	r.fake.msgs[id] = &fakeGmailMsg{raw: raw, labels: labels, thread: "T" + id, history: history}
}

func (r *gmailRig) folderRows(path string) []*message.MessageHeader {
	r.t.Helper()
	fs, _ := folder.NewStore(r.db).List("acct-1")
	for _, f := range fs {
		if f.Path == path {
			hs, err := r.store.ListByFolder(f.ID, 0, 1000)
			if err != nil {
				r.t.Fatal(err)
			}
			return hs
		}
	}
	return nil
}

func (r *gmailRig) run() (GmailResult, error) { return r.g.Run(context.Background()) }

func (r *gmailRig) cursor() message.APICursor {
	c, err := r.store.LoadAPICursor("acct-1", gmailScope)
	if err != nil {
		r.t.Fatal(err)
	}
	return c
}

func TestGmailAPIInitialSync(t *testing.T) {
	r := newGmailRig(t)
	r.add("m1", []string{"INBOX", "UNREAD"}, plainMail("m1", "Hello", "body one"), "50")
	r.add("m2", []string{"INBOX", "Label_1", "STARRED"}, plainMail("m2", "Two folders", "body two"), "51")
	r.add("m3", []string{"CATEGORY_UPDATES"}, plainMail("m3", "Archived", "body three"), "52")
	res, err := r.run()
	if err != nil || !res.Reconciled || res.Added != 3 {
		t.Fatalf("Run = %+v, %v", res, err)
	}
	if c := r.cursor(); c.Cursor != "100" || c.Kind != gmailCursorKind || c.Invalid {
		t.Fatalf("cursor = %+v", c)
	}
	if got := r.fake.events[:2]; got[0] != "profile" || got[1] != "list" {
		t.Fatalf("profile must be captured before listing: %v", r.fake.events)
	}
	if n := len(r.folderRows("INBOX")); n != 2 {
		t.Fatalf("INBOX rows = %d", n)
	}
	if n := len(r.folderRows("Receipts")); n != 1 {
		t.Fatalf("Receipts rows = %d", n)
	}
	if n := len(r.folderRows("[Gmail]/All Mail")); n != 1 {
		t.Fatalf("All Mail rows = %d", n)
	}
	ref, rows, err := r.store.GetAPIRef("acct-1", "gmail", "m2")
	if err != nil || ref.RemoteThreadID != "Tm2" || ref.Version != "51" || len(rows) != 2 {
		t.Fatalf("ref = %+v rows=%v err=%v", ref, rows, err)
	}
	// The local thread ID derives from headers, not from Gmail's threadId.
	for _, h := range r.folderRows("Receipts") {
		m, err := r.store.Get(h.ID)
		if err != nil || m.ThreadID == "Tm2" || m.ThreadID == "" {
			t.Fatalf("thread ID = %q, %v", m.ThreadID, err)
		}
	}
	// A second run with no changes is incremental and fetches no raw messages.
	r.fake.rawGets = nil
	res, err = r.run()
	if err != nil || res.Reconciled || res.Added != 0 || len(r.fake.rawGets) != 0 {
		t.Fatalf("second Run = %+v, %v, raw=%v", res, err, r.fake.rawGets)
	}
}

func TestGmailAPIFixturesParseLikeIMAP(t *testing.T) {
	r := newGmailRig(t)
	latin := "From: =?ISO-8859-1?Q?Andr=E9?= <andre@example.com>\r\nTo: me@example.com\r\nSubject: =?ISO-8859-1?Q?Caf=E9?=\r\nMessage-ID: <latin@example.com>\r\nDate: Mon, 02 Jan 2026 10:00:00 +0000\r\nContent-Type: text/plain; charset=iso-8859-1\r\nContent-Transfer-Encoding: 8bit\r\n\r\ncaf\xe9 cr\xe8me\r\n"
	idn := "From: =?UTF-8?B?w5xtbGF1dA==?= <user@münchen.example>\r\nTo: me@example.com\r\nSubject: IDN\r\nMessage-ID: <idn@example.com>\r\nDate: Mon, 02 Jan 2026 10:00:00 +0000\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nhallo\r\n"
	signed := "From: s@example.com\r\nTo: me@example.com\r\nSubject: signed\r\nMessage-ID: <signed@example.com>\r\nDate: Mon, 02 Jan 2026 10:00:00 +0000\r\nMIME-Version: 1.0\r\nContent-Type: multipart/signed; protocol=\"application/pkcs7-signature\"; micalg=sha-256; boundary=\"B\"\r\n\r\n--B\r\nContent-Type: text/plain\r\n\r\nsigned text\r\n--B\r\nContent-Type: application/pkcs7-signature; name=smime.p7s\r\nContent-Transfer-Encoding: base64\r\n\r\nAAAA\r\n--B--\r\n"
	enc := "From: s@example.com\r\nTo: me@example.com\r\nSubject: secret\r\nMessage-ID: <enc@example.com>\r\nDate: Mon, 02 Jan 2026 10:00:00 +0000\r\nMIME-Version: 1.0\r\nContent-Type: application/pkcs7-mime; smime-type=enveloped-data; name=smime.p7m\r\nContent-Transfer-Encoding: base64\r\n\r\nAAAAAAAA\r\n"
	att := "From: s@example.com\r\nTo: me@example.com\r\nSubject: attached\r\nMessage-ID: <att@example.com>\r\nDate: Mon, 02 Jan 2026 10:00:00 +0000\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=\"X\"\r\n\r\n--X\r\nContent-Type: text/plain\r\n\r\nsee file\r\n--X\r\nContent-Type: application/pdf; name=\"r.pdf\"\r\nContent-Disposition: attachment; filename=\"r.pdf\"\r\nContent-Transfer-Encoding: base64\r\n\r\nJVBERg==\r\n--X--\r\n"
	for i, raw := range map[string]string{"latin": latin, "idn": idn, "signed": signed, "enc": enc, "att": att} {
		r.add(i, []string{"INBOX"}, raw, "7")
	}
	if _, err := r.run(); err != nil {
		t.Fatal(err)
	}
	byID := map[string]*message.Message{}
	for _, h := range r.folderRows("INBOX") {
		m, err := r.store.Get(h.ID)
		if err != nil {
			t.Fatal(err)
		}
		byID[m.MessageID] = m
	}
	if m := byID["latin@example.com"]; m == nil || m.Subject != "Café" || m.FromName != "André" || !strings.Contains(m.BodyText, "café crème") {
		t.Fatalf("latin-1 message = %+v", m)
	}
	if m := byID["idn@example.com"]; m == nil || m.FromEmail != "user@münchen.example" || m.FromName != "Ümlaut" {
		t.Fatalf("IDN message = %+v", m)
	}
	if m := byID["signed@example.com"]; m == nil || !m.HasSMIME {
		t.Fatalf("signed message lost its raw S/MIME body: %+v", m)
	}
	if m := byID["enc@example.com"]; m == nil || !m.SMIMEEncrypted || !m.HasSMIME {
		t.Fatalf("encrypted message = %+v", m)
	}
	m := byID["att@example.com"]
	if m == nil || !m.HasAttachments {
		t.Fatalf("attachment message = %+v", m)
	}
	atts, err := message.NewAttachmentStore(r.db).GetByMessage(m.ID)
	if err != nil || len(atts) != 1 || atts[0].Filename != "r.pdf" {
		t.Fatalf("attachments = %+v, %v", atts, err)
	}
}

func TestGmailAPIIncrementalHistory(t *testing.T) {
	r := newGmailRig(t)
	r.add("m1", []string{"INBOX", "UNREAD"}, plainMail("m1", "One", "b1"), "10")
	r.add("m2", []string{"INBOX"}, plainMail("m2", "Two", "b2"), "11")
	r.add("m3", []string{"INBOX"}, plainMail("m3", "Three", "b3"), "12")
	if _, err := r.run(); err != nil {
		t.Fatal(err)
	}
	// m1 is read (flag change only), m2 gains a user label (new folder row),
	// m3 is deleted, m4 arrives.
	r.fake.msgs["m1"].labels, r.fake.msgs["m1"].history = []string{"INBOX"}, "101"
	r.fake.msgs["m2"].labels, r.fake.msgs["m2"].history = []string{"INBOX", "Label_1"}, "102"
	delete(r.fake.msgs, "m3")
	r.add("m4", []string{"INBOX"}, plainMail("m4", "Four", "b4"), "103")
	r.fake.profile = "104"
	r.fake.history = []map[string]any{
		{"id": "101", "labelsRemoved": []map[string]any{{"message": map[string]string{"id": "m1"}}}},
		{"id": "102", "labelsAdded": []map[string]any{{"message": map[string]string{"id": "m2"}}}},
		{"id": "103", "messagesDeleted": []map[string]any{{"message": map[string]string{"id": "m3"}}}},
		{"id": "104", "messagesAdded": []map[string]any{{"message": map[string]string{"id": "m4"}}}},
	}
	r.fake.rawGets = nil
	res, err := r.run()
	if err != nil || res.Reconciled || res.Added != 1 || res.Updated != 2 || res.Removed != 1 {
		t.Fatalf("Run = %+v, %v", res, err)
	}
	sort.Strings(r.fake.rawGets)
	if fmt.Sprint(r.fake.rawGets) != "[m2 m4]" { // m1 needs no body, only a flag change
		t.Fatalf("raw fetches = %v", r.fake.rawGets)
	}
	var unread int
	for _, h := range r.folderRows("INBOX") {
		if !h.IsRead {
			unread++
		}
	}
	if n := len(r.folderRows("INBOX")); n != 3 || unread != 0 {
		t.Fatalf("INBOX rows=%d unread=%d", n, unread)
	}
	if n := len(r.folderRows("Receipts")); n != 1 {
		t.Fatalf("Receipts rows = %d", n)
	}
	if ref, _, _ := r.store.GetAPIRef("acct-1", "gmail", "m3"); ref != nil {
		t.Fatal("deleted message ref remains")
	}
	if c := r.cursor(); c.Cursor != "104" {
		t.Fatalf("cursor = %+v", c)
	}
}

// A renamed label keeps the message's history ID, so the unchanged-version
// shortcut must not hide the move to the new folder path.
func TestGmailAPILabelRenameMovesRows(t *testing.T) {
	r := newGmailRig(t)
	r.add("m1", []string{"INBOX", "Label_1"}, plainMail("m1", "One", "b1"), "10")
	if _, err := r.run(); err != nil {
		t.Fatal(err)
	}
	r.fake.label1, r.fake.profile = "Bills", "20"
	r.fake.history = []map[string]any{
		{"id": "20", "labelsAdded": []map[string]any{{"message": map[string]string{"id": "m1"}}}},
	}
	if _, err := r.run(); err != nil {
		t.Fatal(err)
	}
	if old, renamed := len(r.folderRows("Receipts")), len(r.folderRows("Bills")); old != 0 || renamed != 1 {
		t.Fatalf("Receipts rows = %d, Bills rows = %d", old, renamed)
	}
}

func TestGmailAPIHistoryResetReconciles(t *testing.T) {
	r := newGmailRig(t)
	r.add("keep", []string{"INBOX"}, plainMail("keep", "Keep", "b"), "10")
	r.add("gone", []string{"INBOX"}, plainMail("gone", "Gone", "b"), "11")
	if _, err := r.run(); err != nil {
		t.Fatal(err)
	}
	// A local-only message (no API ref) must survive the reconcile.
	fs, _ := folder.NewStore(r.db).List("acct-1")
	local := &message.Message{ID: "local-1", AccountID: "acct-1", FolderID: fs[0].ID, UID: 9000, Date: time.Now(), Subject: "local draft"}
	if err := r.store.Create(local); err != nil {
		t.Fatal(err)
	}
	delete(r.fake.msgs, "gone")
	r.add("new", []string{"INBOX"}, plainMail("new", "New", "b"), "20")
	r.fake.expired, r.fake.profile = true, "200"
	res, err := r.run()
	if err != nil || !res.Reconciled || res.Added != 1 || res.Removed != 1 {
		t.Fatalf("Run = %+v, %v", res, err)
	}
	subjects := map[string]bool{}
	for _, h := range r.folderRows("INBOX") {
		subjects[h.Subject] = true
	}
	if !subjects["Keep"] || !subjects["New"] || subjects["Gone"] || !subjects["local draft"] {
		t.Fatalf("subjects = %v", subjects)
	}
	c := r.cursor()
	if c.Cursor != "200" || c.Invalid || c.Generation < 1 {
		t.Fatalf("cursor = %+v (generation must be bumped)", c)
	}
}

func TestGmailAPIMidReconcileFailureKeepsOldData(t *testing.T) {
	r := newGmailRig(t)
	r.add("a", []string{"INBOX"}, plainMail("a", "Old A", "b"), "10")
	if _, err := r.run(); err != nil {
		t.Fatal(err)
	}
	r.add("b", []string{"INBOX"}, plainMail("b", "New B", "b"), "20")
	delete(r.fake.msgs, "a")
	r.fake.expired = true
	r.fake.setHook(func(req *http.Request) (int, string, map[string]string) {
		if req.URL.Path == "/messages/b" {
			return 403, `{"error":{"errors":[{"reason":"forbidden"}]}}`, nil
		}
		return 0, "", nil
	})
	if _, err := r.run(); err == nil {
		t.Fatal("expected failure")
	}
	if c := r.cursor(); !c.Invalid {
		t.Fatalf("cursor must stay unreconciled: %+v", c)
	}
	rows := r.folderRows("INBOX")
	if len(rows) != 1 || rows[0].Subject != "Old A" {
		t.Fatalf("old data was touched: %+v", rows)
	}
	if len(r.fails) != 1 || r.fails[0] != oauth2.MailAPIFailurePolicy {
		t.Fatalf("failures = %v", r.fails)
	}
	// Once Gmail answers again the reconcile completes.
	r.fake.hook, r.fake.expired = nil, false
	if res, err := r.run(); err != nil || !res.Reconciled {
		t.Fatalf("recovery Run = %+v, %v", res, err)
	}
	rows = r.folderRows("INBOX")
	if len(rows) != 1 || rows[0].Subject != "New B" || r.cursor().Invalid {
		t.Fatalf("after recovery: %+v %+v", rows, r.cursor())
	}
}

func TestGmailAPICrashBetweenPageAndCursor(t *testing.T) {
	r := newGmailRig(t)
	r.add("seed", []string{"INBOX"}, plainMail("seed", "Seed", "b"), "1")
	if _, err := r.run(); err != nil {
		t.Fatal(err)
	}
	var added []map[string]any
	for i := 0; i < gmailBatchSize+10; i++ {
		id := fmt.Sprintf("n%03d", i)
		r.add(id, []string{"INBOX"}, plainMail(id, "Msg "+id, "b"), "5")
		added = append(added, map[string]any{"id": "5", "messagesAdded": []map[string]any{{"message": map[string]string{"id": id}}}})
	}
	r.fake.profile, r.fake.history = "300", added
	// The second batch fails after the first has been committed.
	r.fake.setHook(func(req *http.Request) (int, string, map[string]string) {
		if req.URL.Path == "/messages/n055" {
			return 500, "{}", nil
		}
		return 0, "", nil
	})
	if _, err := r.run(); err == nil {
		t.Fatal("expected failure")
	}
	if c := r.cursor(); c.Cursor != "100" {
		t.Fatalf("cursor advanced past an unfinished run: %+v", c)
	}
	if n := len(r.folderRows("INBOX")); n != 1+gmailBatchSize {
		t.Fatalf("rows after crash = %d, want the committed first batch", n)
	}
	r.fake.setHook(nil)
	res, err := r.run()
	if err != nil || res.Reconciled {
		t.Fatalf("resume = %+v, %v", res, err)
	}
	if n := len(r.folderRows("INBOX")); n != 1+gmailBatchSize+10 {
		t.Fatalf("rows after resume = %d (duplicates or losses)", n)
	}
	if c := r.cursor(); c.Cursor != "300" {
		t.Fatalf("cursor = %+v", c)
	}
}

func TestGmailAPICancellation(t *testing.T) {
	r := newGmailRig(t)
	r.add("a", []string{"INBOX"}, plainMail("a", "A", "b"), "1")
	ctx, cancel := context.WithCancel(context.Background())
	r.fake.setHook(func(req *http.Request) (int, string, map[string]string) {
		if req.URL.Path == "/messages/a" {
			cancel()
		}
		return 0, "", nil
	})
	if _, err := r.g.Run(ctx); err == nil {
		t.Fatal("expected cancellation error")
	}
	if c := r.cursor(); c.Cursor != "" || c.Kind != "" {
		t.Fatalf("cursor advanced after cancellation: %+v", c)
	}
	if len(r.fails) != 0 {
		t.Fatalf("cancellation reported as an authorization failure: %v", r.fails)
	}
}

func TestGmailAPIRateLimitBackoff(t *testing.T) {
	t.Run("retries honour Retry-After then succeed", func(t *testing.T) {
		r := newGmailRig(t)
		r.add("a", []string{"INBOX"}, plainMail("a", "A", "b"), "1")
		hits := 0
		r.fake.setHook(func(req *http.Request) (int, string, map[string]string) {
			if req.URL.Path == "/messages/a" && req.URL.Query().Get("format") == "raw" {
				if hits++; hits <= 2 {
					return 429, "{}", map[string]string{"Retry-After": "2"}
				}
			}
			return 0, "", nil
		})
		if _, err := r.run(); err != nil {
			t.Fatal(err)
		}
		if fmt.Sprint(r.sleeps) != "[2s 2s]" {
			t.Fatalf("sleeps = %v", r.sleeps)
		}
	})
	t.Run("bounded: gives up with a hint and keeps credentials", func(t *testing.T) {
		r := newGmailRig(t)
		r.add("a", []string{"INBOX"}, plainMail("a", "A", "b"), "1")
		r.fake.setHook(func(req *http.Request) (int, string, map[string]string) {
			if req.URL.Path == "/messages/a" {
				return 403, `{"error":{"errors":[{"reason":"rateLimitExceeded"}]}}`, map[string]string{"Retry-After": "3600"}
			}
			return 0, "", nil
		})
		_, err := r.run()
		var re *gmailapi.ReadError
		if !asReadError(err, &re) || !re.RateLimited || re.RetryAfter != time.Hour {
			t.Fatalf("err = %#v", err)
		}
		if len(r.sleeps) != apiRetryLimit || r.sleeps[0] != apiMaxBackoff {
			t.Fatalf("sleeps = %v (waits are capped)", r.sleeps)
		}
		if len(r.fails) != 0 {
			t.Fatalf("a rate limit must not raise an authorization failure: %v", r.fails)
		}
	})
}

func asReadError(err error, target **gmailapi.ReadError) bool {
	re, ok := err.(*gmailapi.ReadError)
	*target = re
	return ok
}

func TestGmailAPIFailurePolicy(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   oauth2.MailAPIFailure
	}{
		{"403 policy", 403, `{"error":{"errors":[{"reason":"forbidden"}]}}`, oauth2.MailAPIFailurePolicy},
		{"401 after refresh", 401, `{}`, oauth2.MailAPIFailureReauthorize},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newGmailRig(t)
			r.fake.setHook(func(req *http.Request) (int, string, map[string]string) { return tc.status, tc.body, nil })
			if _, err := r.run(); err == nil {
				t.Fatal("expected failure")
			}
			if len(r.fails) != 1 || r.fails[0] != tc.want {
				t.Fatalf("failures = %v, want %v", r.fails, tc.want)
			}
		})
	}
}

func TestGmailAPIRequiresReadCapability(t *testing.T) {
	r := newGmailRig(t)
	requests := 0
	r.fake.setHook(func(*http.Request) (int, string, map[string]string) { requests++; return 0, "", nil })
	r.g.Tokens = func() (*credentials.OAuthTokens, error) {
		return &credentials.OAuthTokens{Provider: "google", ScopeResource: "https://www.googleapis.com", ScopesKnown: true,
			Scopes: []string{"https://www.googleapis.com/auth/gmail.send"}}, nil
	}
	if _, err := r.run(); err == nil || requests != 0 {
		t.Fatalf("err=%v requests=%d", err, requests)
	}
	if len(r.fails) != 1 || r.fails[0] != oauth2.MailAPIFailurePartialConsent {
		t.Fatalf("failures = %v", r.fails)
	}
}
