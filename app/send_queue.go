package app

import (
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/caseylabs/mailven/internal/logging"
	"github.com/rs/zerolog"
)

// sendDelay is how long a queued send waits, so it can be undone.
const sendDelay = 10 * time.Second

// sendFlushWait bounds how long quitting waits for queued sends to go out.
const sendFlushWait = 30 * time.Second

// sendFunc performs a queued send. On failure it returns the ID of the draft
// that keeps the message, if any.
type sendFunc func() (draftID string, err error)

// SendResult is the send:finished event payload.
type SendResult struct {
	ID      string `json:"id"`
	Error   string `json:"error"`
	DraftID string `json:"draftId"` // draft keeping a message that failed to send
}

// sendQueue holds sends for a delay before running them, so they can be
// undone. Flushing runs every waiting send at once, for quitting.
type sendQueue struct {
	mu      sync.Mutex
	delay   time.Duration
	notify  func(SendResult) // nil once muted
	waiting map[string]*time.Timer
	sends   map[string]sendFunc
	next    int
	flushed bool // after a flush, new sends run at once
	running sync.WaitGroup
}

func newSendQueue(delay time.Duration, notify func(SendResult)) *sendQueue {
	return &sendQueue{delay: delay, notify: notify, waiting: map[string]*time.Timer{}, sends: map[string]sendFunc{}}
}

// add queues send and returns its ID for cancel.
func (q *sendQueue) add(send sendFunc) string {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.next++
	id := fmt.Sprintf("send-%d", q.next)
	q.sends[id] = send
	if q.flushed {
		q.startLocked(id)
		return id
	}
	q.waiting[id] = time.AfterFunc(q.delay, func() {
		q.mu.Lock()
		defer q.mu.Unlock()
		q.startLocked(id)
	})
	return id
}

// cancel stops a send that hasn't started and reports whether it did.
func (q *sendQueue) cancel(id string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	t, ok := q.waiting[id]
	if !ok {
		return false
	}
	t.Stop()
	delete(q.waiting, id)
	delete(q.sends, id)
	return true
}

// flush starts every waiting send and waits up to wait for all running sends
// to finish. Sends added afterwards run at once. With mute set, results are
// no longer reported (the frontend is gone).
func (q *sendQueue) flush(wait time.Duration, mute bool) {
	q.mu.Lock()
	q.flushed = true
	if mute {
		q.notify = nil
	}
	for id := range q.sends {
		q.startLocked(id)
	}
	q.mu.Unlock()

	done := make(chan struct{})
	go func() {
		q.running.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(wait):
		sendLog().Warn().Msg("Queued sends still running at quit")
	}
}

// startLocked runs send id in its own goroutine unless it was cancelled or
// already started. q.mu must be held.
func (q *sendQueue) startLocked(id string) {
	send, ok := q.sends[id]
	if !ok {
		return
	}
	if t := q.waiting[id]; t != nil {
		t.Stop()
	}
	delete(q.waiting, id)
	delete(q.sends, id)
	q.running.Add(1)
	go func() {
		defer q.running.Done()
		res := SendResult{ID: id}
		if draftID, err := runSend(send); err != nil {
			sendLog().Warn().Err(err).Str("sendID", id).Msg("Queued send failed")
			res.Error, res.DraftID = err.Error(), draftID
		}
		q.mu.Lock()
		notify := q.notify
		q.mu.Unlock()
		if notify != nil {
			notify(res)
		}
	}()
}

// runSend calls send, turning a panic into an error so the outcome is still
// reported.
func runSend(send sendFunc) (draftID string, err error) {
	defer func() {
		if r := recover(); r != nil {
			sendLog().Error().Interface("panic", r).Str("stack", string(debug.Stack())).Msg("Queued send panicked (recovered)")
			err = fmt.Errorf("send failed: %v", r)
		}
	}()
	return send()
}

func sendLog() *zerolog.Logger {
	log := logging.WithComponent("app.send")
	return &log
}

// UndoSend cancels a queued send that hasn't started and reports whether it
// did. The message stays in its draft, if it had one.
func (a *App) UndoSend(id string) bool {
	return a.sendQueue.cancel(id)
}

// finishBeforeQuit waits (bounded) for the frontend to save chat drafts, then
// sends every queued message, so quitting doesn't drop either.
func (a *App) finishBeforeQuit() {
	a.waitChatDraftsFlushed()
	a.sendQueue.flush(sendFlushWait, false)
}
