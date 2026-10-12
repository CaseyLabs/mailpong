package backend

// AlarmScheduler arms desktop notifications for VALARM-bearing events.
// Phase 1G.
//
// Lifecycle:
//   - Created in CalendarBridge.ensureInit (lazy; disabled extensions
//     don't allocate it).
//   - Start subscribes to calendar:sync-complete (so newly-arrived alarms
//     get armed) and system:wake (so missed alarms during sleep are swept
//     to 'fired' without firing-after-the-fact, and future ones re-armed).
//   - time.AfterFunc timers cover the 24h horizon. On wrap-around (>24h
//     out), Reevaluate doesn't arm anything for that alarm yet — the next
//     hourly tick or sync re-pass will pick it up.
//
// Writes and CalDAV syncs materialize alarms for the events they touch.
// A background worker recomputes alarms (alarm.go) so recurring events
// keep upcoming alarms, Google and Microsoft events get theirs, and stale
// pending rows are dropped: every event on Start, wake, the hourly tick
// and a sync-complete without a sourceId; only the synced source's events
// on a sync-complete that names one. Requests that arrive while a pass
// runs are coalesced into the next pass, so Start doesn't block and a
// burst of per-source syncs doesn't queue repeated work. Reevaluate then
// arms time.AfterFunc callbacks for the pending rows inside the horizon.

import (
	"context"
	"fmt"
	"sync"
	"time"

	coreapi "github.com/caseylabs/mailven/internal/core/api/v1"
)

const alarmHorizon = 24 * time.Hour

// alarmRefreshInterval is how often the alarm window is rolled forward.
const alarmRefreshInterval = time.Hour

type AlarmScheduler struct {
	store  *Store
	notif  coreapi.Notifications
	events coreapi.EventBus
	log    coreapi.Logger
	ctx    context.Context
	cancel context.CancelFunc

	mu     sync.Mutex
	timers map[string]*time.Timer // alarmID → timer

	unsubs []func()

	// Refresh requests waiting for the worker, guarded by mu. refreshAll
	// covers every source; refreshSources names single synced sources.
	refreshAll     bool
	refreshSources map[string]struct{}
	refreshWake    chan struct{} // buffered(1): signals pending requests
	workerDone     chan struct{} // closed when the worker exits; nil if not started

	// evalMu serializes Reevaluate's read-then-arm so a slower call can't
	// re-arm timers from a pending set a newer call already replaced.
	evalMu sync.Mutex
}

func NewAlarmScheduler(store *Store, notif coreapi.Notifications, events coreapi.EventBus, log coreapi.Logger) *AlarmScheduler {
	return &AlarmScheduler{
		store:          store,
		notif:          notif,
		events:         events,
		log:            log,
		timers:         make(map[string]*time.Timer),
		refreshSources: make(map[string]struct{}),
		refreshWake:    make(chan struct{}, 1),
	}
}

// warn logs a formatted warning via the extension's coreapi.Logger. Nil-
// safe: a nil logger means construction time skipped wiring (e.g., in a
// future test), and we simply drop the message rather than panic.
func (s *AlarmScheduler) warn(format string, args ...any) {
	if s.log == nil {
		return
	}
	s.log.Warn(fmt.Sprintf(format, args...))
}

// Start subscribes to events and launches the background worker, which
// sweeps past alarms, runs the initial full refresh and arms the pending
// alarms in the 24h horizon. It returns without waiting for that pass.
// Safe to call once. Returns a cancel func the caller (bridge ensureInit)
// can ignore — Stop is the canonical teardown path.
func (s *AlarmScheduler) Start(ctx context.Context) context.CancelFunc {
	s.mu.Lock()
	if s.ctx != nil {
		cancel := s.cancel
		s.mu.Unlock()
		return cancel
	}
	s.ctx, s.cancel = context.WithCancel(ctx)
	runCtx, cancel := s.ctx, s.cancel
	s.workerDone = make(chan struct{})
	done := s.workerDone
	s.refreshAll = true // initial pass, run by the worker
	s.mu.Unlock()

	// Ignore Subscribe errors so a missing EventBus doesn't block
	// scheduling — the hourly tick still keeps alarms fresh.
	if s.events != nil {
		syncUnsub, _ := s.events.Subscribe("calendar:sync-complete", func(payload any) {
			s.requestRefresh(syncedSourceID(payload))
		})
		wakeUnsub, _ := s.events.Subscribe("system:wake", func(_ any) {
			// Sweep past alarms first; user was asleep, don't fire-after.
			if err := s.store.MarkPastAlarmsFired(time.Now().Unix()); err != nil {
				s.warn("mark past on wake: %v", err)
			}
			s.requestRefresh("")
		})
		s.mu.Lock()
		s.unsubs = append(s.unsubs, syncUnsub, wakeUnsub)
		s.mu.Unlock()
	}

	go s.run(runCtx, done)
	return cancel
}

// syncedSourceID returns the sourceId carried by a calendar:sync-complete
// payload, or "" (meaning every source) when it names none.
func syncedSourceID(payload any) string {
	m, ok := payload.(map[string]any)
	if !ok {
		return ""
	}
	id, _ := m["sourceId"].(string)
	return id
}

// requestRefresh queues a refresh of one source's alarms, or of every
// source when sourceID is "", and wakes the worker. Never blocks.
func (s *AlarmScheduler) requestRefresh(sourceID string) {
	s.mu.Lock()
	if sourceID == "" {
		s.refreshAll = true
	} else {
		s.refreshSources[sourceID] = struct{}{}
	}
	s.mu.Unlock()
	select {
	case s.refreshWake <- struct{}{}:
	default: // a wake-up is already pending
	}
}

// takeRefreshRequests returns and clears the queued refresh requests.
func (s *AlarmScheduler) takeRefreshRequests() (all bool, sources []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all = s.refreshAll
	if !all {
		for id := range s.refreshSources {
			sources = append(sources, id)
		}
	}
	s.refreshAll = false
	clear(s.refreshSources)
	return all, sources
}

// run is the worker: it handles queued refresh requests one pass at a time
// and rolls the alarm window forward every alarmRefreshInterval until ctx
// is cancelled by Stop.
func (s *AlarmScheduler) run(ctx context.Context, done chan struct{}) {
	defer close(done)

	// Sweep alarms that should have fired in the past so they don't
	// notify retroactively when we arm.
	if err := s.store.MarkPastAlarmsFired(time.Now().Unix()); err != nil {
		s.warn("mark past alarms fired: %v", err)
	}

	ticker := time.NewTicker(alarmRefreshInterval)
	defer ticker.Stop()
	for {
		if all, sources := s.takeRefreshRequests(); all || len(sources) > 0 {
			s.refresh(ctx, all, sources)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.requestRefresh("")
		case <-s.refreshWake:
		}
	}
}

// refresh recomputes upcoming alarms for every source (all) or the listed
// sources, then re-arms timers. Errors are logged; a partial refresh still
// arms what it produced.
func (s *AlarmScheduler) refresh(ctx context.Context, all bool, sources []string) {
	now := time.Now()
	if all {
		if err := RefreshAllAlarms(ctx, s.store, now); err != nil && ctx.Err() == nil {
			s.warn("refresh alarms: %v", err)
		}
	}
	for _, id := range sources {
		if ctx.Err() != nil {
			return
		}
		if err := RefreshSourceAlarms(ctx, s.store, id, now); err != nil && ctx.Err() == nil {
			s.warn("refresh alarms for source %s: %v", id, err)
		}
	}
	if ctx.Err() != nil {
		return
	}
	if err := s.Reevaluate(); err != nil {
		s.warn("reevaluate alarms: %v", err)
	}
}

// Stop cancels event subscriptions, waits for the worker to exit, then
// stops all timers.
func (s *AlarmScheduler) Stop() {
	s.mu.Lock()
	unsubs := s.unsubs
	s.unsubs = nil
	cancel, done := s.cancel, s.workerDone
	s.cancel, s.ctx, s.workerDone = nil, nil, nil
	s.mu.Unlock()

	for _, u := range unsubs {
		u()
	}
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}

	s.mu.Lock()
	for id, t := range s.timers {
		t.Stop()
		delete(s.timers, id)
	}
	s.mu.Unlock()
}

// Reevaluate scans pending alarms in [now, now+24h], stops any timers no
// longer matching pending rows (deletes / dismissals), and arms a
// time.AfterFunc for each new row. Idempotent — safe to call repeatedly.
func (s *AlarmScheduler) Reevaluate() error {
	s.evalMu.Lock()
	defer s.evalMu.Unlock()
	now := time.Now().Unix()
	pending, err := s.store.PendingAlarmsInRange(now, now+int64(alarmHorizon.Seconds()))
	if err != nil {
		return fmt.Errorf("list pending: %w", err)
	}

	wantArmed := make(map[string]struct{}, len(pending))
	for _, a := range pending {
		wantArmed[a.ID] = struct{}{}
	}

	s.mu.Lock()
	// Drop any timer that's no longer in the pending set.
	for id, t := range s.timers {
		if _, ok := wantArmed[id]; !ok {
			t.Stop()
			delete(s.timers, id)
		}
	}
	// Arm new timers.
	for _, a := range pending {
		if _, ok := s.timers[a.ID]; ok {
			continue
		}
		delay := time.Until(time.Unix(a.TriggerUnix, 0))
		if delay < 0 {
			delay = 0
		}
		alarmID := a.ID
		s.timers[alarmID] = time.AfterFunc(delay, func() {
			s.dispatch(alarmID)
		})
	}
	s.mu.Unlock()
	return nil
}

// dispatch fires one alarm: re-reads the row to confirm pending status
// (user may have dismissed in the UI between arming and firing), shows
// the notification, marks fired.
func (s *AlarmScheduler) dispatch(alarmID string) {
	s.mu.Lock()
	delete(s.timers, alarmID)
	s.mu.Unlock()

	a, err := s.store.GetAlarm(alarmID)
	if err != nil || a == nil || a.Status != "pending" {
		return
	}

	// Only DISPLAY actions surface as desktop notifications in 1G.
	if a.Action != "display" {
		_ = s.store.MarkAlarmFired(alarmID, time.Now().Unix())
		return
	}

	ev, err := s.store.GetEvent(a.EventID)
	if err != nil || ev == nil {
		_ = s.store.MarkAlarmFired(alarmID, time.Now().Unix())
		return
	}

	title := ev.Summary
	if title == "" {
		title = "(no title)"
	}
	body := formatAlarmBody(*ev, a, time.Unix(a.InstanceUnix, 0))

	if s.notif != nil {
		_ = s.notif.Show(coreapi.NotifyRequest{
			Title: title,
			Body:  body,
			OnClick: coreapi.NotifyClickAction{
				Kind:        "open-extension",
				ExtensionID: "calendar",
				Path:        "/event/" + ev.ID,
			},
		})
	}

	_ = s.store.MarkAlarmFired(alarmID, time.Now().Unix())
}

// formatAlarmBody renders a single-line notification body in the host's
// system locale. Backend-only formatting — svelte-i18n isn't reachable from
// Go. Future scope: pipe a locale string from the user's UI preference.
func formatAlarmBody(ev Event, _ *Alarm, instanceStart time.Time) string {
	when := instanceStart.Local().Format("Mon Jan 2, 3:04 PM")
	if ev.IsAllDay {
		when = instanceStart.Local().Format("Mon Jan 2") + " (all day)"
	}
	if ev.Location != "" {
		return when + " · " + ev.Location
	}
	return when
}
