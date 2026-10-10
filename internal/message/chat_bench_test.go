package message

import (
	"fmt"
	"testing"
	"time"
)

const benchMessages = 50000

// benchChatStore fills the inbox with benchMessages messages: threads of one
// to five messages and a quarter of them unthreaded, from 2000 senders, with
// combining left on (the default).
func benchChatStore(b *testing.B) (*Store, []string, time.Time) {
	b.Helper()
	s, accountID, inboxID := newBodyFailedTestStore(b)
	now := time.Now().UTC().Truncate(time.Second)

	tx, err := s.db.Begin()
	if err != nil {
		b.Fatalf("begin: %v", err)
	}
	stmt, err := tx.Prepare(`
		INSERT INTO messages (id, account_id, folder_id, uid, thread_id, subject, from_email, date, received_at, is_read, is_bulk, snippet)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'snippet')`)
	if err != nil {
		b.Fatalf("prepare: %v", err)
	}
	var newest []string
	thread := 0
	for i := 0; i < benchMessages; i++ {
		var threadID any
		if i%4 != 0 {
			threadID = fmt.Sprintf("<t%d@x>", thread)
			if i%3 == 0 {
				thread++
			}
		}
		at := now.Add(-time.Duration(benchMessages-i) * time.Minute)
		id := fmt.Sprintf("m%d", i)
		sender := fmt.Sprintf("sender%d@example.com", (i*7919)%2000)
		if _, err := stmt.Exec(id, accountID, inboxID, i+1000, threadID, id, sender, at, at, i%5 == 0, i%3 == 0); err != nil {
			b.Fatalf("insert: %v", err)
		}
		if i >= benchMessages-50 {
			newest = append(newest, id)
		}
	}
	if err := stmt.Close(); err != nil {
		b.Fatalf("close stmt: %v", err)
	}
	if err := tx.Commit(); err != nil {
		b.Fatalf("commit: %v", err)
	}
	return s, newest, now
}

func BenchmarkChatQueries(b *testing.B) {
	b.Run("combine", func(b *testing.B) { benchChatQueries(b, true) })
	b.Run("split", func(b *testing.B) { benchChatQueries(b, false) })
}

func benchChatQueries(b *testing.B, combine bool) {
	s, newest, now := benchChatStore(b)
	if !combine {
		if _, err := s.db.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('chat_combine_senders', 'false')`); err != nil {
			b.Fatal(err)
		}
	}

	b.Run("ListChats", func(b *testing.B) {
		for b.Loop() {
			if _, err := s.ListChats("", ChatSectionAll, now, 0, 50); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("ListChatsUnread", func(b *testing.B) {
		for b.Loop() {
			if _, err := s.ListChats("", ChatSectionUnread, now, 0, 50); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("CountChats", func(b *testing.B) {
		for b.Loop() {
			if _, err := s.CountChats("", ChatSectionAll, now); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("ListNewMail", func(b *testing.B) {
		for b.Loop() {
			if _, err := s.ListNewMail(newest); err != nil {
				b.Fatal(err)
			}
		}
	})
}
