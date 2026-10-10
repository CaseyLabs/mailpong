package draft

import (
	"path/filepath"
	"testing"

	"github.com/hkdb/aerion/internal/database"
)

func TestChatLink(t *testing.T) {
	db := openTestDB(t)
	accountID := insertTestAccount(t, db)
	store := NewStore(db)

	d := &Draft{AccountID: accountID, Subject: "Re: Hi", BodyText: "hello\n\nquote"}
	if err := store.Create(d); err != nil {
		t.Fatal(err)
	}

	if got, err := store.GetChatLink(accountID, "t1@x"); err != nil || got != nil {
		t.Fatalf("GetChatLink before set = %v, %v; want nil, nil", got, err)
	}

	link := ChatLink{DraftID: d.ID, MessageID: "m-1", ReplyAll: true, TextLen: 5}
	if err := store.SetChatLink(accountID, "<t1@x>", link); err != nil {
		t.Fatalf("SetChatLink: %v", err)
	}
	got, err := store.GetChatLink(accountID, "t1@x")
	if err != nil || got == nil || *got != link {
		t.Fatalf("GetChatLink = %+v, %v; want %+v", got, err, link)
	}

	link.ReplyAll, link.TextLen = false, 3
	if err := store.SetChatLink(accountID, "t1@x", link); err != nil {
		t.Fatalf("SetChatLink update: %v", err)
	}
	if got, _ := store.GetChatLink(accountID, "t1@x"); got == nil || *got != link {
		t.Fatalf("after update GetChatLink = %+v, want %+v", got, link)
	}

	// Deleting the draft drops the link.
	if err := store.Delete(d.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.GetChatLink(accountID, "t1@x"); got != nil {
		t.Errorf("link survived draft delete: %+v", got)
	}

	if err := store.SetChatLink(accountID, "", link); err == nil {
		t.Error("SetChatLink with empty key: want error")
	}
}

// A sender chat's draft link is keyed by its "sender:" chat key and survives
// closing and reopening the database.
func TestChatLinkSenderKeyPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	accountID := insertTestAccount(t, db)
	d := &Draft{AccountID: accountID, Subject: "Re: Grade posted", BodyText: "thanks"}
	if err := NewStore(db).Create(d); err != nil {
		t.Fatal(err)
	}
	const key = "sender:noreply@lms.example"
	link := ChatLink{DraftID: d.ID, MessageID: "n2", TextLen: 6}
	if err := NewStore(db).SetChatLink(accountID, key, link); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if got, err := NewStore(db).GetChatLink(accountID, key); err != nil || got == nil || *got != link {
		t.Fatalf("GetChatLink after reopen = %+v, %v; want %+v", got, err, link)
	}
}

// MoveChatLink re-keys a link unless the target has one, which wins; an
// empty target drops the link. Drafts stay either way.
func TestMoveChatLink(t *testing.T) {
	db := openTestDB(t)
	accountID := insertTestAccount(t, db)
	store := NewStore(db)
	links := map[string]ChatLink{}
	for _, key := range []string{"a@x", "b@x", "c@x"} {
		d := &Draft{AccountID: accountID, Subject: key}
		if err := store.Create(d); err != nil {
			t.Fatal(err)
		}
		links[key] = ChatLink{DraftID: d.ID, MessageID: key}
		if err := store.SetChatLink(accountID, key, links[key]); err != nil {
			t.Fatal(err)
		}
	}
	const sender = "sender:bob@example.com"
	steps := []struct {
		from, to string
		want     ChatLink // link on sender afterwards
	}{
		{"a@x", sender, links["a@x"]},
		{"b@x", sender, links["a@x"]},
		{"c@x", "", links["a@x"]},
	}
	for _, st := range steps {
		if err := store.MoveChatLink(accountID, st.from, st.to); err != nil {
			t.Fatalf("MoveChatLink(%q, %q): %v", st.from, st.to, err)
		}
		if got, _ := store.GetChatLink(accountID, st.from); got != nil {
			t.Errorf("MoveChatLink(%q, %q): source link kept: %+v", st.from, st.to, got)
		}
		if got, err := store.GetChatLink(accountID, sender); err != nil || got == nil || *got != st.want {
			t.Errorf("MoveChatLink(%q, %q): sender link = %+v, %v; want %+v", st.from, st.to, got, err, st.want)
		}
	}
	for key, link := range links {
		if d, err := store.Get(link.DraftID); err != nil || d == nil {
			t.Errorf("draft for %s = %v, %v; want kept", key, d, err)
		}
	}
}
