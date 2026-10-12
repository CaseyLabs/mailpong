package app

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSendQueue(t *testing.T) {
	const delay = 30 * time.Millisecond
	tests := []struct {
		name     string
		send     sendFunc
		act      func(t *testing.T, q *sendQueue, id string) // runs right after add
		wantRuns int32
		want     *SendResult // nil: no notification
	}{
		{
			name:     "fires after the delay",
			send:     func() (string, error) { return "", nil },
			act:      func(t *testing.T, q *sendQueue, id string) { time.Sleep(3 * delay) },
			wantRuns: 1,
			want:     &SendResult{},
		},
		{
			name:     "reports a failure with its draft",
			send:     func() (string, error) { return "d1", errors.New("smtp down") },
			act:      func(t *testing.T, q *sendQueue, id string) { time.Sleep(3 * delay) },
			wantRuns: 1,
			want:     &SendResult{Error: "smtp down", DraftID: "d1"},
		},
		{
			name:     "reports a panic as a failure",
			send:     func() (string, error) { panic("boom") },
			act:      func(t *testing.T, q *sendQueue, id string) { time.Sleep(3 * delay) },
			wantRuns: 1,
			want:     &SendResult{Error: "send failed: boom"},
		},
		{
			name: "cancel before the delay stops it",
			send: func() (string, error) { return "", nil },
			act:  func(t *testing.T, q *sendQueue, id string) { _ = q.cancel(id); time.Sleep(3 * delay) },
		},
		{
			name: "cancel after it started fails",
			send: func() (string, error) { time.Sleep(2 * delay); return "", nil },
			act: func(t *testing.T, q *sendQueue, id string) {
				time.Sleep(delay + delay/2)
				if q.cancel(id) {
					t.Error("cancel succeeded on a running send")
				}
				q.flush(time.Second, false)
			},
			wantRuns: 1,
			want:     &SendResult{},
		},
		{
			name:     "flush sends at once and waits",
			send:     func() (string, error) { time.Sleep(delay / 3); return "", nil },
			act:      func(t *testing.T, q *sendQueue, id string) { q.flush(time.Second, false) },
			wantRuns: 1,
			want:     &SendResult{},
		},
		{
			name:     "muted flush sends without notifying",
			send:     func() (string, error) { return "", nil },
			act:      func(t *testing.T, q *sendQueue, id string) { q.flush(time.Second, true) },
			wantRuns: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mu sync.Mutex
			var got []SendResult
			q := newSendQueue(delay, func(r SendResult) { mu.Lock(); got = append(got, r); mu.Unlock() })
			var runs atomic.Int32
			id := q.add(func() (string, error) { runs.Add(1); return tt.send() })
			tt.act(t, q, id)
			q.flush(time.Second, false)

			if n := runs.Load(); n != tt.wantRuns {
				t.Errorf("runs = %d, want %d", n, tt.wantRuns)
			}
			mu.Lock()
			defer mu.Unlock()
			if tt.want == nil {
				if len(got) != 0 {
					t.Errorf("notified %v, want none", got)
				}
				return
			}
			want := *tt.want
			want.ID = id
			if len(got) != 1 || got[0] != want {
				t.Errorf("notified %v, want [%v]", got, want)
			}
		})
	}
}

func TestSendQueueCancel(t *testing.T) {
	q := newSendQueue(time.Hour, nil)
	id := q.add(func() (string, error) { t.Error("cancelled send ran"); return "", nil })
	if !q.cancel(id) {
		t.Fatal("first cancel failed")
	}
	if q.cancel(id) {
		t.Error("second cancel succeeded")
	}
	if q.cancel("send-unknown") {
		t.Error("cancel of an unknown id succeeded")
	}
	q.flush(time.Second, true)
}

func TestSendQueueAddAfterFlushRunsAtOnce(t *testing.T) {
	q := newSendQueue(time.Hour, nil)
	q.flush(time.Second, true)
	done := make(chan struct{})
	q.add(func() (string, error) { close(done); return "", nil })
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("send added after flush did not run")
	}
}

func TestSendQueueFlushTimesOut(t *testing.T) {
	q := newSendQueue(time.Hour, nil)
	release := make(chan struct{})
	defer close(release)
	q.add(func() (string, error) { <-release; return "", nil })
	start := time.Now()
	q.flush(50*time.Millisecond, true)
	if d := time.Since(start); d > time.Second {
		t.Errorf("flush waited %v past its bound", d)
	}
}
