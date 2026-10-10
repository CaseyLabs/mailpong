package undo

import (
	"reflect"
	"testing"
	"time"

	"github.com/hkdb/aerion/internal/message"
)

type mockCommand struct {
	BaseCommand
	executeFn func() error
	undoFn    func() error
}

func (m *mockCommand) Execute() error {
	if m.executeFn != nil {
		return m.executeFn()
	}
	return nil
}

func (m *mockCommand) Undo() error {
	if m.undoFn != nil {
		return m.undoFn()
	}
	return nil
}

func newMock(desc string) *mockCommand {
	return &mockCommand{BaseCommand: NewBaseCommand(desc)}
}

func TestNewStack(t *testing.T) {
	s := NewStack(50, 30*time.Second)
	if s == nil {
		t.Fatal("expected non-nil stack")
	}
	if s.Size() != 0 {
		t.Fatalf("expected size 0, got %d", s.Size())
	}
	if s.CanUndo() {
		t.Fatal("expected CanUndo=false for empty stack")
	}
}

func TestPushAndPop(t *testing.T) {
	s := NewStack(50, 30*time.Second)
	cmds := []*mockCommand{newMock("first"), newMock("second"), newMock("third")}
	for _, c := range cmds {
		s.Push(c)
	}

	// Pop should return in LIFO order
	for i := len(cmds) - 1; i >= 0; i-- {
		got := s.Pop()
		if got == nil {
			t.Fatalf("expected command at index %d, got nil", i)
		}
		if got.Description() != cmds[i].Description() {
			t.Fatalf("expected %q, got %q", cmds[i].Description(), got.Description())
		}
	}
}

func TestPeek(t *testing.T) {
	s := NewStack(50, 30*time.Second)
	cmd := newMock("peek-me")
	s.Push(cmd)

	peeked := s.Peek()
	if peeked == nil {
		t.Fatal("expected non-nil from Peek")
	}
	if peeked.Description() != "peek-me" {
		t.Fatalf("expected %q, got %q", "peek-me", peeked.Description())
	}
	if s.Size() != 1 {
		t.Fatalf("expected size 1 after Peek, got %d", s.Size())
	}
}

func TestClear(t *testing.T) {
	s := NewStack(50, 30*time.Second)
	s.Push(newMock("a"))
	s.Push(newMock("b"))
	s.Clear()
	if s.Size() != 0 {
		t.Fatalf("expected size 0 after Clear, got %d", s.Size())
	}
}

func TestCanUndo(t *testing.T) {
	s := NewStack(50, 30*time.Second)
	if s.CanUndo() {
		t.Fatal("expected CanUndo=false for empty stack")
	}

	s.Push(newMock("cmd"))
	if !s.CanUndo() {
		t.Fatal("expected CanUndo=true after push")
	}

	s.Pop()
	if s.CanUndo() {
		t.Fatal("expected CanUndo=false after popping all")
	}
}

func TestMaxSize(t *testing.T) {
	s := NewStack(2, 30*time.Second)
	s.Push(newMock("first"))
	s.Push(newMock("second"))
	s.Push(newMock("third"))

	if s.Size() != 2 {
		t.Fatalf("expected size 2, got %d", s.Size())
	}

	// Oldest ("first") should have been dropped
	got := s.Pop()
	if got.Description() != "third" {
		t.Fatalf("expected %q, got %q", "third", got.Description())
	}
	got = s.Pop()
	if got.Description() != "second" {
		t.Fatalf("expected %q, got %q", "second", got.Description())
	}
}

func TestExpiration(t *testing.T) {
	s := NewStack(50, 50*time.Millisecond)
	s.Push(newMock("expires"))

	time.Sleep(100 * time.Millisecond)

	if s.CanUndo() {
		t.Fatal("expected CanUndo=false after expiration")
	}
}

func TestPopEmptyStack(t *testing.T) {
	s := NewStack(50, 30*time.Second)
	got := s.Pop()
	if got != nil {
		t.Fatalf("expected nil from empty stack, got %v", got)
	}
}

func TestBaseCommand(t *testing.T) {
	before := time.Now()
	bc := NewBaseCommand("test description")
	after := time.Now()

	if bc.Description() != "test description" {
		t.Fatalf("expected %q, got %q", "test description", bc.Description())
	}

	created := bc.CreatedAt()
	if created.Before(before) || created.After(after) {
		t.Fatalf("CreatedAt %v not between %v and %v", created, before, after)
	}
}

func TestSize(t *testing.T) {
	s := NewStack(50, 30*time.Second)
	n := 5
	for i := 0; i < n; i++ {
		s.Push(newMock("cmd"))
	}
	if s.Size() != n {
		t.Fatalf("expected size %d, got %d", n, s.Size())
	}
}

type fakeChatStore struct {
	states map[string]message.ChatState
}

func (f *fakeChatStore) RestoreChatState(accountID, threadKey string, state message.ChatState) error {
	f.states[accountID+"/"+threadKey] = state
	return nil
}

func TestChatStateCommandUndo(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	later := now.Add(time.Hour)
	tests := []struct {
		name     string
		previous message.ChatState
		after    message.ChatState
	}{
		{"undo pin", message.ChatState{}, message.ChatState{PinnedAt: &now}},
		{"undo unpin", message.ChatState{PinnedAt: &now}, message.ChatState{}},
		{"undo snooze keeps pin", message.ChatState{PinnedAt: &now}, message.ChatState{PinnedAt: &now, SnoozedUntil: &later, SnoozedAt: &now}},
		{"undo unsnooze", message.ChatState{SnoozedUntil: &later, SnoozedAt: &now}, message.ChatState{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeChatStore{states: map[string]message.ChatState{"acct/k": tt.after}}
			stack := NewStack(10, time.Minute)
			stack.Push(NewChatStateCommand(store, "acct", "k", tt.previous, tt.name))

			cmd := stack.Pop()
			if cmd == nil || cmd.Description() != tt.name {
				t.Fatalf("Pop = %v", cmd)
			}
			if err := cmd.Undo(); err != nil {
				t.Fatalf("Undo: %v", err)
			}
			if got := store.states["acct/k"]; !reflect.DeepEqual(got, tt.previous) {
				t.Errorf("state after undo = %+v, want %+v", got, tt.previous)
			}
		})
	}
}

type fakeSenderChatStore struct {
	choices map[string]*bool
	states  map[string]message.ChatState
}

func (f *fakeSenderChatStore) RestoreSenderChat(accountID, email string, choice *bool, state message.ChatState) error {
	f.choices[accountID+"/"+email] = choice
	f.states[accountID+"/"+email] = state
	return nil
}

func TestSenderChatCommandUndo(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	split, combined := false, true
	tests := []struct {
		name     string
		previous *bool
		state    message.ChatState
	}{
		{"undo combine", &split, message.ChatState{}},
		{"undo split restores pin", &combined, message.ChatState{PinnedAt: &now}},
		{"undo split restores default", nil, message.ChatState{PinnedAt: &now}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeSenderChatStore{
				choices: map[string]*bool{"acct/a@x": &split},
				states:  map[string]message.ChatState{},
			}
			stack := NewStack(10, time.Minute)
			stack.Push(NewSenderChatCommand(store, "acct", "a@x", tt.previous, tt.state, tt.name))

			cmd := stack.Pop()
			if cmd == nil || cmd.Description() != tt.name {
				t.Fatalf("Pop = %v", cmd)
			}
			if err := cmd.Undo(); err != nil {
				t.Fatalf("Undo: %v", err)
			}
			if got := store.choices["acct/a@x"]; got != tt.previous {
				t.Errorf("choice after undo = %v, want %v", got, tt.previous)
			}
			if got := store.states["acct/a@x"]; !reflect.DeepEqual(got, tt.state) {
				t.Errorf("state after undo = %+v, want %+v", got, tt.state)
			}
		})
	}
}
