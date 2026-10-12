package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/caseylabs/mailven/internal/account"
	"github.com/caseylabs/mailven/internal/database"
	"github.com/caseylabs/mailven/internal/draft"
	"github.com/caseylabs/mailven/internal/folder"
	"github.com/caseylabs/mailven/internal/message"
	"github.com/caseylabs/mailven/internal/settings"
	"github.com/caseylabs/mailven/internal/smtp"
	"github.com/caseylabs/mailven/internal/undo"
)

func TestChatTextToHTML(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", ""},
		{"  \n ", ""},
		{"hi", "<p>hi</p>"},
		{"a\nb", "<p>a<br>b</p>"},
		{"a\n\nb", "<p>a</p><p>b</p>"},
		{"a\n \n\n b", "<p>a</p><p>b</p>"},
		{"<b>&\"", "<p>&lt;b&gt;&amp;&quot;</p>"},
	}
	for _, tt := range tests {
		if got := chatTextToHTML(tt.in); got != tt.want {
			t.Errorf("chatTextToHTML(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestChatSignatureFor(t *testing.T) {
	on := account.Identity{SignatureEnabled: true, SignatureForReply: true, SignatureHTML: "<p>Al</p>", SignatureText: "Al"}
	tests := []struct {
		name string
		id   *account.Identity
		want chatSignature
	}{
		{"nil identity", nil, chatSignature{}},
		{"disabled", &account.Identity{SignatureForReply: true, SignatureHTML: "<p>Al</p>"}, chatSignature{}},
		{"not for replies", &account.Identity{SignatureEnabled: true, SignatureHTML: "<p>Al</p>"}, chatSignature{}},
		{"empty", &account.Identity{SignatureEnabled: true, SignatureForReply: true}, chatSignature{}},
		{"above", &on, chatSignature{html: "<p>Al</p>", text: "Al"}},
		{"below with separator", func() *account.Identity {
			id := on
			id.SignaturePlacement, id.SignatureSeparator = "below", true
			return &id
		}(), chatSignature{html: "<p>-- </p><p>Al</p>", text: "-- \nAl", below: true}},
		{"text derived from html", &account.Identity{SignatureEnabled: true, SignatureForReply: true, SignatureHTML: "<p>Al</p>"}, chatSignature{html: "<p>Al</p>", text: "Al"}},
		{"html derived from text", &account.Identity{SignatureEnabled: true, SignatureForReply: true, SignatureText: "A<l"}, chatSignature{html: "<p>A&lt;l</p>", text: "A<l"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := chatSignatureFor(tt.id)
			got.text = strings.TrimSpace(got.text)
			if got != tt.want {
				t.Errorf("chatSignatureFor = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestAssembleChatReply(t *testing.T) {
	prepared := func() *smtp.ComposeMessage {
		return &smtp.ComposeMessage{
			HTMLBody: `<p></p><p></p><p>On Mon, Bob wrote:</p><blockquote type="cite"><img src="cid:logo"></blockquote>`,
			TextBody: "\n\nOn Mon, Bob wrote:\n> hi",
			Attachments: []smtp.Attachment{
				{Filename: "logo.png", ContentID: "logo", Inline: true},
			},
		}
	}
	sig := chatSignature{html: "<p>Al</p>", text: "Al"}
	sigBelow := chatSignature{html: "<p>Al</p>", text: "Al", below: true}
	quoteHTML := `<p>On Mon, Bob wrote:</p><blockquote type="cite"><img src="cid:logo"></blockquote>`
	quoteText := "On Mon, Bob wrote:\n> hi"

	tests := []struct {
		name            string
		sig             chatSignature
		quote           bool
		wantHTML        string
		wantText        string
		wantAttachments int
	}{
		{"quote only", chatSignature{}, true, "<p>Thanks</p>" + quoteHTML, "Thanks\n\n" + quoteText, 1},
		{"signature above quote", sig, true, "<p>Thanks</p><p>Al</p>" + quoteHTML, "Thanks\n\nAl\n\n" + quoteText, 1},
		{"signature below quote", sigBelow, true, "<p>Thanks</p>" + quoteHTML + "<p>Al</p>", "Thanks\n\n" + quoteText + "\n\nAl", 1},
		{"no quote drops its inline parts", sigBelow, false, "<p>Thanks</p><p>Al</p>", "Thanks\n\nAl", 0},
		{"no quote, no signature", chatSignature{}, false, "<p>Thanks</p>", "Thanks", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := prepared()
			assembleChatReply(msg, "Thanks", tt.sig, tt.quote)
			if msg.HTMLBody != tt.wantHTML {
				t.Errorf("HTMLBody = %q, want %q", msg.HTMLBody, tt.wantHTML)
			}
			if msg.TextBody != tt.wantText {
				t.Errorf("TextBody = %q, want %q", msg.TextBody, tt.wantText)
			}
			if len(msg.Attachments) != tt.wantAttachments {
				t.Errorf("attachments = %d, want %d", len(msg.Attachments), tt.wantAttachments)
			}
			if got := chatDraftText(msg.TextBody, len("Thanks")); got != "Thanks" {
				t.Errorf("chatDraftText = %q, want the typed text", got)
			}
		})
	}
}

func TestChatDraftText(t *testing.T) {
	tests := []struct {
		body string
		n    int
		want string
	}{
		{"hi\n\nquote", 2, "hi"},
		{"hi", 0, ""},
		{"hi", 5, "hi"},       // body shorter than recorded: return it whole
		{"héllo", 2, "héllo"}, // cut inside a rune: return it whole
		{"hi", -1, "hi"},
	}
	for _, tt := range tests {
		if got := chatDraftText(tt.body, tt.n); got != tt.want {
			t.Errorf("chatDraftText(%q, %d) = %q, want %q", tt.body, tt.n, got, tt.want)
		}
	}
}

func TestNormalizeChatText(t *testing.T) {
	if got := normalizeChatText("a\r\nb  \n\n"); got != "a\nb" {
		t.Errorf("normalizeChatText = %q", got)
	}
}

// newChatReplyTestApp returns an App backed by a migrated database holding
// one account with a signed identity and one inbox message from Bob to the
// account and Carol.
func newChatReplyTestApp(t *testing.T) *App {
	t.Helper()
	a, _ := newChatReplyTestAppDB(t)
	return a
}

// newChatReplyTestAppDB is newChatReplyTestApp that also returns the
// database, for tests that break it on purpose.
func newChatReplyTestAppDB(t *testing.T) (*App, *database.DB) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`INSERT INTO accounts (id, name, email, imap_host, smtp_host, username)
			VALUES ('acct-1', 'Me', 'me@example.com', 'imap.example.com', 'smtp.example.com', 'me@example.com')`,
		`INSERT INTO folders (id, account_id, name, path, folder_type) VALUES ('f-1', 'acct-1', 'INBOX', 'INBOX', 'inbox')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	a := &App{
		accountStore:    account.NewStore(db),
		messageStore:    message.NewStore(db),
		attachmentStore: message.NewAttachmentStore(db),
		draftStore:      draft.NewStore(db),
		settingsStore:   settings.NewStore(db),
	}
	if _, err := a.accountStore.CreateIdentity("acct-1", &account.IdentityConfig{
		Email: "me@example.com", Name: "Me", SignatureEnabled: true, SignatureForReply: true,
		SignatureHTML: "<p>Me</p>", SignatureText: "Me",
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.messageStore.Create(&message.Message{
		ID: "m-1", AccountID: "acct-1", FolderID: "f-1", UID: 1, MessageID: "<orig@example.com>",
		Subject: "Lunch", FromName: "Bob", FromEmail: "bob@example.com",
		ToList: "me@example.com, Carol <carol@example.com>", Date: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
		BodyText: "Lunch at noon?", BodyFetched: true,
	}); err != nil {
		t.Fatal(err)
	}
	return a, db
}

func TestBuildChatReply(t *testing.T) {
	a := newChatReplyTestApp(t)
	addrs := func(list []smtp.Address) string {
		var out []string
		for _, ad := range list {
			out = append(out, ad.Address)
		}
		return strings.Join(out, ",")
	}

	tests := []struct {
		name     string
		replyAll bool
		quote    bool
		wantTo   string
		wantText string
	}{
		{"reply with quote", false, true, "bob@example.com", "Sure\n\nMe\n\nOn "},
		{"reply-all without quote", true, false, "bob@example.com,carol@example.com", "Sure\n\nMe"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := a.settingsStore.SetChatIncludeQuote(tt.quote); err != nil {
				t.Fatal(err)
			}
			msg, err := a.buildChatReply(ChatReply{
				AccountID: "acct-1", MessageID: "m-1", Text: "Sure\r\n", ReplyAll: tt.replyAll,
				Attachments: []smtp.Attachment{{Filename: "a.txt", ContentBase64: "aGk="}},
			})
			if err != nil {
				t.Fatal(err)
			}
			if got := addrs(msg.To); got != tt.wantTo {
				t.Errorf("To = %q, want %q", got, tt.wantTo)
			}
			if msg.Subject != "Re: Lunch" {
				t.Errorf("Subject = %q", msg.Subject)
			}
			if msg.InReplyTo != "<orig@example.com>" {
				t.Errorf("InReplyTo = %q", msg.InReplyTo)
			}
			if got := strings.Join(msg.References, " "); !strings.HasSuffix(got, "<orig@example.com>") {
				t.Errorf("References = %q", got)
			}
			if !strings.HasPrefix(msg.TextBody, tt.wantText) {
				t.Errorf("TextBody = %q, want prefix %q", msg.TextBody, tt.wantText)
			}
			if tt.quote != strings.Contains(msg.TextBody, "> Lunch at noon?") {
				t.Errorf("quote in TextBody = %v, want %v: %q", !tt.quote, tt.quote, msg.TextBody)
			}
			if len(msg.Attachments) != 1 || msg.Attachments[0].Filename != "a.txt" {
				t.Errorf("Attachments = %+v, want the user's file", msg.Attachments)
			}
		})
	}

	if _, err := a.buildChatReply(ChatReply{AccountID: "acct-1"}); err == nil {
		t.Error("buildChatReply without a message: want error")
	}
}

// A sender chat reply targets the chosen message, not the chat's latest one:
// its threading headers come from that message's thread.
func TestBuildChatReplySenderChat(t *testing.T) {
	a := newChatReplyTestApp(t)
	if err := a.messageStore.Create(&message.Message{
		ID: "m-2", AccountID: "acct-1", FolderID: "f-1", UID: 2, MessageID: "<later@example.com>",
		Subject: "Dinner", FromName: "Bob", FromEmail: "bob@example.com", ToList: "me@example.com",
		Date: time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC), BodyText: "Dinner?", BodyFetched: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.messageStore.SetSenderChat("acct-1", "bob@example.com", true); err != nil {
		t.Fatal(err)
	}
	key := message.SenderChatKey("bob@example.com")

	tests := []struct{ messageID, wantSubject, wantInReplyTo string }{
		{"m-1", "Re: Lunch", "<orig@example.com>"},
		{"m-2", "Re: Dinner", "<later@example.com>"},
	}
	for _, tt := range tests {
		t.Run(tt.messageID, func(t *testing.T) {
			msg, err := a.buildChatReply(ChatReply{AccountID: "acct-1", ThreadKey: key, MessageID: tt.messageID, Text: "Yes"})
			if err != nil {
				t.Fatal(err)
			}
			if msg.Subject != tt.wantSubject || msg.InReplyTo != tt.wantInReplyTo {
				t.Errorf("Subject, InReplyTo = %q, %q; want %q, %q", msg.Subject, msg.InReplyTo, tt.wantSubject, tt.wantInReplyTo)
			}
			if got := strings.Join(msg.References, " "); got != tt.wantInReplyTo {
				t.Errorf("References = %q, want %q", got, tt.wantInReplyTo)
			}
		})
	}
}

func TestChatReplyEmpty(t *testing.T) {
	tests := []struct {
		name string
		r    ChatReply
		want bool
	}{
		{"no text", ChatReply{}, true},
		{"whitespace only", ChatReply{Text: " \r\n\t "}, true},
		{"text", ChatReply{Text: "hi"}, false},
		{"attachment only", ChatReply{Attachments: []smtp.Attachment{{Filename: "a.txt"}}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := chatReplyEmpty(tt.r); got != tt.want {
				t.Errorf("chatReplyEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
	// sendChatReply rejects an empty reply before touching any store.
	if err := (&App{}).sendChatReply(ChatReply{AccountID: "a", MessageID: "m", Text: "  "}); err == nil {
		t.Error("sendChatReply with empty text: want error")
	}
}

// SetSenderChat refuses an unknown account before writing anything.
func TestSetSenderChatUnknownAccount(t *testing.T) {
	a := newChatReplyTestApp(t)
	if _, err := a.SetSenderChat("nope", "bob@example.com", false); err == nil {
		t.Fatal("SetSenderChat for unknown account: want error")
	}
	// Senders combine by default, so a write would have split bob.
	if on, err := a.messageStore.IsSenderChat("nope", "bob@example.com"); err != nil || !on {
		t.Errorf("IsSenderChat = %v, %v; want true", on, err)
	}
}

// SetSenderChat on a sender that already combines (the default) reports no
// change and records no undo, so Undo can't reach an unrelated action.
func TestSetSenderChatUnchanged(t *testing.T) {
	a := newChatReplyTestApp(t)
	a.undoStack = undo.NewStack(10, time.Minute)
	changed, err := a.SetSenderChat("acct-1", "Bob@Example.com", true)
	if err != nil || changed {
		t.Fatalf("SetSenderChat = %v, %v; want false, nil", changed, err)
	}
	if n := a.undoStack.Size(); n != 0 {
		t.Errorf("undo stack size = %d, want 0", n)
	}
	if choice, err := a.messageStore.SenderChatChoice("acct-1", "bob@example.com"); err != nil || choice != nil {
		t.Errorf("SenderChatChoice = %v, %v; want nil", choice, err)
	}
}

// splitFixture adds to the chat reply app a second thread from bob (newer
// than m-1), a thread from carol, and bob's sender chat with a pin and a chat
// draft link replying to replyID. It returns the pin time and the link.
func splitFixture(t *testing.T, a *App, email, replyID string) (time.Time, draft.ChatLink) {
	t.Helper()
	for _, m := range []*message.Message{
		{ID: "m-2", UID: 2, Subject: "Dinner", FromEmail: "bob@example.com", Date: time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)},
		{ID: "m-3", UID: 3, Subject: "Hello", FromEmail: "carol@example.com", Date: time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)},
	} {
		m.AccountID, m.FolderID = "acct-1", "f-1"
		if err := a.messageStore.Create(m); err != nil {
			t.Fatal(err)
		}
	}
	pinned := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	key := message.SenderChatKey(email)
	if err := a.messageStore.SetChatState("acct-1", key, message.ChatState{PinnedAt: &pinned}); err != nil {
		t.Fatal(err)
	}
	d := &draft.Draft{AccountID: "acct-1", Subject: "Hi"}
	if err := a.draftStore.Create(d); err != nil {
		t.Fatal(err)
	}
	link := draft.ChatLink{DraftID: d.ID, MessageID: replyID, TextLen: 3}
	if err := a.draftStore.SetChatLink("acct-1", key, link); err != nil {
		t.Fatal(err)
	}
	return pinned, link
}

// checkChatKey fails unless key holds exactly the given pin and link.
func checkChatKey(t *testing.T, a *App, key string, pinned *time.Time, link *draft.ChatLink) {
	t.Helper()
	st, err := a.messageStore.GetChatState("acct-1", key)
	if err != nil {
		t.Fatal(err)
	}
	if (st.PinnedAt == nil) != (pinned == nil) || (pinned != nil && !st.PinnedAt.Equal(*pinned)) {
		t.Errorf("%s pin = %v, want %v", key, st.PinnedAt, pinned)
	}
	got, err := a.draftStore.GetChatLink("acct-1", key)
	if err != nil {
		t.Fatal(err)
	}
	if (got == nil) != (link == nil) || (link != nil && *got != *link) {
		t.Errorf("%s link = %+v, want %+v", key, got, link)
	}
}

// Splitting a sender chat gives its pin to the newest inbox thread the sender
// started and its chat draft link to the thread of the reply target, falling
// back to the newest thread; a sender with no thread loses both.
func TestSplitSenderChat(t *testing.T) {
	tests := []struct {
		name, email, replyID string
		pinKey, linkKey      string // "" = dropped
	}{
		{"reply target in newest thread", "bob@example.com", "m-2", "m-2", "m-2"},
		{"reply target in older thread", "bob@example.com", "m-1", "m-2", "m-1"},
		{"reply target deleted", "bob@example.com", "m-gone", "m-2", "m-2"},
		{"reply target in another sender's thread", "bob@example.com", "m-3", "m-2", "m-2"},
		{"no inbox mail", "ghost@example.com", "m-1", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newChatReplyTestApp(t)
			pinned, link := splitFixture(t, a, tt.email, tt.replyID)
			if _, err := a.splitSenderChat("acct-1", tt.email); err != nil {
				t.Fatal(err)
			}
			checkChatKey(t, a, message.SenderChatKey(tt.email), nil, nil)
			for _, key := range []string{"m-1", "m-2"} {
				var p *time.Time
				var l *draft.ChatLink
				if key == tt.pinKey {
					p = &pinned
				}
				if key == tt.linkKey {
					l = &link
				}
				checkChatKey(t, a, key, p, l)
			}
		})
	}
}

// Undoing a split returns the sender chat and both target threads to their
// state and link from before, including state already on a target thread.
func TestSplitSenderChatUndo(t *testing.T) {
	a := newChatReplyTestApp(t)
	a.undoStack = undo.NewStack(10, time.Minute)
	pinned, link := splitFixture(t, a, "bob@example.com", "m-1")
	snoozed := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	if err := a.messageStore.SetChatState("acct-1", "m-1", message.ChatState{SnoozedUntil: &snoozed, SnoozedAt: &pinned}); err != nil {
		t.Fatal(err)
	}
	other := draft.Draft{AccountID: "acct-1", Subject: "Other"}
	if err := a.draftStore.Create(&other); err != nil {
		t.Fatal(err)
	}
	otherLink := draft.ChatLink{DraftID: other.ID, MessageID: "m-2"}
	if err := a.draftStore.SetChatLink("acct-1", "m-2", otherLink); err != nil {
		t.Fatal(err)
	}
	// Combined by default; the sender chat holds the pin and link.
	splitBob(t, a)
	checkChatKey(t, a, "m-1", nil, &link) // the reply target's thread
	checkChatKey(t, a, "m-2", &pinned, &otherLink)
	undoLast(t, a)
	if on, err := a.messageStore.IsSenderChat("acct-1", "bob@example.com"); err != nil || !on {
		t.Errorf("IsSenderChat after undo = %v, %v; want true", on, err)
	}
	checkChatKey(t, a, message.SenderChatKey("bob@example.com"), &pinned, &link)
	checkChatKey(t, a, "m-2", nil, &otherLink)
	st, err := a.messageStore.GetChatState("acct-1", "m-1")
	if err != nil || st.SnoozedUntil == nil || !st.SnoozedUntil.Equal(snoozed) || st.PinnedAt != nil {
		t.Errorf("m-1 state after undo = %+v, %v; want only its snooze", st, err)
	}
	checkChatKey(t, a, "m-1", nil, nil)
}

// splitBob splits bob's sender chat, which is combined by default.
func splitBob(t *testing.T, a *App) {
	t.Helper()
	if changed, err := a.SetSenderChat("acct-1", "bob@example.com", false); err != nil || !changed {
		t.Fatalf("SetSenderChat split = %v, %v", changed, err)
	}
}

// undoLast undoes the newest undo stack entry.
func undoLast(t *testing.T, a *App) {
	t.Helper()
	cmd := a.undoStack.Pop()
	if cmd == nil {
		t.Fatal("no undo entry")
	}
	if err := cmd.Undo(); err != nil {
		t.Fatalf("Undo: %v", err)
	}
}

// Undoing a split takes back only what the split moved: changes made to the
// target threads or the moved draft since then survive the undo.
func TestSplitSenderChatUndoAfterChanges(t *testing.T) {
	later := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		after func(t *testing.T, a *App, pinned time.Time, link draft.ChatLink)
		check func(t *testing.T, a *App, pinned time.Time, link draft.ChatLink)
	}{
		{
			"draft sent",
			func(t *testing.T, a *App, _ time.Time, link draft.ChatLink) {
				if err := a.draftStore.Delete(link.DraftID); err != nil {
					t.Fatal(err)
				}
			},
			func(t *testing.T, a *App, pinned time.Time, _ draft.ChatLink) {
				checkChatKey(t, a, message.SenderChatKey("bob@example.com"), &pinned, nil)
				checkChatKey(t, a, "m-1", nil, nil)
			},
		},
		{
			"typed more",
			func(t *testing.T, a *App, _ time.Time, link draft.ChatLink) {
				link.TextLen = 40
				if err := a.draftStore.SetChatLink("acct-1", "m-1", link); err != nil {
					t.Fatal(err)
				}
			},
			func(t *testing.T, a *App, pinned time.Time, link draft.ChatLink) {
				link.TextLen = 40
				checkChatKey(t, a, message.SenderChatKey("bob@example.com"), &pinned, &link)
				checkChatKey(t, a, "m-1", nil, nil)
			},
		},
		{
			"new draft on newest thread",
			func(t *testing.T, a *App, _ time.Time, _ draft.ChatLink) {
				d := &draft.Draft{AccountID: "acct-1", Subject: "New"}
				if err := a.draftStore.Create(d); err != nil {
					t.Fatal(err)
				}
				if err := a.draftStore.SetChatLink("acct-1", "m-2", draft.ChatLink{DraftID: d.ID, MessageID: "m-2"}); err != nil {
					t.Fatal(err)
				}
			},
			func(t *testing.T, a *App, pinned time.Time, link draft.ChatLink) {
				checkChatKey(t, a, message.SenderChatKey("bob@example.com"), &pinned, &link)
				if got, err := a.draftStore.GetChatLink("acct-1", "m-2"); err != nil || got == nil {
					t.Errorf("m-2 link = %+v, %v; want the new draft's", got, err)
				}
			},
		},
		{
			"snoozed newest thread",
			func(t *testing.T, a *App, pinned time.Time, _ draft.ChatLink) {
				st := message.ChatState{PinnedAt: &pinned, SnoozedUntil: &later, SnoozedAt: &pinned}
				if err := a.messageStore.SetChatState("acct-1", "m-2", st); err != nil {
					t.Fatal(err)
				}
			},
			func(t *testing.T, a *App, pinned time.Time, link draft.ChatLink) {
				checkChatKey(t, a, message.SenderChatKey("bob@example.com"), &pinned, &link)
				st, err := a.messageStore.GetChatState("acct-1", "m-2")
				if err != nil || st.PinnedAt != nil || st.SnoozedUntil == nil || !st.SnoozedUntil.Equal(later) {
					t.Errorf("m-2 state = %+v, %v; want only the later snooze", st, err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newChatReplyTestApp(t)
			a.undoStack = undo.NewStack(10, time.Minute)
			pinned, link := splitFixture(t, a, "bob@example.com", "m-1")
			splitBob(t, a)
			tt.after(t, a, pinned, link)
			undoLast(t, a)
			tt.check(t, a, pinned, link)
		})
	}
}

// When the link move fails after the pin moved, the choice, pin and link all
// match their state before the split.
func TestSplitSenderChatRollback(t *testing.T) {
	a, db := newChatReplyTestAppDB(t)
	a.undoStack = undo.NewStack(10, time.Minute)
	pinned, link := splitFixture(t, a, "bob@example.com", "m-1")
	if _, err := db.Exec(`CREATE TRIGGER fail_link_move BEFORE UPDATE ON chat_drafts
		BEGIN SELECT RAISE(FAIL, 'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if changed, err := a.SetSenderChat("acct-1", "bob@example.com", false); err == nil || changed {
		t.Fatalf("SetSenderChat = %v, %v; want error", changed, err)
	}
	if choice, err := a.messageStore.SenderChatChoice("acct-1", "bob@example.com"); err != nil || choice != nil {
		t.Errorf("choice = %v, %v; want default", choice, err)
	}
	if on, err := a.messageStore.IsSenderChat("acct-1", "bob@example.com"); err != nil || !on {
		t.Errorf("IsSenderChat = %v, %v; want true", on, err)
	}
	checkChatKey(t, a, message.SenderChatKey("bob@example.com"), &pinned, &link)
	checkChatKey(t, a, "m-1", nil, nil)
	checkChatKey(t, a, "m-2", nil, nil)
	if n := a.undoStack.Size(); n != 0 {
		t.Errorf("undo stack size = %d, want 0", n)
	}
}

func TestApplyIdentityReplyTo(t *testing.T) {
	existing := &smtp.Address{Address: "kept@example.com"}
	tests := []struct {
		name string
		msg  *smtp.ComposeMessage
		id   *account.Identity
		def  string
		want string
	}{
		{"identity reply-to applied", &smtp.ComposeMessage{}, &account.Identity{ReplyTo: "r@example.com"}, "d@example.com", "r@example.com"},
		{"existing kept", &smtp.ComposeMessage{ReplyTo: existing}, &account.Identity{ReplyTo: "r@example.com"}, "", "kept@example.com"},
		{"account default used", &smtp.ComposeMessage{}, &account.Identity{}, " d@example.com; e@example.com", "d@example.com"},
		{"identity without reply-to", &smtp.ComposeMessage{}, &account.Identity{}, "", ""},
		{"no identity", &smtp.ComposeMessage{}, nil, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applyIdentityReplyTo(tt.msg, tt.id, tt.def)
			got := ""
			if tt.msg.ReplyTo != nil {
				got = tt.msg.ReplyTo.Address
			}
			if got != tt.want {
				t.Errorf("ReplyTo = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestChatReplyFrom(t *testing.T) {
	a, db := newChatReplyTestAppDB(t)
	a.ctx, a.folderStore = context.Background(), folder.NewStore(db)
	if _, err := db.Exec(`UPDATE identities SET is_default = 1 WHERE email = 'me@example.com'`); err != nil {
		t.Fatal(err)
	}
	if _, err := a.accountStore.CreateIdentity("acct-1", &account.IdentityConfig{Email: "alias@example.com", Name: "Me"}); err != nil {
		t.Fatal(err)
	}
	uid := uint32(1)
	add := func(id, to, bcc, deliveredTo string) {
		t.Helper()
		uid++
		m := &message.Message{ID: id, AccountID: "acct-1", FolderID: "f-1", UID: uid, Subject: "Hi",
			FromEmail: "bob@example.com", ToList: to, BccList: bcc, Date: time.Now()}
		if err := a.messageStore.Create(m); err != nil {
			t.Fatal(err)
		}
		if deliveredTo != "" {
			if err := a.messageStore.SetDeliveredTo(id, deliveredTo); err != nil {
				t.Fatal(err)
			}
		}
	}
	add("m-bcc", "list@example.com", "alias@example.com", "")
	add("m-fwd", "list@example.com", "", `["alias@example.com"]`)
	add("m-none", "list@example.com", "", `[]`)

	tests := []struct{ id, want string }{
		{"m-1", ""}, // addressed to the default identity
		{"m-bcc", "alias@example.com"},
		{"m-fwd", "alias@example.com"},
		{"m-none", ""},
		{"missing", ""},
	}
	for _, tt := range tests {
		got, err := a.ChatReplyFrom(tt.id)
		if err != nil {
			t.Fatalf("%s: %v", tt.id, err)
		}
		if got != tt.want {
			t.Errorf("ChatReplyFrom(%s) = %q, want %q", tt.id, got, tt.want)
		}
	}
}

func TestStaleSenderChatDraftTargets(t *testing.T) {
	for _, trigger := range []string{"default change", "startup reconcile"} {
		for _, target := range []string{"m-1", "m-2", "m-gone", "m-3"} {
			t.Run(trigger+"/"+target, func(t *testing.T) {
				a := newChatReplyTestApp(t)
				pinned, link := splitFixture(t, a, "bob@example.com", target)
				if trigger == "default change" {
					if err := a.settingsStore.SetChatCombineSenders(false); err != nil {
						t.Fatal(err)
					}
				} else if err := a.messageStore.SetSenderChat("acct-1", "bob@example.com", false); err != nil {
					t.Fatal(err)
				}
				if _, err := a.moveStaleChatState(); err != nil {
					t.Fatal(err)
				}
				checkChatKey(t, a, message.SenderChatKey("bob@example.com"), nil, nil)
				if target == "m-1" {
					checkChatKey(t, a, "m-1", nil, &link)
					checkChatKey(t, a, "m-2", &pinned, nil)
				} else {
					checkChatKey(t, a, "m-1", nil, nil)
					checkChatKey(t, a, "m-2", &pinned, &link)
				}
			})
		}
	}
}
