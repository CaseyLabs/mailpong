package imap

import (
	"context"
	"errors"
	"slices"
	"sort"
	"testing"
	"time"
)

func newTestIdleManager(t *testing.T) *IdleManager {
	t.Helper()
	cfg := DefaultIdleConfig()
	cfg.ReconnectBackoff = time.Hour // failed dials park in backoff
	cfg.ShutdownTimeout = time.Second
	m := NewIdleManager(cfg, func(string) (*ClientConfig, error) {
		return nil, errors.New("no credentials in test")
	})
	m.Start(context.Background())
	t.Cleanup(m.Stop)
	return m
}

func (m *IdleManager) watched(accountID string) []string {
	folders := m.Watched(accountID)
	sort.Strings(folders)
	return folders
}

func TestIdleManagerStartAccountFolders(t *testing.T) {
	tests := []struct {
		name    string
		folders []string
		want    []string
	}{
		{"defaults to INBOX", nil, []string{"INBOX"}},
		{"inbox and sent", []string{"INBOX", "Sent"}, []string{"INBOX", "Sent"}},
		{"nested sent path", []string{"INBOX", "[Gmail]/Sent Mail"}, []string{"INBOX", "[Gmail]/Sent Mail"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestIdleManager(t)
			m.StartAccount("a1", "Acc", tt.folders...)
			if got := m.watched("a1"); !slices.Equal(got, tt.want) {
				t.Fatalf("watched = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIdleManagerStartAccountIsIdempotentPerFolder(t *testing.T) {
	m := newTestIdleManager(t)
	m.StartAccount("a1", "Acc")
	m.mu.Lock()
	inbox := m.connections[idleKey{"a1", "INBOX"}]
	m.mu.Unlock()

	// Adding Sent later keeps the running INBOX connection.
	m.StartAccount("a1", "Acc", "INBOX", "Sent")
	if got := m.watched("a1"); !slices.Equal(got, []string{"INBOX", "Sent"}) {
		t.Fatalf("watched = %v", got)
	}
	m.mu.Lock()
	same := m.connections[idleKey{"a1", "INBOX"}] == inbox
	m.mu.Unlock()
	if !same {
		t.Fatal("INBOX connection was replaced")
	}
}

func TestIdleManagerStopAccountStopsAllFolders(t *testing.T) {
	m := newTestIdleManager(t)
	m.StartAccount("a1", "One", "INBOX", "Sent")
	m.StartAccount("a2", "Two", "INBOX", "Sent")

	m.StopAccount("a1")
	if got := m.watched("a1"); len(got) != 0 {
		t.Fatalf("a1 still watching %v", got)
	}
	if got := m.watched("a2"); !slices.Equal(got, []string{"INBOX", "Sent"}) {
		t.Fatalf("a2 watched = %v", got)
	}
}

func TestIdleManagerRestartAccount(t *testing.T) {
	m := newTestIdleManager(t)
	m.StartAccount("a1", "Acc")
	m.RestartAccount("a1", "Acc", "INBOX", "Sent")
	if got := m.watched("a1"); !slices.Equal(got, []string{"INBOX", "Sent"}) {
		t.Fatalf("watched = %v", got)
	}
}

func TestNewIdleConnectionFolder(t *testing.T) {
	ic := newIdleConnection("a1", "Acc", "Sent", DefaultIdleConfig(), nil)
	if ic.folder != "Sent" {
		t.Fatalf("folder = %q", ic.folder)
	}
}

func TestIdleManagerStopFolder(t *testing.T) {
	m := newTestIdleManager(t)
	m.StartAccount("a1", "Acc", "INBOX", "Sent")
	m.StopFolder("a1", "Sent")
	if got := m.watched("a1"); !slices.Equal(got, []string{"INBOX"}) {
		t.Fatalf("watched = %v", got)
	}
}

func TestIdleManagerRunning(t *testing.T) {
	m := newTestIdleManager(t)
	m.StartAccount("a1", "Acc", "INBOX", "Sent")
	if !m.Running("a1", "Sent") {
		t.Fatal("Sent not running after StartAccount")
	}
	if m.Running("a1", "Drafts") || m.Running("a2", "Sent") {
		t.Fatal("unwatched folder reported running")
	}
	m.StopAccount("a1")
	if m.Running("a1", "Sent") {
		t.Fatal("Sent still running after StopAccount")
	}
}
