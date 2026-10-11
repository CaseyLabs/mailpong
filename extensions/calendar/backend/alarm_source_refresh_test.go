package backend

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	coreapi "github.com/caseylabs/mailpong/internal/core/api/v1"
)

// seedSourceCalendar inserts a local source with one calendar.
func seedSourceCalendar(t *testing.T, store *Store, srcID, calID string) {
	t.Helper()
	err := store.WithTx(func(tx *sql.Tx) error {
		if err := store.CreateSourceTx(tx, Source{
			ID: srcID, Type: SourceTypeLocal, Name: srcID, Enabled: true, Writable: true,
		}); err != nil {
			return err
		}
		return store.CreateCalendarTx(tx, Calendar{
			ID: calID, SourceID: srcID, URL: calID, DisplayName: calID, Visible: true,
		})
	})
	if err != nil {
		t.Fatalf("seed %s: %v", srcID, err)
	}
}

// alarmEvent builds a one-off event starting at start with a 30-minute alarm.
func alarmEvent(id, calID string, start time.Time) Event {
	stamp := start.UTC().Format("20060102T150405Z")
	end := start.Add(time.Hour).UTC().Format("20060102T150405Z")
	return Event{
		ID:          id,
		CalendarID:  calID,
		UID:         id,
		Summary:     id,
		DTStartUnix: start.Unix(),
		DTEndUnix:   start.Add(time.Hour).Unix(),
		ICSBlob: wrapICS(fmt.Sprintf(`BEGIN:VEVENT
UID:%s
DTSTART:%s
DTEND:%s
SUMMARY:%s
BEGIN:VALARM
ACTION:DISPLAY
TRIGGER:-PT30M
END:VALARM
END:VEVENT`, id, stamp, end, id)),
	}
}

func upsertEvents(t *testing.T, store *Store, evs ...Event) {
	t.Helper()
	err := store.WithTx(func(tx *sql.Tx) error {
		for _, ev := range evs {
			if err := store.UpsertEventTx(tx, ev); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("upsert events: %v", err)
	}
}

func TestListOverridesForCalendars(t *testing.T) {
	store := newTestStore(t)
	seedSourceCalendar(t, store, "src-a", "cal-a")
	seedSourceCalendar(t, store, "src-b", "cal-b")
	start := time.Date(2026, 6, 1, 14, 0, 0, 0, time.UTC)
	upsertEvents(t, store,
		alarmEvent("ev-a1", "cal-a", start),
		alarmEvent("ev-a2", "cal-a", start),
		alarmEvent("ev-b1", "cal-b", start),
	)
	err := store.WithTx(func(tx *sql.Tx) error {
		for _, o := range []struct {
			ev  string
			rec int64
		}{{"ev-a1", 100}, {"ev-a1", 200}, {"ev-b1", 300}} {
			if err := store.UpsertOverrideTx(tx, o.ev, o.rec, "blob"); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed overrides: %v", err)
	}

	tests := []struct {
		name      string
		calendars []string
		want      map[string][]int64 // event ID → sorted recurrence IDs
	}{
		{"no calendars", nil, map[string][]int64{}},
		{"one calendar", []string{"cal-a"}, map[string][]int64{"ev-a1": {100, 200}}},
		{"both calendars", []string{"cal-a", "cal-b"}, map[string][]int64{"ev-a1": {100, 200}, "ev-b1": {300}}},
		{"unknown calendar", []string{"cal-x"}, map[string][]int64{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := store.ListOverridesForCalendars(tc.calendars)
			if err != nil {
				t.Fatalf("ListOverridesForCalendars: %v", err)
			}
			gotIDs := make(map[string][]int64, len(got))
			for evID, ovs := range got {
				for _, o := range ovs {
					if o.EventID != evID {
						t.Errorf("override for %s filed under %s", o.EventID, evID)
					}
					gotIDs[evID] = append(gotIDs[evID], o.RecurrenceIDUnix)
				}
				slices.Sort(gotIDs[evID])
			}
			if fmt.Sprint(gotIDs) != fmt.Sprint(tc.want) {
				t.Errorf("got %v, want %v", gotIDs, tc.want)
			}
		})
	}
}

func TestRefreshSourceAlarms(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	start := now.Add(10 * time.Hour)
	fresh := alarmRow{start.Unix(), start.Add(-30 * time.Minute).Unix(), "pending"}
	stale := alarmRow{start.Unix(), start.Add(-2 * time.Hour).Unix(), "pending"}

	tests := []struct {
		name      string
		refresh   func(*Store) error
		wantA     []alarmRow
		wantB     []alarmRow
		dontWantA []alarmRow
		dontWantB []alarmRow
	}{
		{
			name:      "source A only leaves B untouched",
			refresh:   func(s *Store) error { return RefreshSourceAlarms(context.Background(), s, "src-a", now) },
			wantA:     []alarmRow{fresh},
			dontWantA: []alarmRow{stale},
			wantB:     []alarmRow{stale},
			dontWantB: []alarmRow{fresh},
		},
		{
			name:      "unknown source changes nothing",
			refresh:   func(s *Store) error { return RefreshSourceAlarms(context.Background(), s, "src-x", now) },
			wantA:     []alarmRow{stale},
			wantB:     []alarmRow{stale},
			dontWantA: []alarmRow{fresh},
			dontWantB: []alarmRow{fresh},
		},
		{
			name:      "all sources",
			refresh:   func(s *Store) error { return RefreshAllAlarms(context.Background(), s, now) },
			wantA:     []alarmRow{fresh},
			wantB:     []alarmRow{fresh},
			dontWantA: []alarmRow{stale},
			dontWantB: []alarmRow{stale},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newTestStore(t)
			seedSourceCalendar(t, store, "src-a", "cal-a")
			seedSourceCalendar(t, store, "src-b", "cal-b")
			upsertEvents(t, store, alarmEvent("ev-a", "cal-a", start), alarmEvent("ev-b", "cal-b", start))
			err := store.WithTx(func(tx *sql.Tx) error {
				for _, ev := range []string{"ev-a", "ev-b"} {
					if err := store.ReplacePendingAlarmsTx(tx, ev, now.Unix(), []Alarm{{
						ID: "stale-" + ev, EventID: ev, InstanceUnix: stale.instance,
						TriggerUnix: stale.trigger, Action: "display",
					}}); err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				t.Fatalf("seed alarms: %v", err)
			}

			if err := tc.refresh(store); err != nil {
				t.Fatalf("refresh: %v", err)
			}
			check := func(eventID string, want, dontWant []alarmRow) {
				got := listAlarmRows(t, store, eventID)
				for _, w := range want {
					if !hasAlarm(got, w) {
						t.Errorf("%s: missing alarm %+v in %+v", eventID, w, got)
					}
				}
				for _, d := range dontWant {
					if hasAlarm(got, d) {
						t.Errorf("%s: unexpected alarm %+v", eventID, d)
					}
				}
				if len(got) != len(want) {
					t.Errorf("%s: got %d alarms %+v, want %d", eventID, len(got), got, len(want))
				}
			}
			check("ev-a", tc.wantA, tc.dontWantA)
			check("ev-b", tc.wantB, tc.dontWantB)
		})
	}
}

func TestSyncedSourceID(t *testing.T) {
	tests := []struct {
		name    string
		payload any
		want    string
	}{
		{"source id", map[string]any{"sourceId": "src-a"}, "src-a"},
		{"empty map", map[string]any{}, ""},
		{"nil", nil, ""},
		{"wrong type", map[string]any{"sourceId": 7}, ""},
		{"not a map", "src-a", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := syncedSourceID(tc.payload); got != tc.want {
				t.Errorf("syncedSourceID(%v) = %q, want %q", tc.payload, got, tc.want)
			}
		})
	}
}

func TestAlarmSchedulerCoalescesRefreshRequests(t *testing.T) {
	tests := []struct {
		name        string
		requests    []string // "" = every source
		wantAll     bool
		wantSources []string
	}{
		{"none", nil, false, nil},
		{"one source", []string{"a"}, false, []string{"a"}},
		{"repeated sources collapse", []string{"a", "b", "a", "b", "a"}, false, []string{"a", "b"}},
		{"full refresh absorbs sources", []string{"a", "", "b"}, true, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := NewAlarmScheduler(nil, nil, nil, nil)
			for _, r := range tc.requests {
				s.requestRefresh(r)
			}
			all, sources := s.takeRefreshRequests()
			slices.Sort(sources)
			if all != tc.wantAll || !slices.Equal(sources, tc.wantSources) {
				t.Errorf("got (%v, %v), want (%v, %v)", all, sources, tc.wantAll, tc.wantSources)
			}
			if all, sources := s.takeRefreshRequests(); all || len(sources) != 0 {
				t.Errorf("requests not cleared: (%v, %v)", all, sources)
			}
		})
	}
}

// dispatchingEventBus delivers published events to subscribers synchronously,
// like the host event bus.
type dispatchingEventBus struct {
	mu       sync.Mutex
	handlers map[string][]func(any)
}

func (b *dispatchingEventBus) Publish(name string, payload any) error {
	b.mu.Lock()
	hs := slices.Clone(b.handlers[name])
	b.mu.Unlock()
	for _, h := range hs {
		h(payload)
	}
	return nil
}

func (b *dispatchingEventBus) Subscribe(name string, fn func(any)) (coreapi.Unsubscribe, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.handlers == nil {
		b.handlers = make(map[string][]func(any))
	}
	b.handlers[name] = append(b.handlers[name], fn)
	idx := len(b.handlers[name]) - 1
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		b.handlers[name][idx] = func(any) {}
	}, nil
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestAlarmSchedulerBackgroundRefresh checks that Start's initial pass and a
// per-source sync-complete both materialize and arm alarms off the caller's
// goroutine, and that concurrent sync events and Stop are race-free.
func TestAlarmSchedulerBackgroundRefresh(t *testing.T) {
	store := newTestStore(t)
	seedSourceCalendar(t, store, "src-a", "cal-a")
	seedSourceCalendar(t, store, "src-b", "cal-b")
	start := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	upsertEvents(t, store, alarmEvent("ev-a", "cal-a", start))

	bus := &dispatchingEventBus{}
	s := NewAlarmScheduler(store, nil, bus, nil)
	s.Start(context.Background())

	armed := func(n int) func() bool {
		return func() bool {
			s.mu.Lock()
			defer s.mu.Unlock()
			return len(s.timers) == n
		}
	}
	waitFor(t, "initial alarm armed", armed(1))

	// A source-b event arrives through sync; its sync-complete refreshes it.
	upsertEvents(t, store, alarmEvent("ev-b", "cal-b", start))
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			src := []string{"src-a", "src-b"}[i%2]
			_ = bus.Publish("calendar:sync-complete", map[string]any{"sourceId": src})
		}(i)
	}
	wg.Wait()
	waitFor(t, "source-b alarm armed", armed(2))

	for _, ev := range []string{"ev-a", "ev-b"} {
		rows := listAlarmRows(t, store, ev)
		if len(rows) != 1 || rows[0].trigger != start.Add(-30*time.Minute).Unix() {
			t.Errorf("%s alarms = %+v, want one at start-30m", ev, rows)
		}
	}

	s.Stop()
	if !armed(0)() {
		t.Error("timers left armed after Stop")
	}
	_ = bus.Publish("calendar:sync-complete", map[string]any{"sourceId": "src-a"})
}
