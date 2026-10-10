package message

import (
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

const lms = "noreply@lms.example"

var lmsKey = SenderChatKey(lms)

// senderChatFixture extends chatFixture with mail from an automated sender:
// three unthreaded notifications (n1-n3), a thread it started with a human
// reply (t1, t2), a thread alice started that it replied to (o1, o2), and a
// notification in a second account (n9).
func senderChatFixture(t *testing.T) (*Store, string, string, time.Time) {
	t.Helper()
	s, accountID, inboxID, now := chatFixture(t)
	for _, q := range []string{
		`INSERT INTO accounts (id, name, email, imap_host, smtp_host, username) VALUES ('acct-2', 'Two', 'two@example.com', 'imap', 'smtp', 'two')`,
		`INSERT INTO folders (id, account_id, name, path, folder_type) VALUES ('folder-2', 'acct-2', 'INBOX', 'INBOX', 'inbox')`,
	} {
		if _, err := s.db.Exec(q); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	bulk, human := true, false
	seed := []struct {
		id, account, folder, thread, from, subject string
		age                                        time.Duration
		read                                       bool
		isBulk                                     *bool
	}{
		{"n1", accountID, inboxID, "", "NoReply@lms.example", "Grade posted: Math", 20 * time.Hour, false, &bulk},
		{"n2", accountID, inboxID, "", lms, "Grade posted: Art", 19 * time.Hour, false, &bulk},
		{"n3", accountID, inboxID, "", lms, "Quiz due", 18 * time.Hour, true, &bulk},
		{"t1", accountID, inboxID, "<t@x>", lms, "Forum post", 17 * time.Hour, true, &bulk},
		{"t2", accountID, inboxID, "<t@x>", "carol@example.com", "Re: Forum post", 16 * time.Hour, false, &human},
		{"o1", accountID, inboxID, "<o@x>", "alice@example.com", "Question", 15 * time.Hour, true, &human},
		{"o2", accountID, inboxID, "<o@x>", lms, "Re: Question", 14 * time.Hour, true, &bulk},
		{"n9", "acct-2", "folder-2", "", lms, "Grade posted: Two", 20 * time.Hour, false, &bulk},
	}
	for i, m := range seed {
		err := s.Create(&Message{
			ID: m.id, AccountID: m.account, FolderID: m.folder, UID: uint32(200 + i),
			ThreadID: m.thread, Subject: m.subject, FromEmail: m.from,
			Date: now.Add(-m.age), ReceivedAt: now.Add(-m.age), IsRead: m.read, IsBulk: m.isBulk,
		})
		if err != nil {
			t.Fatalf("seed %s: %v", m.id, err)
		}
	}
	splitSenders(t, s)
	return s, accountID, inboxID, now
}

func chatByKey(chats []*Chat, key string) *Chat {
	for _, c := range chats {
		if c.ThreadKey == key {
			return c
		}
	}
	return nil
}

func TestSenderChatFlag(t *testing.T) {
	s, accountID, _, _ := senderChatFixture(t)
	if err := s.SetSenderChat(accountID, "  ", true); err == nil {
		t.Error("SetSenderChat with empty email: want error")
	}
	steps := []struct {
		email    string
		combined bool
		want     bool
	}{
		{" NoReply@LMS.example ", true, true},
		{lms, true, true}, // idempotent
		{"NOREPLY@lms.example", false, false},
		{lms, false, false},
	}
	for _, st := range steps {
		if err := s.SetSenderChat(accountID, st.email, st.combined); err != nil {
			t.Fatalf("SetSenderChat(%q, %v): %v", st.email, st.combined, err)
		}
		got, err := s.IsSenderChat(accountID, lms)
		if err != nil || got != st.want {
			t.Errorf("after SetSenderChat(%q, %v): IsSenderChat = %v, %v; want %v", st.email, st.combined, got, err, st.want)
		}
	}
}

func TestSenderChatMapping(t *testing.T) {
	s, accountID, _, now := senderChatFixture(t)
	threadKeys := []string{"n1", "n2", "n3", "t@x", "o@x"}

	list := func(t *testing.T) []*Chat {
		t.Helper()
		chats, err := s.ListChats("", ChatSectionAll, now, 0, 100)
		if err != nil {
			t.Fatal(err)
		}
		n, err := s.CountChats("", ChatSectionAll, now)
		if err != nil || n != len(chats) {
			t.Errorf("CountChats = %d, %v; want %d", n, err, len(chats))
		}
		return chats
	}

	before := list(t)
	for _, key := range append(threadKeys, "n9") {
		if chatByKey(before, key) == nil {
			t.Errorf("flag off: chat %s missing from %v", key, chatKeys(before))
		}
	}

	if err := s.SetSenderChat(accountID, lms, true); err != nil {
		t.Fatal(err)
	}
	combined := list(t)
	if len(combined) != len(before)-3 {
		t.Errorf("flag on: %d chats, want %d: %v", len(combined), len(before)-3, chatKeys(combined))
	}
	tests := []struct {
		key     string
		present bool
	}{
		{lmsKey, true},
		{"n1", false}, {"n2", false}, {"n3", false},
		{"t@x", false}, // started by the sender: whole thread joins, human reply included
		{"o@x", true},  // started by alice: stays a thread chat
		{"n9", true},   // other account is unaffected
		{"a@x", true},
	}
	for _, tt := range tests {
		if got := chatByKey(combined, tt.key) != nil; got != tt.present {
			t.Errorf("flag on: chat %s present = %v, want %v", tt.key, got, tt.present)
		}
	}

	c := chatByKey(combined, lmsKey)
	if c == nil {
		t.Fatal("sender chat missing")
	}
	ids := append([]string(nil), c.MessageIDs...)
	sort.Strings(ids)
	if want := []string{"n1", "n2", "n3", "t1", "t2"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("MessageIDs = %v, want %v", ids, want)
	}
	if c.MessageCount != 5 || c.UnreadCount != 3 || c.SenderEmail != lms || c.AccountID != accountID {
		t.Errorf("sender chat = count %d unread %d sender %q account %q; want 5, 3, %q, %q",
			c.MessageCount, c.UnreadCount, c.SenderEmail, c.AccountID, lms, accountID)
	}
	if !c.LatestDate.Equal(now.Add(-16 * time.Hour)) {
		t.Errorf("LatestDate = %v, want t2's %v", c.LatestDate, now.Add(-16*time.Hour))
	}
	if c.Subject != "Re: Forum post" {
		t.Errorf("Subject = %q, want the latest message's %q", c.Subject, "Re: Forum post")
	}
	if o := chatByKey(combined, "o@x"); o != nil && (o.SenderEmail != "" || o.Subject != "Question") {
		t.Errorf("thread chat SenderEmail, Subject = %q, %q; want empty, %q", o.SenderEmail, o.Subject, "Question")
	}

	if err := s.SetSenderChat(accountID, lms, false); err != nil {
		t.Fatal(err)
	}
	if after := list(t); !reflect.DeepEqual(chatKeys(after), chatKeys(before)) {
		t.Errorf("flag off again: chats = %v, want %v", chatKeys(after), chatKeys(before))
	}
}

func TestSenderChatSections(t *testing.T) {
	s, accountID, inboxID, now := senderChatFixture(t)
	if err := s.SetSenderChat(accountID, lms, true); err != nil {
		t.Fatal(err)
	}
	inSection := func(section string) bool {
		t.Helper()
		chats, err := s.ListChats("", section, now, 0, 100)
		if err != nil {
			t.Fatal(err)
		}
		return chatByKey(chats, lmsKey) != nil
	}

	// carol's human reply in t@x lifts the chat out of Low.
	if !inSection(ChatSectionPriority) || inSection(ChatSectionLow) {
		t.Error("with a human reply: want Priority, not Low")
	}
	if _, err := s.db.Exec(`DELETE FROM messages WHERE id = 't2'`); err != nil {
		t.Fatal(err)
	}
	if inSection(ChatSectionPriority) || !inSection(ChatSectionLow) {
		t.Error("all bulk: want Low, not Priority")
	}

	if err := s.SetChatState(accountID, lmsKey, ChatState{SnoozedUntil: ptr(now.Add(time.Hour)), SnoozedAt: ptr(now)}); err != nil {
		t.Fatal(err)
	}
	for _, section := range []string{ChatSectionAll, ChatSectionUnread, ChatSectionPriority, ChatSectionLow, ChatSectionSnoozed} {
		if got, want := inSection(section), section == ChatSectionSnoozed; got != want {
			t.Errorf("snoozed sender chat in %s = %v, want %v", section, got, want)
		}
	}

	// Pin and snooze survive new mail from the sender (the snooze wakes, as for threads).
	if err := s.SetChatState(accountID, lmsKey, ChatState{PinnedAt: ptr(now)}); err != nil {
		t.Fatal(err)
	}
	if err := s.Create(&Message{ID: "n4", AccountID: accountID, FolderID: inboxID, UID: 300, Subject: "New grade",
		FromEmail: lms, Date: now, ReceivedAt: now}); err != nil {
		t.Fatal(err)
	}
	chats, err := s.ListChats("", ChatSectionAll, now, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if c := chatByKey(chats, lmsKey); c == nil || !c.IsPinned || c.MessageCount != 5 {
		t.Errorf("after new mail: sender chat = %+v, want pinned with 5 messages", c)
	}
}

func TestSenderChatLastFromMe(t *testing.T) {
	s, accountID, _, now := senderChatFixture(t)
	if err := s.SetSenderChat(accountID, lms, true); err != nil {
		t.Fatal(err)
	}
	// My reply in t@x is newer than every message in the sender chat.
	if err := s.Create(&Message{ID: "ts", AccountID: accountID, FolderID: "sent-1", UID: 301, ThreadID: "<t@x>",
		Subject: "Re: Forum post", FromEmail: "test@example.com", Date: now.Add(-time.Hour), ReceivedAt: now.Add(-time.Hour), IsRead: true}); err != nil {
		t.Fatal(err)
	}
	chats, err := s.ListChats("", ChatSectionAll, now, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]bool{lmsKey: true, "o@x": false, "n9": false} {
		if c := chatByKey(chats, key); c == nil || c.LastFromMe != want {
			t.Errorf("chat %s LastFromMe = %v, want %v", key, c != nil && c.LastFromMe, want)
		}
	}
}

func TestSenderChatSearch(t *testing.T) {
	s, accountID, inboxID, _ := senderChatFixture(t)
	if err := s.SetSenderChat(accountID, lms, true); err != nil {
		t.Fatal(err)
	}
	results, err := s.SearchChats(inboxID, "grade", 0, 50, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1 merged sender chat", len(results))
	}
	r := results[0]
	ids := append([]string(nil), r.MessageIDs...)
	sort.Strings(ids)
	if r.ThreadKey != lmsKey || r.SenderEmail != lms || r.MessageCount != 2 || r.UnreadCount != 2 ||
		!reflect.DeepEqual(ids, []string{"n1", "n2"}) {
		t.Errorf("result = key %q sender %q count %d unread %d ids %v; want %q, %q, 2, 2, [n1 n2]",
			r.ThreadKey, r.SenderEmail, r.MessageCount, r.UnreadCount, ids, lmsKey, lms)
	}

	// The merged row takes its newest message's subject and every thread's
	// participants.
	results, err = s.SearchChats(inboxID, "post", 0, 50, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1 merged sender chat", len(results))
	}
	r = results[0]
	var emails []string
	for _, p := range r.Participants {
		emails = append(emails, p.Email)
	}
	sort.Strings(emails)
	if r.Subject != "Re: Forum post" || r.HighlightedSubject == "" || r.MessageCount != 4 ||
		!reflect.DeepEqual(emails, []string{"carol@example.com", lms}) {
		t.Errorf("result = subject %q highlighted %q count %d participants %v", r.Subject, r.HighlightedSubject, r.MessageCount, emails)
	}

	results, err = s.SearchChats(inboxID, "question", 0, 50, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].ThreadKey != "o@x" || results[0].SenderEmail != "" {
		t.Errorf("thread search = %+v, want o@x thread chat", results)
	}
}

func TestSenderChatDueSnooze(t *testing.T) {
	s, accountID, inboxID, now := senderChatFixture(t)
	if err := s.SetSenderChat(accountID, lms, true); err != nil {
		t.Fatal(err)
	}
	// t@x maps to the sender chat; o@x, started by alice, keeps its own row.
	snoozed := ChatState{SnoozedUntil: ptr(now.Add(-time.Minute)), SnoozedAt: ptr(now.Add(-17 * time.Hour))}
	for _, key := range []string{lmsKey, "t@x", "o@x"} {
		if err := s.SetChatState(accountID, key, snoozed); err != nil {
			t.Fatal(err)
		}
	}
	due, err := s.ListDueSnoozes(now)
	if err != nil {
		t.Fatal(err)
	}
	want := []DueSnooze{
		{AccountID: accountID, ThreadKey: "o@x", WokeByMail: true, LatestMessageID: "o2", FolderID: inboxID,
			ThreadID: "<o@x>", Subject: "Re: Question", FromEmail: lms},
		{AccountID: accountID, ThreadKey: lmsKey, WokeByMail: true, LatestMessageID: "t2", FolderID: inboxID,
			ThreadID: "<t@x>", Subject: "Re: Forum post", FromEmail: "carol@example.com"},
		{AccountID: accountID, ThreadKey: "t@x", InSenderChat: true},
	}
	if !reflect.DeepEqual(due, want) {
		t.Errorf("due = %+v, want %+v", due, want)
	}
}

// TestSenderChatStaleKeys checks that state keys follow the inbox chat list:
// a combined sender keeps its state with or without inbox mail, a split
// sender or one the default no longer combines drops it, and a pinned or
// snoozed thread that joins a sender chat moves into it.
func TestSenderChatStaleKeys(t *testing.T) {
	s, accountID, inboxID, now := senderChatFixture(t)
	if err := s.SetSenderChat(accountID, lms, true); err != nil {
		t.Fatal(err)
	}
	pinned := ChatState{PinnedAt: ptr(now)}
	for _, key := range []string{lmsKey, SenderChatKey("ghost@x")} {
		if err := s.SetChatState(accountID, key, pinned); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.ClearSenderChat(accountID, "ghost@x"); err != nil {
		t.Fatal(err)
	}
	if n, err := s.CleanupChatState(); err != nil || n != 0 {
		t.Errorf("CleanupChatState = %d, %v; want 0 (sender chat state is not cleaned there)", n, err)
	}

	type move = ChatKeyMove
	steps := []struct {
		name   string
		before func() error
		want   []move
	}{
		{"combined senders keep state", func() error { return nil }, nil},
		{"messages leaving the inbox keep state", func() error {
			_, err := s.db.Exec(`UPDATE messages SET folder_id = 'sent-1' WHERE from_email LIKE ?`, lms)
			return err
		}, nil},
		{"default off drops senders without a choice", func() error {
			_, err := s.db.Exec(`INSERT INTO settings (key, value) VALUES ('chat_combine_senders', 'false')`)
			return err
		}, []move{{accountID, SenderChatKey("ghost@x"), ""}}},
		{"split drops state", func() error { return s.SetSenderChat(accountID, lms, false) },
			[]move{{accountID, SenderChatKey("ghost@x"), ""}, {accountID, lmsKey, ""}}},
	}
	for _, st := range steps {
		t.Run(st.name, func(t *testing.T) {
			if err := st.before(); err != nil {
				t.Fatal(err)
			}
			got, err := s.StaleChatKeys()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, st.want) {
				t.Errorf("StaleChatKeys = %+v, want %+v", got, st.want)
			}
		})
	}
	if err := s.MoveChatState(ChatKeyMove{accountID, lmsKey, ""}); err != nil {
		t.Fatal(err)
	}
	if got, err := s.GetChatState(accountID, lmsKey); err != nil || !got.empty() {
		t.Errorf("dropped state = %+v, %v; want empty", got, err)
	}

	// Back in the inbox, lms's thread t@x is pinned while split, then lms
	// combines: the pin moves into the sender chat.
	if _, err := s.db.Exec(`UPDATE messages SET folder_id = ? WHERE from_email LIKE ? AND account_id = ?`, inboxID, lms, accountID); err != nil {
		t.Fatal(err)
	}
	if err := s.SetChatState(accountID, "t@x", pinned); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSenderChat(accountID, lms, true); err != nil {
		t.Fatal(err)
	}
	got, err := s.StaleChatKeys()
	if err != nil {
		t.Fatal(err)
	}
	want := []move{{accountID, SenderChatKey("ghost@x"), ""}, {accountID, "t@x", lmsKey}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("StaleChatKeys = %+v, want %+v", got, want)
	}
	if err := s.MoveChatState(want[1]); err != nil {
		t.Fatal(err)
	}
	if st, err := s.GetChatState(accountID, lmsKey); err != nil || st.PinnedAt == nil || !st.PinnedAt.Equal(now) {
		t.Errorf("sender chat state = %+v, %v; want pinned at %v", st, err, now)
	}
	if st, err := s.GetChatState(accountID, "t@x"); err != nil || !st.empty() {
		t.Errorf("thread state = %+v, %v; want empty", st, err)
	}
}

func TestSenderChatChoice(t *testing.T) {
	s, accountID, _, _ := senderChatFixture(t)
	if err := s.ClearSenderChat(accountID, lms); err != nil {
		t.Fatal(err)
	}
	if got, err := s.SenderChatChoice(accountID, lms); err != nil || got != nil {
		t.Errorf("SenderChatChoice after clear = %v, %v; want nil", got, err)
	}
	for _, combined := range []bool{true, false} {
		if err := s.SetSenderChat(accountID, lms, combined); err != nil {
			t.Fatal(err)
		}
		if got, err := s.SenderChatChoice(accountID, " NoReply@lms.example "); err != nil || got == nil || *got != combined {
			t.Errorf("SenderChatChoice = %v, %v; want %v", got, err, combined)
		}
	}
}

func TestSenderChatNewMail(t *testing.T) {
	s, accountID, _, _ := senderChatFixture(t)
	tests := []struct {
		name     string
		combined bool
		want     map[string]string // subject -> chat key
	}{
		{"flag off", false, map[string]string{"Grade posted: Math": "n1", "Re: Forum post": "t@x", "Re: Question": "o@x", "a1": "a@x", "Grade posted: Two": "n9"}},
		{"flag on", true, map[string]string{"Grade posted: Math": lmsKey, "Re: Forum post": lmsKey, "Re: Question": "o@x", "a1": "a@x", "Grade posted: Two": "n9"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := s.SetSenderChat(accountID, lms, tt.combined); err != nil {
				t.Fatal(err)
			}
			mail, err := s.ListNewMail([]string{"n1", "t2", "o2", "a1", "n9"})
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]string{}
			for _, m := range mail {
				got[m.Subject] = m.ChatKey
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("chat keys = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSenderChatSearchUnifiedInbox(t *testing.T) {
	tests := []struct {
		name     string
		accounts []string // accounts with lms as a sender chat
		want     []string // account|thread key|message IDs
	}{
		{"no sender chats", nil, []string{"acct-2|n9|n9", "acct-1|n1|n1", "acct-1|n2|n2"}},
		{"one account", []string{"acct-1"}, []string{"acct-2|n9|n9", "acct-1|" + lmsKey + "|n1,n2"}},
		{"both accounts stay apart", []string{"acct-1", "acct-2"}, []string{"acct-2|" + lmsKey + "|n9", "acct-1|" + lmsKey + "|n1,n2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _, _, _ := senderChatFixture(t)
			for _, a := range tt.accounts {
				if err := s.SetSenderChat(a, lms, true); err != nil {
					t.Fatal(err)
				}
			}
			results, err := s.SearchChatsUnifiedInbox("grade", 0, 50, "")
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, r := range results {
				ids := append([]string(nil), r.MessageIDs...)
				sort.Strings(ids)
				got = append(got, r.AccountID+"|"+r.ThreadKey+"|"+strings.Join(ids, ","))
			}
			sort.Strings(got)
			want := append([]string(nil), tt.want...)
			sort.Strings(want)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("results = %v, want %v", got, want)
			}
		})
	}
}

// TestSenderChatDefault checks that senders combine without a sender_chat
// row, except the account's own addresses and mail without a sender, and
// that with the chat_combine_senders setting off only combined rows do.
func TestSenderChatDefault(t *testing.T) {
	s, accountID, inboxID := newBodyFailedTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	if _, err := s.db.Exec(`INSERT INTO identities (id, account_id, email, name) VALUES ('id-1', ?, 'Alias@Example.com', 'Alias')`, accountID); err != nil {
		t.Fatalf("seed identity: %v", err)
	}
	seed := []struct{ id, thread, from string }{
		{"b1", "", "bob@example.com"},
		{"b2", "<b@x>", "Bob@Example.com"},
		{"m1", "", "test@example.com"},
		{"m2", "", "alias@example.com"},
		{"e1", "", ""},
	}
	for i, m := range seed {
		err := s.Create(&Message{
			ID: m.id, AccountID: accountID, FolderID: inboxID, UID: uint32(i + 1),
			ThreadID: m.thread, Subject: m.id, FromEmail: m.from,
			Date: now.Add(-time.Duration(i) * time.Hour), ReceivedAt: now,
		})
		if err != nil {
			t.Fatalf("seed %s: %v", m.id, err)
		}
	}

	chats, err := s.ListChats("", ChatSectionAll, now, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	keys := chatKeys(chats)
	sort.Strings(keys)
	if want := []string{"e1", "m1", "m2", SenderChatKey("bob@example.com")}; !reflect.DeepEqual(keys, want) {
		t.Errorf("chats = %v, want %v", keys, want)
	}

	for email, want := range map[string]bool{"bob@example.com": true, "TEST@example.com": false, "alias@example.com": false, "": false} {
		if got, err := s.IsSenderChat(accountID, email); err != nil || got != want {
			t.Errorf("IsSenderChat(%q) = %v, %v; want %v", email, got, err, want)
		}
	}

	if err := s.SetSenderChat(accountID, "bob@example.com", false); err != nil {
		t.Fatal(err)
	}
	if on, err := s.IsSenderChat(accountID, "bob@example.com"); err != nil || on {
		t.Errorf("after split: IsSenderChat = %v, %v; want false", on, err)
	}
	chats, err = s.ListChats("", ChatSectionAll, now, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(chats); got != 5 {
		t.Errorf("after split: %d chats, want 5: %v", got, chatKeys(chats))
	}

	// With the default off, only senders combined explicitly combine.
	if _, err := s.db.Exec(`INSERT INTO settings (key, value) VALUES ('chat_combine_senders', 'false')`); err != nil {
		t.Fatalf("turn default off: %v", err)
	}
	if _, err := s.db.Exec(`DELETE FROM sender_chat`); err != nil {
		t.Fatal(err)
	}
	if on, err := s.IsSenderChat(accountID, "bob@example.com"); err != nil || on {
		t.Errorf("default off: IsSenderChat = %v, %v; want false", on, err)
	}
	if n, err := s.CountChats("", ChatSectionAll, now); err != nil || n != 5 {
		t.Errorf("default off: CountChats = %d, %v; want 5", n, err)
	}
	if err := s.SetSenderChat(accountID, "bob@example.com", true); err != nil {
		t.Fatal(err)
	}
	if n, err := s.CountChats("", ChatSectionAll, now); err != nil || n != 4 {
		t.Errorf("default off, bob combined: CountChats = %d, %v; want 4", n, err)
	}
}
