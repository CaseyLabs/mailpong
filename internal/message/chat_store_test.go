package message

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// chatFixture seeds an inbox and a Sent folder:
//
//	A  alice, 2 messages, latest unread       (priority)
//	B  news@shop.example, bulk, unread         (low)
//	C  bob, read; my Sent reply is newer      (priority, last from me)
//	D  deals@shop.example, bulk, read         (low)
//	E  list mail followed by a human reply    (priority: not every message is bulk)
func chatFixture(t *testing.T) (*Store, string, string, time.Time) {
	t.Helper()
	s, accountID, inboxID := newBodyFailedTestStore(t)
	if _, err := s.db.Exec(`INSERT INTO folders (id, account_id, name, path, folder_type) VALUES ('sent-1', ?, 'Sent', 'Sent', 'sent')`, accountID); err != nil {
		t.Fatalf("seed sent folder: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	bulk, human := true, false
	seed := []struct {
		id, folder, thread, from string
		age                      time.Duration
		read                     bool
		isBulk                   *bool
	}{
		{"a1", inboxID, "<a@x>", "alice@example.com", 3 * time.Hour, true, &human},
		{"a2", inboxID, "<a@x>", "alice@example.com", 1 * time.Hour, false, &human},
		{"b1", inboxID, "", "news@shop.example", 2 * time.Hour, false, &bulk},
		{"c1", inboxID, "<c@x>", "bob@example.com", 4 * time.Hour, true, nil},
		{"s1", "sent-1", "<c@x>", "test@example.com", 30 * time.Minute, true, nil},
		{"d1", inboxID, "<d@x>", "deals@shop.example", 5 * time.Hour, true, &bulk},
		{"e1", inboxID, "<e@x>", "list@groups.example", 7 * time.Hour, true, &bulk},
		{"e2", inboxID, "<e@x>", "carol@example.com", 6 * time.Hour, true, &human},
	}
	for i, m := range seed {
		err := s.Create(&Message{
			ID: m.id, AccountID: accountID, FolderID: m.folder, UID: uint32(i + 1),
			ThreadID: m.thread, Subject: m.id, FromEmail: m.from,
			Date: now.Add(-m.age), ReceivedAt: now.Add(-m.age), IsRead: m.read, IsBulk: m.isBulk,
		})
		if err != nil {
			t.Fatalf("seed %s: %v", m.id, err)
		}
	}
	splitSenders(t, s)
	return s, accountID, inboxID, now
}

// splitSenders shows every sender with mail so far as separate threads, so
// tests can check thread chats; senders combine by default.
func splitSenders(t *testing.T, s *Store) {
	t.Helper()
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO sender_chat (account_id, email, combined)
		SELECT DISTINCT f.account_id, LOWER(m.from_email), 0
		FROM messages m INNER JOIN folders f ON m.folder_id = f.id
		WHERE COALESCE(m.from_email, '') <> ''`)
	if err != nil {
		t.Fatalf("split senders: %v", err)
	}
}

func chatKeys(chats []*Chat) []string {
	keys := make([]string, 0, len(chats))
	for _, c := range chats {
		keys = append(keys, c.ThreadKey)
	}
	return keys
}

func ptr(t time.Time) *time.Time { return &t }

func TestListChatsSections(t *testing.T) {
	s, accountID, inboxID, now := chatFixture(t)

	// Pin C; snooze A for an hour; B's snooze already ended.
	states := map[string]ChatState{
		SenderChatPrefix + "carol@example.com": {SnoozedUntil: ptr(now.Add(3 * time.Hour)), SnoozedAt: ptr(now.Add(-time.Hour))},
		"c@x":                                  {PinnedAt: ptr(now)},
		"a@x":                                  {SnoozedUntil: ptr(now.Add(time.Hour)), SnoozedAt: ptr(now)},
		"b1":                                   {SnoozedUntil: ptr(now.Add(-time.Minute)), SnoozedAt: ptr(now.Add(-time.Hour))},
	}
	for key, st := range states {
		if err := s.SetChatState(accountID, key, st); err != nil {
			t.Fatalf("SetChatState(%s): %v", key, err)
		}
	}

	tests := []struct {
		scope, section string
		want           []string
	}{
		{"", ChatSectionAll, []string{"c@x", "b1", "d@x", "e@x"}},
		{"", ChatSectionUnread, []string{"b1"}},
		{"", ChatSectionPriority, []string{"c@x", "e@x"}},
		{"", ChatSectionLow, []string{"b1", "d@x"}},
		{"", ChatSectionSnoozed, []string{"a@x"}},
		{inboxID, ChatSectionAll, []string{"c@x", "b1", "d@x", "e@x"}},
		{"sent-1", ChatSectionAll, []string{"c@x"}},
	}
	for _, tt := range tests {
		t.Run(tt.scope+"/"+tt.section, func(t *testing.T) {
			chats, err := s.ListChats(tt.scope, tt.section, now, 0, 50)
			if err != nil {
				t.Fatalf("ListChats: %v", err)
			}
			if got := chatKeys(chats); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("keys = %v, want %v", got, tt.want)
			}
			n, err := s.CountChats(tt.scope, tt.section, now)
			if err != nil {
				t.Fatalf("CountChats: %v", err)
			}
			if n != len(tt.want) {
				t.Errorf("count = %d, want %d", n, len(tt.want))
			}
		})
	}

	if _, err := s.ListChats("", "bogus", now, 0, 50); err == nil {
		t.Error("unknown section: want error")
	}
}

func TestListChatsFlags(t *testing.T) {
	s, accountID, _, now := chatFixture(t)
	if err := s.SetChatState(accountID, "<c@x>", ChatState{PinnedAt: ptr(now)}); err != nil {
		t.Fatal(err)
	}
	chats, err := s.ListChats("", ChatSectionAll, now, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]*Chat{}
	for _, c := range chats {
		byKey[c.ThreadKey] = c
	}

	tests := []struct {
		key                  string
		pinned, low          bool
		unread, messageCount int
	}{
		{"a@x", false, false, 1, 2},
		{"b1", false, true, 1, 1},
		{"c@x", true, false, 0, 1},
		{"d@x", false, true, 0, 1},
		{"e@x", false, false, 0, 2},
	}
	for _, tt := range tests {
		c := byKey[tt.key]
		if c == nil {
			t.Errorf("%s: missing", tt.key)
			continue
		}
		if c.IsPinned != tt.pinned || c.IsLowPriority != tt.low ||
			c.UnreadCount != tt.unread || c.MessageCount != tt.messageCount {
			t.Errorf("%s: pinned=%v low=%v unread=%d count=%d, want %v %v %d %d",
				tt.key, c.IsPinned, c.IsLowPriority, c.UnreadCount, c.MessageCount,
				tt.pinned, tt.low, tt.unread, tt.messageCount)
		}
	}
}

func TestSenderCategoryOverride(t *testing.T) {
	s, accountID, inboxID, now := chatFixture(t)

	lowKeys := func() []string {
		t.Helper()
		chats, err := s.ListChats("", ChatSectionLow, now, 0, 50)
		if err != nil {
			t.Fatal(err)
		}
		return chatKeys(chats)
	}

	steps := []struct {
		email, category string
		want            []string
	}{
		{"News@Shop.example", SenderCategoryPriority, []string{"d@x"}},
		{"carol@example.com", SenderCategoryLow, []string{"d@x", "e@x"}},
		{"news@shop.example", "", []string{"b1", "d@x", "e@x"}},
	}
	for _, st := range steps {
		if err := s.SetSenderCategory(accountID, st.email, st.category); err != nil {
			t.Fatalf("SetSenderCategory(%s, %q): %v", st.email, st.category, err)
		}
		if got := lowKeys(); !reflect.DeepEqual(got, st.want) {
			t.Errorf("after %s=%q: low = %v, want %v", st.email, st.category, got, st.want)
		}
	}

	// The override also applies to mail that arrives later, bulk or not.
	if err := s.SetSenderCategory(accountID, "promo@shop.example", SenderCategoryLow); err != nil {
		t.Fatal(err)
	}
	if err := s.Create(&Message{ID: "p1", AccountID: accountID, FolderID: inboxID, UID: 100, FromEmail: "promo@shop.example", Date: now}); err != nil {
		t.Fatal(err)
	}
	splitSenders(t, s)
	if got := lowKeys(); !reflect.DeepEqual(got, []string{"p1", "b1", "d@x", "e@x"}) {
		t.Errorf("new mail from low sender: low = %v", got)
	}

	if err := s.SetSenderCategory(accountID, "x@y", "spam"); err == nil {
		t.Error("invalid category: want error")
	}
}

func TestSnoozeWakesOnNewMail(t *testing.T) {
	s, accountID, inboxID, now := chatFixture(t)
	if err := s.SetChatState(accountID, "a@x", ChatState{SnoozedUntil: ptr(now.Add(time.Hour)), SnoozedAt: ptr(now)}); err != nil {
		t.Fatal(err)
	}

	// A reply moved into the Sent folder doesn't wake it; new inbox mail does.
	for _, m := range []*Message{
		{ID: "s2", FolderID: "sent-1", UID: 200, ThreadID: "<a@x>", FromEmail: "test@example.com", Date: now, ReceivedAt: now.Add(2 * time.Second)},
		{ID: "a3", FolderID: inboxID, UID: 201, ThreadID: "<a@x>", FromEmail: "alice@example.com", Date: now, ReceivedAt: now.Add(2 * time.Second)},
	} {
		m.AccountID = accountID
		snoozed, err := s.ListChats("", ChatSectionSnoozed, now, 0, 50)
		if err != nil {
			t.Fatal(err)
		}
		if got := chatKeys(snoozed); !reflect.DeepEqual(got, []string{"a@x"}) {
			t.Fatalf("before %s: snoozed = %v", m.ID, got)
		}
		if err := s.Create(m); err != nil {
			t.Fatal(err)
		}
	}

	snoozed, err := s.ListChats("", ChatSectionSnoozed, now, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(snoozed) != 0 {
		t.Errorf("after new mail: snoozed = %v, want none", chatKeys(snoozed))
	}

	due, err := s.ListDueSnoozes(now.Add(2 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 1 || due[0].ThreadKey != "a@x" || !due[0].WokeByMail || due[0].LatestMessageID != "a3" {
		t.Errorf("due = %+v, want a@x woken by mail with latest a3", due)
	}
}

func TestDueSnoozesAndNextWake(t *testing.T) {
	s, accountID, _, now := chatFixture(t)

	if next, err := s.NextSnoozeWake(); err != nil || next != nil {
		t.Fatalf("NextSnoozeWake with none = %v, %v", next, err)
	}

	for key, until := range map[string]time.Time{"b1": now.Add(-time.Minute), "d@x": now.Add(time.Hour)} {
		if err := s.SetChatState(accountID, key, ChatState{SnoozedUntil: ptr(until), SnoozedAt: ptr(now.Add(-2 * time.Hour))}); err != nil {
			t.Fatal(err)
		}
	}
	next, err := s.NextSnoozeWake()
	if err != nil || next == nil || !next.Equal(now.Add(-time.Minute)) {
		t.Fatalf("NextSnoozeWake = %v, %v", next, err)
	}

	due, err := s.ListDueSnoozes(now)
	if err != nil {
		t.Fatal(err)
	}
	want := []DueSnooze{{AccountID: accountID, ThreadKey: "b1", LatestMessageID: "b1", FolderID: "folder-1",
		ThreadID: "b1", Subject: "b1", FromEmail: "news@shop.example"}}
	if !reflect.DeepEqual(due, want) {
		t.Errorf("due = %+v, want %+v", due, want)
	}
}

func TestChatStateRoundTripAndCleanup(t *testing.T) {
	s, accountID, _, now := chatFixture(t)

	st := ChatState{PinnedAt: ptr(now), SnoozedUntil: ptr(now.Add(time.Hour)), SnoozedAt: ptr(now)}
	if err := s.SetChatState(accountID, "<a@x>", st); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetChatState(accountID, "a@x")
	if err != nil || !reflect.DeepEqual(got, st) {
		t.Fatalf("GetChatState = %+v, %v; want %+v", got, err, st)
	}

	// Clearing the snooze drops snoozed_at too; clearing everything deletes the row.
	if err := s.SetChatState(accountID, "a@x", ChatState{PinnedAt: st.PinnedAt, SnoozedAt: st.SnoozedAt}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetChatState(accountID, "a@x"); got.SnoozedAt != nil || got.PinnedAt == nil {
		t.Errorf("after unsnooze: %+v", got)
	}
	if err := s.SetChatState(accountID, "a@x", ChatState{}); err != nil {
		t.Fatal(err)
	}
	var rows int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM conversation_state`).Scan(&rows)
	if rows != 0 {
		t.Errorf("rows after clearing = %d, want 0", rows)
	}

	// Cleanup keeps threads with messages anywhere (including Sent-only or
	// archived) and drops threads with none.
	for _, key := range []string{"c@x", "b1", "gone@x"} {
		if err := s.SetChatState(accountID, key, ChatState{PinnedAt: ptr(now)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.DeleteBatch([]string{"c1"}); err != nil {
		t.Fatal(err)
	}
	n, err := s.CleanupChatState()
	if err != nil || n != 1 {
		t.Fatalf("CleanupChatState = %d, %v; want 1", n, err)
	}
	for key, want := range map[string]bool{"c@x": true, "b1": true, "gone@x": false} {
		got, _ := s.GetChatState(accountID, key)
		if (got.PinnedAt != nil) != want {
			t.Errorf("%s kept = %v, want %v", key, got.PinnedAt != nil, want)
		}
	}
}

func TestBulkClassificationStorage(t *testing.T) {
	s, accountID, inboxID, now := chatFixture(t)

	pending, err := s.ListUnclassifiedBulk(inboxID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].ID != "c1" {
		t.Fatalf("unclassified = %+v, want only c1", pending)
	}
	if err := s.SetBulkFlags(map[string]bool{"c1": true}); err != nil {
		t.Fatal(err)
	}
	if pending, _ := s.ListUnclassifiedBulk(inboxID, 0, 10); len(pending) != 0 {
		t.Errorf("after SetBulkFlags: unclassified = %+v", pending)
	}

	// A header re-sync without a classification keeps the stored one.
	if err := s.Upsert(&Message{ID: "c1", AccountID: accountID, FolderID: inboxID, UID: 4, ThreadID: "<c@x>", FromEmail: "bob@example.com", Date: now}); err != nil {
		t.Fatal(err)
	}
	var isBulk *bool
	if err := s.db.QueryRow(`SELECT is_bulk FROM messages WHERE id = 'c1'`).Scan(&isBulk); err != nil || isBulk == nil || !*isBulk {
		t.Errorf("is_bulk after upsert = %v, %v; want true", isBulk, err)
	}
}

func TestListChatsNullSubjectAndSnoozeStart(t *testing.T) {
	s, accountID, _, now := chatFixture(t)

	if _, err := s.db.Exec(`UPDATE messages SET subject = NULL WHERE id = 'b1'`); err != nil {
		t.Fatal(err)
	}
	chats, err := s.ListChats("", ChatSectionAll, now, 0, 50)
	if err != nil {
		t.Fatalf("ListChats with NULL subject: %v", err)
	}
	if len(chats) == 0 {
		t.Fatal("no chats listed")
	}

	// A snooze stored without a start gets one, so existing mail doesn't end it.
	if err := s.SetChatState(accountID, "a@x", ChatState{SnoozedUntil: ptr(now.Add(time.Hour))}); err != nil {
		t.Fatal(err)
	}
	snoozed, err := s.ListChats("", ChatSectionSnoozed, now, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if got := chatKeys(snoozed); !reflect.DeepEqual(got, []string{"a@x"}) {
		t.Errorf("snoozed = %v, want [a@x]", got)
	}

	// A due snooze on a NULL-subject thread still lists, and a thread with no
	// received_at stays snoozed instead of dropping out of every section.
	if err := s.SetChatState(accountID, "b1", ChatState{SnoozedUntil: ptr(now.Add(-time.Minute)), SnoozedAt: ptr(now.Add(-time.Hour))}); err != nil {
		t.Fatal(err)
	}
	if due, err := s.ListDueSnoozes(now); err != nil || len(due) != 1 || due[0].Subject != "" {
		t.Errorf("ListDueSnoozes with NULL subject = %+v, %v", due, err)
	}
	if _, err := s.db.Exec(`UPDATE messages SET received_at = NULL WHERE id IN ('a1', 'a2')`); err != nil {
		t.Fatal(err)
	}
	snoozed, err = s.ListChats("", ChatSectionSnoozed, now, 0, 50)
	if err != nil || !reflect.DeepEqual(chatKeys(snoozed), []string{"a@x"}) {
		t.Errorf("snoozed with NULL received_at = %v, %v", chatKeys(snoozed), err)
	}
}

func TestListNewMail(t *testing.T) {
	s, accountID, _, _ := chatFixture(t)
	// Force alice low to check the sender override path.
	if err := s.SetSenderCategory(accountID, "Alice@Example.com", SenderCategoryLow); err != nil {
		t.Fatalf("SetSenderCategory: %v", err)
	}
	type row struct {
		subject string
		low     bool
	}
	tests := []struct {
		name string
		ids  []string
		want []row
	}{
		{"none", nil, nil},
		{"highest uid first, unknown skipped", []string{"a1", "e2", "missing", "b1"},
			[]row{{"e2", false}, {"b1", true}, {"a1", true}}},
		{"bulk flag", []string{"d1"}, []row{{"d1", true}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.ListNewMail(tt.ids)
			if err != nil {
				t.Fatalf("ListNewMail: %v", err)
			}
			var rows []row
			for _, m := range got {
				rows = append(rows, row{m.Subject, m.IsLow})
			}
			if !reflect.DeepEqual(rows, tt.want) {
				t.Errorf("ListNewMail(%v) = %v, want %v", tt.ids, rows, tt.want)
			}
		})
	}
}

func TestListNewMailChunks(t *testing.T) {
	s, accountID, inboxID, now := chatFixture(t)
	ids := make([]string, 0, newMailChunk+20)
	for i := range newMailChunk + 20 {
		id := fmt.Sprintf("bulk-%d", i)
		if err := s.Create(&Message{ID: id, AccountID: accountID, FolderID: inboxID, UID: uint32(100 + i), Subject: id, Date: now, ReceivedAt: now}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
		ids = append(ids, id)
	}
	got, err := s.ListNewMail(ids)
	if err != nil {
		t.Fatalf("ListNewMail: %v", err)
	}
	if len(got) != len(ids) {
		t.Fatalf("ListNewMail returned %d rows, want %d", len(got), len(ids))
	}
	if got[0].UID != uint32(100+len(ids)-1) || got[len(got)-1].UID != 100 {
		t.Errorf("rows not sorted by UID across chunks: first %d, last %d", got[0].UID, got[len(got)-1].UID)
	}
}

func TestListChatsSentRecipients(t *testing.T) {
	s, accountID, _, now := chatFixture(t)
	err := s.Create(&Message{
		ID: "s2", AccountID: accountID, FolderID: "sent-1", UID: 99, ThreadID: "<s2@x>",
		FromEmail: "test@example.com", ToList: `[{"name":"Dana","email":"dana@example.com"}]`,
		Date: now, ReceivedAt: now, IsRead: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	chats, err := s.ListChats("sent-1", ChatSectionAll, now, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range chats {
		if c.ThreadKey != "s2@x" {
			continue
		}
		if want := []Address{{"Dana", "dana@example.com"}}; !reflect.DeepEqual(c.Recipients, want) {
			t.Errorf("recipients = %v, want %v", c.Recipients, want)
		}
		return
	}
	t.Errorf("s2@x missing from %v", chatKeys(chats))
}

func TestTrimSubjectPrefix(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Budget", "Budget"},
		{"Re: Budget", "Budget"},
		{"RE: Fwd: re[2]: Budget", "Budget"},
		{"  AW:Budget ", "Budget"},
		{"Re: ", ""},
		{"Reply: Budget", "Reply: Budget"},
	}
	for _, tt := range tests {
		if got := trimSubjectPrefix(tt.in); got != tt.want {
			t.Errorf("trimSubjectPrefix(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSearchChatsHighlightSkipsPrefix(t *testing.T) {
	s, accountID, inboxID, now := chatFixture(t)
	if err := s.Create(&Message{ID: "h1", AccountID: accountID, FolderID: inboxID, UID: 300, ThreadID: "<h@x>",
		Subject: "Re: budget review", FromEmail: "dana@example.com", Date: now, ReceivedAt: now}); err != nil {
		t.Fatal(err)
	}
	results, err := s.SearchChats(inboxID, "re", 0, 50, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.ThreadID != "<h@x>" {
			continue
		}
		if want := "budget <mark>re</mark>view"; r.HighlightedSubject != want {
			t.Errorf("highlighted subject = %q, want %q", r.HighlightedSubject, want)
		}
		return
	}
	t.Fatal("search did not find the thread")
}

func TestSearchChatsSentRecipients(t *testing.T) {
	s, accountID, inboxID, now := chatFixture(t)
	seed := []Message{
		{ID: "q1", FolderID: "sent-1", ThreadID: "<q@x>", Subject: "quarterly report", ToList: `[{"name":"Dana","email":"dana@example.com"}]`},
		{ID: "q2", FolderID: "sent-1", ThreadID: "<q@x>", Subject: "Re: quarterly report", ToList: `[{"name":"Dana","email":"DANA@example.com"},{"name":"Eve","email":"eve@example.com"}]`},
		{ID: "q3", FolderID: inboxID, ThreadID: "<q@x>", Subject: "Re: quarterly report", FromEmail: "dana@example.com", ToList: `[{"name":"Me","email":"test@example.com"}]`},
		{ID: "r1", FolderID: "sent-1", Subject: "roadmap draft", ToList: `not json`},
	}
	for i, m := range seed {
		m.AccountID, m.UID, m.Date, m.ReceivedAt, m.IsRead = accountID, uint32(100+i), now, now, true
		if m.FromEmail == "" {
			m.FromEmail = "test@example.com"
		}
		if err := s.Create(&m); err != nil {
			t.Fatalf("seed %s: %v", m.ID, err)
		}
	}

	tests := []struct {
		name, folder, query string
		want                map[string][]Address // thread ID -> recipients
	}{
		{"sent thread merges recipients", "sent-1", "quarterly", map[string][]Address{
			"<q@x>": {{"Dana", "dana@example.com"}, {"Eve", "eve@example.com"}},
		}},
		{"invalid to list", "sent-1", "roadmap", map[string][]Address{"r1": nil}},
		{"inbox has none", inboxID, "quarterly", map[string][]Address{"<q@x>": nil}},
		{"no match", "sent-1", "nothingmatches", map[string][]Address{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := s.SearchChats(tt.folder, tt.query, 0, 50, "")
			if err != nil {
				t.Fatal(err)
			}
			got := make(map[string][]Address, len(results))
			for _, r := range results {
				got[r.ThreadID] = r.Recipients
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("recipients = %v, want %v", got, tt.want)
			}
		})
	}
}

// chatLine summarizes a chat for comparing ListChats with the one-query list.
func chatLine(account, key string, count, unread int, low, pinned bool, ids []string) string {
	ids = slices.Sorted(slices.Values(ids))
	return fmt.Sprint(account, "|", key, "|", count, "|", unread, "|", low, "|", pinned, "|", ids)
}

// listChatsOneQuery is ListChats before it ran in two passes: one grouped
// query computing every aggregate before LIMIT. It returns a chatLine per
// chat.
func listChatsOneQuery(t *testing.T, s *Store, scope, section string, now time.Time, offset, limit int) []string {
	t.Helper()
	senderChats, err := s.hasSenderChats()
	if err != nil {
		t.Fatal(err)
	}
	folderCond, args := chatFolderCond(scope)
	where, err := chatSectionWhere(section)
	if err != nil {
		t.Fatal(err)
	}
	args = append(args, now.Unix())
	from := ` FROM (` + chatGroups(folderCond, chatListColumns, "1", senderChats) + `) g` + chatStateJoin + ` WHERE ` + where
	rows, err := s.db.Query(`SELECT g.thread_key, g.account_id, g.message_count, g.unread_count, g.is_low, cs.pinned_at IS NOT NULL,
		g.message_ids`+from+chatOrder(section)+" LIMIT ? OFFSET ?", append(args, limit, offset)...)
	if err != nil {
		t.Fatalf("one-query list: %v", err)
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var key, account, ids string
		var count, unread int
		var low, pinned bool
		if err := rows.Scan(&key, &account, &count, &unread, &low, &pinned, &ids); err != nil {
			t.Fatalf("scan one-query list: %v", err)
		}
		lines = append(lines, chatLine(account, key, count, unread, low, pinned, strings.Split(ids, ",")))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return lines
}

// TestListChatsMatchesOneQuery checks the two-pass ListChats against the
// one-query list on a mixed fixture with pins and snoozes, for every section,
// scope, combine setting and page.
func TestListChatsMatchesOneQuery(t *testing.T) {
	s, accountID, inboxID, now := chatFixture(t)
	if _, err := s.db.Exec(`DELETE FROM sender_chat`); err != nil {
		t.Fatal(err)
	}
	// More mail: alice and carol start several threads, so sender chats merge
	// threads; a human reply lifts a bulk thread; one unthreaded message;
	// one thread key that threadKeysMatch cannot find.
	extra := []struct {
		id, thread, from string
		age              time.Duration
		read             bool
	}{
		{"a3", "<a2@x>", "alice@example.com", 90 * time.Minute, false},
		{"a4", "<a2@x>", "bob@example.com", 80 * time.Minute, true},
		{"a5", "<a3@x>", "alice@example.com", 8 * time.Hour, true},
		{"c2", "<c2@x>", "carol@example.com", 9 * time.Hour, false},
		{"c3", "", "carol@example.com", 10 * time.Hour, true},
		{"b2", "<b2@x>", "news@shop.example", 11 * time.Hour, false},
		{"t1", "<t1@x>", "test@example.com", 12 * time.Hour, true},
		// A broken Message-ID: its thread key drops the inner brackets.
		{"w1", "w<1>@x", "walt@example.com", 13 * time.Hour, false},
	}
	for i, m := range extra {
		err := s.Create(&Message{
			ID: m.id, AccountID: accountID, FolderID: inboxID, UID: uint32(100 + i),
			ThreadID: m.thread, Subject: m.id, FromEmail: m.from,
			Date: now.Add(-m.age), ReceivedAt: now.Add(-m.age), IsRead: m.read,
		})
		if err != nil {
			t.Fatalf("seed %s: %v", m.id, err)
		}
	}

	states := map[string]ChatState{
		"c@x":                                  {PinnedAt: ptr(now)},
		SenderChatPrefix + "alice@example.com": {PinnedAt: ptr(now.Add(-time.Hour))},
		"a@x":                                  {SnoozedUntil: ptr(now.Add(time.Hour)), SnoozedAt: ptr(now)},
		"c2@x":                                 {SnoozedUntil: ptr(now.Add(2 * time.Hour)), SnoozedAt: ptr(now.Add(-time.Hour))},
		"b1":                                   {SnoozedUntil: ptr(now.Add(-time.Minute)), SnoozedAt: ptr(now.Add(-time.Hour))},
	}
	for key, st := range states {
		if err := s.SetChatState(accountID, key, st); err != nil {
			t.Fatalf("SetChatState(%s): %v", key, err)
		}
	}

	sections := []string{ChatSectionAll, ChatSectionUnread, ChatSectionPriority, ChatSectionLow, ChatSectionSnoozed}
	for _, combine := range []string{"true", "false"} {
		if _, err := s.db.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('chat_combine_senders', ?)`, combine); err != nil {
			t.Fatal(err)
		}
		for _, scope := range []string{"", inboxID, "sent-1"} {
			for _, section := range sections {
				for _, page := range [][2]int{{0, 50}, {0, 2}, {2, 2}, {1, 1}} {
					name := fmt.Sprintf("combine=%s/%q/%s/%d+%d", combine, scope, section, page[0], page[1])
					chats, err := s.ListChats(scope, section, now, page[0], page[1])
					if err != nil {
						t.Fatalf("%s: ListChats: %v", name, err)
					}
					var got []string
					for _, c := range chats {
						got = append(got, chatLine(c.AccountID, c.ThreadKey, c.MessageCount, c.UnreadCount, c.IsLowPriority, c.IsPinned, c.MessageIDs))
					}
					if want := listChatsOneQuery(t, s, scope, section, now, page[0], page[1]); !reflect.DeepEqual(got, want) {
						t.Errorf("%s:\n got %v\nwant %v", name, got, want)
					}
				}
			}
		}
	}
}

// TestThreadStartersTriggers checks that the thread_starters cache matches a
// recomputation from messages after each kind of write, and that its
// refreshes find a thread's messages through the index.
func TestThreadStartersTriggers(t *testing.T) {
	s, accountID, inboxID := newBodyFailedTestStore(t)
	if _, err := s.db.Exec(`INSERT INTO folders (id, account_id, name, path, folder_type) VALUES ('other', ?, 'Other', 'Other', 'folder')`, accountID); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	insert := `INSERT INTO messages (id, account_id, folder_id, uid, thread_id, from_email, date) VALUES (?, ?, ?, ?, ?, ?, ?)`
	steps := []struct {
		name string
		sql  string
		args []any
	}{
		{"first", insert, []any{"a", accountID, inboxID, 1, "<t@x>", "Bob@x", base.Add(time.Hour)}},
		{"later", insert, []any{"b", accountID, inboxID, 2, "t@x", "carol@x", base.Add(2 * time.Hour)}},
		{"earlier", insert, []any{"c", accountID, inboxID, 3, "<t@x>", "dave@x", base}},
		{"null date", insert, []any{"d", accountID, inboxID, 4, "<u@x>", "erin@x", nil}},
		{"unthreaded", insert, []any{"e", accountID, inboxID, 5, nil, "frank@x", base}},
		{"broken id", insert, []any{"f", accountID, inboxID, 6, "w<1>@x", "gail@x", base}},
		{"same day", insert, []any{"g", accountID, inboxID, 7, "<u@x>", "hal@x", nil}},
		{"upsert rethreads", insert + ` ON CONFLICT(folder_id, uid) DO UPDATE SET thread_id = excluded.thread_id`, []any{"c", accountID, inboxID, 3, "<v@x>", "dave@x", base}},
		{"date change", `UPDATE messages SET date = ? WHERE id = 'b'`, []any{base.Add(-time.Hour)}},
		{"sender change", `UPDATE messages SET from_email = 'Zed@x' WHERE id = 'b'`, nil},
		{"read change", `UPDATE messages SET is_read = 1 WHERE id = 'b'`, nil},
		{"move", `UPDATE messages SET folder_id = 'other' WHERE id = 'b'`, nil},
		{"delete starter", `DELETE FROM messages WHERE id = 'd'`, nil},
		{"delete other", `DELETE FROM messages WHERE id = 'a'`, nil},
		{"delete last", `DELETE FROM messages WHERE id = 'e'`, nil},
		{"delete folder", `DELETE FROM folders WHERE id = 'other'`, nil},
	}
	key := threadKeyExpr("")
	recompute := `SELECT folder_id, thread_key, starter_id, from_email FROM (
		SELECT folder_id, ` + key + ` AS thread_key, id AS starter_id, LOWER(COALESCE(from_email, '')) AS from_email,
			ROW_NUMBER() OVER (PARTITION BY folder_id, ` + key + ` ORDER BY date, id) AS rn
		FROM messages) WHERE rn = 1 ORDER BY 1, 2`
	cached := `SELECT folder_id, thread_key, starter_id, from_email FROM thread_starters ORDER BY 1, 2`
	rows := func(q string) []string {
		r, err := s.db.Query(q)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		var out []string
		for r.Next() {
			var f, k, id, email string
			if err := r.Scan(&f, &k, &id, &email); err != nil {
				t.Fatal(err)
			}
			out = append(out, f+"|"+k+"|"+id+"|"+email)
		}
		return out
	}
	for _, st := range steps {
		if _, err := s.db.Exec(st.sql, st.args...); err != nil {
			t.Fatalf("%s: %v", st.name, err)
		}
		if got, want := rows(cached), rows(recompute); !reflect.DeepEqual(got, want) {
			t.Errorf("%s:\n got %v\nwant %v", st.name, got, want)
		}
	}

	r, err := s.db.Query(`EXPLAIN QUERY PLAN
		SELECT id FROM messages WHERE folder_id = ? AND `+key+` = ? ORDER BY date, id LIMIT 1`, inboxID, "t@x")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var plan string
	for r.Next() {
		var id, parent, unused int
		var detail string
		if err := r.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan += detail + "; "
	}
	if !strings.Contains(plan, "idx_messages_folder_thread_key") {
		t.Errorf("refresh lookup does not use the index: %s", plan)
	}
}
