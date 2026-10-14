package sync

import (
	"context"
	"encoding/json"
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
	"github.com/caseylabs/mailven/internal/graphapi"
	"github.com/caseylabs/mailven/internal/message"
	"github.com/caseylabs/mailven/internal/oauth2"
)

type fakeGraphMsg struct {
	parent, change, conv, raw string
	read, flagged, draft      bool
}

// fakeGraph serves the Graph read endpoints from memory. Every delta lists the
// folder's current messages plus the IDs queued in removed.
type fakeGraph struct {
	mu       gosync.Mutex
	base     string
	msgs     map[string]*fakeGraphMsg
	folders  map[string]string   // folder ID -> display name (all children of the root)
	removed  map[string][]string // folder ID -> IDs reported as @removed
	expired  map[string]bool     // folder ID -> delta answers 410
	hook     func(r *http.Request) (int, string, map[string]string)
	rawGets  []string
	deltaHit []string
}

// setHook swaps the hook under the lock: a cancelled fan-out request can still
// be in ServeHTTP when Run returns.
func (f *fakeGraph) setHook(h func(r *http.Request) (int, string, map[string]string)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hook = h
}

func (f *fakeGraph) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	wire := func(id string, m *fakeGraphMsg) map[string]any {
		flag := "notFlagged"
		if m.flagged {
			flag = "flagged"
		}
		return map[string]any{"id": id, "parentFolderId": m.parent, "changeKey": m.change, "conversationId": m.conv,
			"isRead": m.read, "isDraft": m.draft, "flag": map[string]string{"flagStatus": flag}}
	}
	p := strings.TrimPrefix(r.URL.Path, "/me/")
	switch parts := strings.Split(p, "/"); {
	case p == "mailFolders/msgfolderroot":
		out(map[string]string{"id": "root-id"})
	case p == "mailFolders/inbox":
		out(map[string]string{"id": "inbox-id"})
	case len(parts) == 2 && parts[0] == "mailFolders":
		w.WriteHeader(404)
	case p == "mailFolders":
		var vals []map[string]any
		for id, name := range f.folders {
			vals = append(vals, map[string]any{"id": id, "displayName": name, "parentFolderId": "root-id"})
		}
		sort.Slice(vals, func(i, j int) bool { return vals[i]["id"].(string) < vals[j]["id"].(string) })
		out(map[string]any{"value": vals})
	case len(parts) == 4 && parts[0] == "mailFolders" && parts[3] == "delta":
		fid := parts[1]
		f.deltaHit = append(f.deltaHit, fid)
		if f.expired[fid] {
			w.WriteHeader(410)
			return
		}
		var vals []map[string]any
		ids := make([]string, 0)
		for id, m := range f.msgs {
			if m.parent == fid {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		for _, id := range ids {
			vals = append(vals, wire(id, f.msgs[id]))
		}
		for _, id := range f.removed[fid] {
			vals = append(vals, map[string]any{"id": id, "@removed": map[string]string{"reason": "deleted"}})
		}
		out(map[string]any{"value": vals, "@odata.deltaLink": f.base + r.URL.Path + "?$deltatoken=t"})
	case len(parts) == 3 && parts[0] == "messages" && parts[2] == "$value":
		m, ok := f.msgs[parts[1]]
		if !ok {
			w.WriteHeader(404)
			return
		}
		f.rawGets = append(f.rawGets, parts[1])
		_, _ = w.Write([]byte(m.raw))
	case len(parts) == 2 && parts[0] == "messages":
		m, ok := f.msgs[parts[1]]
		if !ok {
			w.WriteHeader(404)
			return
		}
		out(wire(parts[1], m))
	default:
		w.WriteHeader(500)
	}
}

type graphRig struct {
	t     *testing.T
	fake  *fakeGraph
	g     *GraphSync
	store *message.Store
	db    *database.DB
	fails []oauth2.MailAPIFailure
	slept []time.Duration
}

func newGraphRig(t *testing.T) *graphRig {
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
	r := &graphRig{t: t, db: db, fake: &fakeGraph{msgs: map[string]*fakeGraphMsg{}, folders: map[string]string{"inbox-id": "Inbox", "recv-id": "Receipts"},
		removed: map[string][]string{}, expired: map[string]bool{}}}
	srv := httptest.NewServer(r.fake)
	t.Cleanup(srv.Close)
	r.fake.base = srv.URL
	r.store = message.NewStore(db)
	eng := NewEngine(nil, account.NewStore(db), folder.NewStore(db), r.store, message.NewAttachmentStore(db))
	r.g = &GraphSync{
		Engine: eng, AccountID: "acct-1", Concurrency: 3,
		Reader: &graphapi.Reader{Tokens: staticTokens{}, Base: srv.URL + "/v1.0-unused"},
		Tokens: func() (*credentials.OAuthTokens, error) {
			return &credentials.OAuthTokens{Provider: "microsoft", ScopeResource: "https://graph.microsoft.com", ScopesKnown: true,
				Scopes: []string{"https://graph.microsoft.com/Mail.Read"}}, nil
		},
		Sleep:     func(_ context.Context, d time.Duration) error { r.slept = append(r.slept, d); return nil },
		OnFailure: func(f oauth2.MailAPIFailure, _ error) { r.fails = append(r.fails, f) },
	}
	r.g.Reader.Base = srv.URL
	return r
}

func (r *graphRig) add(id, parent, change string, raw string) {
	r.fake.msgs[id] = &fakeGraphMsg{parent: parent, change: change, conv: "C" + id, raw: raw}
}

func (r *graphRig) run() (GraphResult, error) { return r.g.Run(context.Background()) }

func (r *graphRig) rows(path string) []*message.MessageHeader {
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

func (r *graphRig) paths() (out []string) {
	fs, _ := folder.NewStore(r.db).List("acct-1")
	for _, f := range fs {
		out = append(out, f.Path)
	}
	return
}

func (r *graphRig) hasFolder(path string) bool {
	fs, _ := folder.NewStore(r.db).List("acct-1")
	for _, f := range fs {
		if f.Path == path {
			return true
		}
	}
	return false
}

func TestGraphSyncInitialAndIncremental(t *testing.T) {
	r := newGraphRig(t)
	r.add("m1", "inbox-id", "k1", plainMail("m1", "Hello", "body one"))
	r.add("m2", "recv-id", "k1", plainMail("m2", "Receipt", "body two"))
	r.add("m3", "other-id", "k1", plainMail("m3", "Unmapped", "body three"))
	res, err := r.run()
	if err != nil || res.Added != 2 || res.Reconciled != 2 {
		t.Fatalf("Run = %+v, %v", res, err)
	}
	if len(r.rows("Inbox")) != 1 || len(r.rows("Receipts")) != 1 {
		t.Fatalf("rows %v: inbox=%d receipts=%d", r.paths(), len(r.rows("Inbox")), len(r.rows("Receipts")))
	}
	if c, _ := r.store.LoadAPICursor("acct-1", "folder:inbox-id"); c.Kind != graphCursorKind || c.Cursor == "" || c.Invalid {
		t.Fatalf("cursor = %+v", c)
	}
	// Unchanged second run: nothing fetched.
	r.fake.rawGets = nil
	if res, err = r.run(); err != nil || res.Added+res.Updated+res.Removed != 0 || len(r.fake.rawGets) != 0 {
		t.Fatalf("idle run = %+v, %v, raw=%v", res, err, r.fake.rawGets)
	}
	// Flag change: refreshed from metadata without refetching the body.
	m := r.fake.msgs["m1"]
	m.change, m.read, m.flagged = "k2", true, true
	if res, err = r.run(); err != nil || res.Updated != 1 || len(r.fake.rawGets) != 0 {
		t.Fatalf("flag run = %+v, %v, raw=%v", res, err, r.fake.rawGets)
	}
	if h := r.rows("Inbox")[0]; !h.IsRead || !h.IsStarred {
		t.Fatalf("flags not applied: %+v", h)
	}
	// Deleted upstream: a verified @removed entry removes the row.
	delete(r.fake.msgs, "m1")
	r.fake.removed["inbox-id"] = []string{"m1"}
	if res, err = r.run(); err != nil || res.Removed != 1 || len(r.rows("Inbox")) != 0 {
		t.Fatalf("remove run = %+v, %v, rows=%d", res, err, len(r.rows("Inbox")))
	}
}

func TestGraphSyncCreatesEmptyListedFolders(t *testing.T) {
	r := newGraphRig(t)
	if _, err := r.run(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"Inbox", "Receipts"} {
		if !r.hasFolder(path) {
			t.Errorf("listed empty folder %q was not created", path)
		}
	}
}

func TestGraphSyncMoveKeepsMessage(t *testing.T) {
	r := newGraphRig(t)
	r.add("m1", "inbox-id", "k1", plainMail("m1", "Hello", "body"))
	if _, err := r.run(); err != nil {
		t.Fatal(err)
	}
	r.fake.msgs["m1"].parent = "recv-id"
	r.fake.removed["inbox-id"] = []string{"m1"}
	if res, err := r.run(); err != nil || res.Removed != 0 {
		t.Fatalf("move run = %+v, %v", res, err)
	}
	if len(r.rows("Inbox")) != 0 || len(r.rows("Receipts")) != 1 {
		t.Fatalf("after move: inbox=%d receipts=%d", len(r.rows("Inbox")), len(r.rows("Receipts")))
	}
}

func TestGraphSyncExpiredDeltaReconcilesOneFolder(t *testing.T) {
	r := newGraphRig(t)
	r.add("m1", "inbox-id", "k1", plainMail("m1", "Hello", "body"))
	r.add("m2", "recv-id", "k1", plainMail("m2", "Receipt", "body"))
	if _, err := r.run(); err != nil {
		t.Fatal(err)
	}
	delete(r.fake.msgs, "m1") // vanished while the delta token was lost
	r.fake.expired["inbox-id"] = true
	r.fake.deltaHit = nil
	// Expire only the resumed request: the fresh enumeration must succeed.
	r.fake.setHook(func(req *http.Request) (int, string, map[string]string) {
		if strings.Contains(req.URL.Path, "inbox-id") && req.URL.Query().Get("$deltatoken") == "" {
			r.fake.expired["inbox-id"] = false
		}
		return 0, "", nil
	})
	res, err := r.run()
	if err != nil || res.Reconciled != 1 || res.Removed != 1 || len(r.rows("Inbox")) != 0 || len(r.rows("Receipts")) != 1 {
		t.Fatalf("recovery = %+v, %v", res, err)
	}
}

func TestGraphSyncFolderRenameAndDelete(t *testing.T) {
	r := newGraphRig(t)
	r.add("m2", "recv-id", "k1", plainMail("m2", "Receipt", "body"))
	if _, err := r.run(); err != nil {
		t.Fatal(err)
	}
	r.fake.folders["recv-id"] = "Bills"
	if res, err := r.run(); err != nil || res.Reconciled != 1 {
		t.Fatalf("rename run = %+v, %v", res, err)
	}
	if len(r.rows("Bills")) != 1 || r.hasFolder("Receipts") {
		t.Fatalf("rename: bills=%d oldFolder=%v", len(r.rows("Bills")), r.hasFolder("Receipts"))
	}
	delete(r.fake.folders, "recv-id")
	delete(r.fake.msgs, "m2")
	if _, err := r.run(); err != nil {
		t.Fatal(err)
	}
	if r.hasFolder("Bills") {
		t.Fatal("deleted folder still present")
	}
}

func TestGraphSyncRateLimitBackoff(t *testing.T) {
	r := newGraphRig(t)
	r.add("m1", "inbox-id", "k1", plainMail("m1", "Hello", "body"))
	n := 0
	r.fake.setHook(func(req *http.Request) (int, string, map[string]string) {
		if strings.HasSuffix(req.URL.Path, "/$value") && n < 2 {
			n++
			return 429, "{}", map[string]string{"Retry-After": "3"}
		}
		return 0, "", nil
	})
	if _, err := r.run(); err != nil || len(r.slept) != 2 || r.slept[0] != 3*time.Second || len(r.fails) != 0 {
		t.Fatalf("err=%v slept=%v fails=%v", err, r.slept, r.fails)
	}
	r.slept = nil
	r.fake.setHook(func(*http.Request) (int, string, map[string]string) { return 503, "{}", nil })
	if _, err := r.run(); err == nil || len(r.slept) != apiRetryLimit {
		t.Fatalf("persistent 503: err=%v slept=%v", err, r.slept)
	}
}

func TestGraphSyncSharedNeedsGrant(t *testing.T) {
	r := newGraphRig(t)
	r.g.Reader.Shared = "team@contoso.com"
	hit := false
	r.fake.setHook(func(*http.Request) (int, string, map[string]string) { hit = true; return 0, "", nil })
	if _, err := r.run(); err == nil || hit || len(r.fails) != 1 || r.fails[0] != oauth2.MailAPIFailurePartialConsent {
		t.Fatalf("err=%v requested=%v fails=%v", err, hit, r.fails)
	}
}

func TestGraphSyncAuthFailureReported(t *testing.T) {
	r := newGraphRig(t)
	r.fake.setHook(func(*http.Request) (int, string, map[string]string) {
		return 401, `{"error":{"code":"InvalidAuthenticationToken"}}`, nil
	})
	if _, err := r.run(); err == nil || len(r.fails) != 1 {
		t.Fatalf("err=%v fails=%v", err, r.fails)
	}
}

func TestGraphSyncRejectsOffOriginLink(t *testing.T) {
	r := newGraphRig(t)
	r.fake.setHook(func(req *http.Request) (int, string, map[string]string) {
		if strings.HasSuffix(req.URL.Path, "/delta") {
			return 200, `{"value":[],"@odata.nextLink":"https://evil.example/x"}`, nil
		}
		return 0, "", nil
	})
	if _, err := r.run(); err == nil || !strings.Contains(err.Error(), "origin") {
		t.Fatalf("err = %v", err)
	}
}

func TestGraphSyncCancellation(t *testing.T) {
	r := newGraphRig(t)
	r.add("m1", "inbox-id", "k1", plainMail("m1", "Hello", "body"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.g.Run(ctx); err == nil {
		t.Fatal("cancelled run succeeded")
	}
	if c, _ := r.store.LoadAPICursor("acct-1", "folder:inbox-id"); c.Cursor != "" {
		t.Fatalf("cursor stored after cancel: %+v", c)
	}
}

func TestGraphSyncCrashBeforeCursorIsIdempotent(t *testing.T) {
	r := newGraphRig(t)
	r.add("m1", "inbox-id", "k1", plainMail("m1", "Hello", "body"))
	// Fail the delta of the second folder after the first folder's page
	// committed; the retry must not duplicate rows.
	fail := true
	r.fake.setHook(func(req *http.Request) (int, string, map[string]string) {
		if fail && strings.Contains(req.URL.Path, "recv-id") && strings.HasSuffix(req.URL.Path, "/delta") {
			return 400, "{}", nil
		}
		return 0, "", nil
	})
	if _, err := r.run(); err == nil {
		t.Fatal("expected failure")
	}
	fail = false
	if _, err := r.run(); err != nil || len(r.rows("Inbox")) != 1 {
		t.Fatalf("retry: %v rows=%d", err, len(r.rows("Inbox")))
	}
}

func TestGraphSyncFixturesParseLikeIMAP(t *testing.T) {
	r := newGraphRig(t)
	latin := "From: =?ISO-8859-1?Q?Andr=E9?= <andre@example.com>\r\nTo: me@example.com\r\nSubject: =?ISO-8859-1?Q?Caf=E9?=\r\nMessage-ID: <latin@example.com>\r\nDate: Mon, 02 Jan 2026 10:00:00 +0000\r\nContent-Type: text/plain; charset=iso-8859-1\r\nContent-Transfer-Encoding: 8bit\r\n\r\ncaf\xe9 cr\xe8me\r\n"
	idn := "From: =?UTF-8?B?w5xtbGF1dA==?= <user@münchen.example>\r\nTo: me@example.com\r\nSubject: IDN\r\nMessage-ID: <idn@example.com>\r\nDate: Mon, 02 Jan 2026 10:00:00 +0000\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nhallo\r\n"
	signed := "From: s@example.com\r\nTo: me@example.com\r\nSubject: signed\r\nMessage-ID: <signed@example.com>\r\nDate: Mon, 02 Jan 2026 10:00:00 +0000\r\nMIME-Version: 1.0\r\nContent-Type: multipart/signed; protocol=\"application/pkcs7-signature\"; micalg=sha-256; boundary=\"B\"\r\n\r\n--B\r\nContent-Type: text/plain\r\n\r\nsigned text\r\n--B\r\nContent-Type: application/pkcs7-signature; name=smime.p7s\r\nContent-Transfer-Encoding: base64\r\n\r\nAAAA\r\n--B--\r\n"
	enc := "From: s@example.com\r\nTo: me@example.com\r\nSubject: secret\r\nMessage-ID: <enc@example.com>\r\nDate: Mon, 02 Jan 2026 10:00:00 +0000\r\nMIME-Version: 1.0\r\nContent-Type: application/pkcs7-mime; smime-type=enveloped-data; name=smime.p7m\r\nContent-Transfer-Encoding: base64\r\n\r\nAAAAAAAA\r\n"
	att := "From: s@example.com\r\nTo: me@example.com\r\nSubject: attached\r\nMessage-ID: <att@example.com>\r\nDate: Mon, 02 Jan 2026 10:00:00 +0000\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=\"X\"\r\n\r\n--X\r\nContent-Type: text/plain\r\n\r\nsee file\r\n--X\r\nContent-Type: application/pdf; name=\"r.pdf\"\r\nContent-Disposition: attachment; filename=\"r.pdf\"\r\nContent-Transfer-Encoding: base64\r\n\r\nJVBERg==\r\n--X--\r\n"
	for i, raw := range map[string]string{"latin": latin, "idn": idn, "signed": signed, "enc": enc, "att": att} {
		r.add(i, "inbox-id", "k", raw)
	}
	if _, err := r.run(); err != nil {
		t.Fatal(err)
	}
	byID := map[string]*message.Message{}
	for _, h := range r.rows("Inbox") {
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
