package backend

import (
	"sync"
	"testing"

	coreapi "github.com/caseylabs/mailven/internal/core/api/v1"
)

// countingEventBus tracks live subscriptions per event name.
type countingEventBus struct {
	recordingEventBus
	mu   sync.Mutex
	live map[string]int
}

func (b *countingEventBus) Subscribe(name string, _ func(any)) (coreapi.Unsubscribe, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.live[name]++
	var once sync.Once
	return func() {
		once.Do(func() {
			b.mu.Lock()
			b.live[name]--
			b.mu.Unlock()
		})
	}, nil
}

func (b *countingEventBus) count(name string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.live[name]
}

// TestSyncerSubscriptionsPerInstance checks that every Syncer subscribes to
// wake/network events (a re-created one included) and that its stop func
// removes those subscriptions.
func TestSyncerSubscriptionsPerInstance(t *testing.T) {
	bus := &countingEventBus{live: map[string]int{}}
	store := newTestStore(t)
	first := NewSyncer(store, fakeSecrets{}, bus, nil, nil, nil, noopLogger{})
	second := NewSyncer(store, fakeSecrets{}, bus, nil, nil, nil, noopLogger{})

	stopFirst := first.Start()
	stopSecond := second.Start()
	second.Start() // a repeat Start must not subscribe twice
	if got := bus.count("system:wake"); got != 2 {
		t.Fatalf("live wake subscriptions = %d, want 2", got)
	}

	stopFirst()
	stopSecond()
	// Let the initial background sync finish so it cannot write under the
	// TempDir while t.TempDir's cleanup removes it.
	first.wait()
	second.wait()
	for _, name := range []string{"system:wake", "system:network-online"} {
		if got := bus.count(name); got != 0 {
			t.Errorf("live %s subscriptions after stop = %d, want 0", name, got)
		}
	}
}
