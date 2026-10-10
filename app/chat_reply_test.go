package app

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hkdb/aerion/internal/account"
	"github.com/hkdb/aerion/internal/database"
	"github.com/hkdb/aerion/internal/draft"
	"github.com/hkdb/aerion/internal/message"
	"github.com/hkdb/aerion/internal/settings"
	"github.com/hkdb/aerion/internal/smtp"
	"github.com/hkdb/aerion/internal/undo"
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
	return a
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
	// SendChatReply rejects an empty reply before touching any store.
	if err := (&App{}).SendChatReply(ChatReply{AccountID: "a", MessageID: "m", Text: "  "}); err == nil {
		t.Error("SendChatReply with empty text: want error")
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
