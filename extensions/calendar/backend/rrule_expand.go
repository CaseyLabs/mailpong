package backend

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/teambition/rrule-go"
)

// Expansion limits per series. A rule like FREQ=SECONDLY, or one that
// started long before the window, would otherwise generate millions of
// occurrences. maxExpandSteps bounds the occurrences walked from DTSTART;
// maxExpandInstances bounds those returned in the window.
const (
	maxExpandSteps     = 100_000
	maxExpandInstances = 5_000
)

// ExpandInRange expands a stored Event into concrete EventInstances within
// `[from, to]`. Non-recurring events return at most one instance; recurring
// events return zero or more from `Component.RecurrenceSet(loc)`, capped by
// the expansion limits.
// RECURRENCE-ID overrides REPLACE the matching default-expanded instance
// (matched by occurrence start time = override's RECURRENCE-ID).
//
// Returns instances sorted by InstanceStartUnix ASC.
func ExpandInRange(ev Event, overrides []EventOverride, from, to time.Time) ([]EventInstance, error) {
	if from.After(to) {
		return nil, nil
	}

	// Non-recurring: include if the event overlaps the window, unless the
	// server cancelled it.
	if ev.RRuleText == "" {
		if masterCancelled(ev.ICSBlob) {
			return nil, nil
		}
		evStart := time.Unix(ev.DTStartUnix, 0)
		evEnd := time.Unix(ev.DTEndUnix, 0)
		if evEnd.Before(from) || evStart.After(to) {
			return nil, nil
		}
		return []EventInstance{{
			Event:             ev,
			InstanceStartUnix: ev.DTStartUnix,
			InstanceEndUnix:   ev.DTEndUnix,
		}}, nil
	}

	// Recurring: re-parse the master ICS blob, run RecurrenceSet,
	// generate the occurrence list in window, then apply overrides.
	loc := resolveLocation(ev.TZName)
	cal, err := decodeICS(ev.ICSBlob)
	if err != nil {
		return nil, fmt.Errorf("rrule_expand: decode master ICS: %w", err)
	}
	events := cal.Events()
	if len(events) == 0 {
		return nil, fmt.Errorf("rrule_expand: master ICS has no VEVENT")
	}

	// Pick the VEVENT without RECURRENCE-ID (the master).
	var masterEv *ical.Event
	for i := range events {
		if events[i].Props.Get(ical.PropRecurrenceID) == nil {
			e := events[i]
			masterEv = &e
			break
		}
	}
	if masterEv == nil {
		// Edge case — treat the first as master.
		first := events[0]
		masterEv = &first
	}
	if isCancelled(masterEv) {
		return nil, nil // whole series cancelled on the server
	}

	// RFC 5545 allows EXDATE/RDATE to carry a comma-separated list in one
	// property, but go-ical's RecurrenceSet parses each property's whole value
	// with a single time.Parse and errors on the comma — which would drop the
	// entire calendar. Split multi-value props into single-value props first.
	splitMultiValueDateLists(masterEv)

	set, err := masterEv.RecurrenceSet(loc)
	if err != nil {
		return nil, fmt.Errorf("rrule_expand: build recurrence set: %w", err)
	}

	// Start the walk one master duration early so a multi-day occurrence
	// that began before the window but runs into it is included.
	masterDuration := ev.DTEndUnix - ev.DTStartUnix
	walkFrom := from.Add(-time.Duration(masterDuration) * time.Second)
	occurrences := occurrencesBetween(set, walkFrom, to)

	// Index overrides by their RECURRENCE-ID for O(1) lookup. Multiple
	// overrides at the same instant shouldn't happen; if they do, the last
	// one wins (per RFC 5545, the most recent definition).
	overrideByInstant := make(map[int64]EventOverride, len(overrides))
	for _, ov := range overrides {
		overrideByInstant[ov.RecurrenceIDUnix] = ov
	}

	out := make([]EventInstance, 0, len(occurrences))
	add := func(inst EventInstance) {
		if overlapsWindow(inst, from, to) {
			out = append(out, inst)
		}
	}
	for _, occ := range occurrences {
		instUnix := occ.Unix()
		ov, ok := overrideByInstant[instUnix]
		delete(overrideByInstant, instUnix)
		if !ok {
			add(defaultInstance(ev, instUnix, masterDuration))
			continue
		}
		// Apply override: parse its ICS, extract DTSTART/DTEND/SUMMARY,
		// build an EventInstance using overrides where present and
		// master values where absent.
		inst, err := applyOverride(ev, ov)
		if errors.Is(err, errOverrideCancelled) {
			continue // occurrence cancelled on the server
		}
		if err != nil {
			// Skip malformed override; fall back to default expansion.
			add(defaultInstance(ev, instUnix, masterDuration))
			continue
		}
		inst.RecurrenceIDUnix = instUnix
		add(inst)
	}

	// An override can move its occurrence into the window from a slot
	// outside the walked range. Unmatched overrides whose slot lies inside
	// that range belong to excluded occurrences and stay hidden.
	for rid, ov := range overrideByInstant {
		if rid >= walkFrom.Unix() && rid <= to.Unix() {
			continue
		}
		inst, err := applyOverride(ev, ov)
		if err != nil {
			continue
		}
		inst.RecurrenceIDUnix = rid
		add(inst)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].InstanceStartUnix < out[j].InstanceStartUnix
	})

	return out, nil
}

// splitMultiValueDateLists rewrites EXDATE/RDATE properties on the master
// VEVENT so that each property carries a single date value. RFC 5545 permits
// a comma-separated list in one property (e.g. EXDATE:20250501T084500,
// 20250502T084500), and MailPong's own instance-delete writes coalesce that way,
// but go-ical's RecurrenceSet calls time.Parse on the whole value and fails on
// the comma. Each split prop clones the original's Params so TZID / VALUE=DATE
// semantics are preserved per value. Single-value props are left untouched.
// This mutates the in-memory master only — never the stored ICS blob.
func splitMultiValueDateLists(comp *ical.Event) {
	for _, name := range []string{ical.PropExceptionDates, ical.PropRecurrenceDates} {
		props := comp.Props.Values(name)
		if len(props) == 0 {
			continue
		}
		expanded := make([]ical.Prop, 0, len(props))
		for _, p := range props {
			if !strings.Contains(p.Value, ",") {
				expanded = append(expanded, p)
				continue
			}
			for _, v := range strings.Split(p.Value, ",") {
				v = strings.TrimSpace(v)
				if v == "" {
					continue
				}
				expanded = append(expanded, ical.Prop{
					Name:   p.Name,
					Params: cloneParams(p.Params),
					Value:  v,
				})
			}
		}
		comp.Props[name] = expanded
	}
}

// cloneParams deep-copies a Params map so split EXDATE/RDATE props don't share
// (and thus can't mutate) one another's parameter slices.
func cloneParams(src ical.Params) ical.Params {
	if src == nil {
		return nil
	}
	dst := make(ical.Params, len(src))
	for k, v := range src {
		dst[k] = append([]string(nil), v...)
	}
	return dst
}

// resolveLocation looks up the IANA timezone name; returns time.Local on
// empty or unknown names so expansion still works (just less correct for
// DST around floating events).
func resolveLocation(tzName string) *time.Location {
	// tz-less recurring events (all-day / floating) expand in the configured
	// display tz, matching how the frontend buckets days. An explicit, valid
	// TZID still wins.
	if tzName == "" {
		return configuredTZ()
	}
	loc, err := loadTZ(tzName)
	if err != nil {
		return configuredTZ()
	}
	return loc
}

// isCancelled reports whether a VEVENT has STATUS:CANCELLED.
func isCancelled(ev *ical.Event) bool {
	return strings.EqualFold(propText(ev, ical.PropStatus), "CANCELLED")
}

// masterCancelled reports whether the master VEVENT in blob is cancelled. The
// substring check skips decoding the blob for the common case.
func masterCancelled(blob string) bool {
	if !strings.Contains(strings.ToUpper(blob), "CANCELLED") {
		return false
	}
	ev := masterEvent(blob)
	return ev != nil && isCancelled(ev)
}

// errOverrideCancelled marks an override whose STATUS is CANCELLED: the
// occurrence it replaces is not shown.
var errOverrideCancelled = errors.New("override cancelled")

// applyOverride parses an override's ICS blob and merges its non-empty
// fields onto the master to produce one EventInstance. Override semantics
// per RFC 5545 §3.8.4.4: an override is a full VEVENT — the fields it
// specifies replace the corresponding master fields for that instance only.
func applyOverride(master Event, ov EventOverride) (EventInstance, error) {
	cal, err := decodeICS(ov.ICSBlob)
	if err != nil {
		return EventInstance{}, err
	}
	events := cal.Events()
	if len(events) == 0 {
		return EventInstance{}, fmt.Errorf("override ICS has no VEVENT")
	}
	ev := events[0]
	if isCancelled(&ev) {
		return EventInstance{}, errOverrideCancelled
	}

	loc := resolveLocation(master.TZName)
	dtstartProp := ev.Props.Get(ical.PropDateTimeStart)
	if dtstartProp != nil {
		if tz := dtstartProp.Params.Get(ical.ParamTimezoneID); tz != "" {
			if l, lerr := loadTZ(tz); lerr == nil {
				loc = l
			}
		}
	}

	dtstart, err := ev.DateTimeStart(loc)
	if err != nil {
		return EventInstance{}, fmt.Errorf("override DTSTART: %w", err)
	}
	dtend, err := ev.DateTimeEnd(loc)
	if err != nil {
		dtend = dtstart
	}

	inst := EventInstance{
		Event:                master,
		InstanceStartUnix:    dtstart.Unix(),
		InstanceEndUnix:      dtend.Unix(),
		IsRecurrenceOverride: true,
	}
	if v := propText(&ev, ical.PropSummary); v != "" {
		inst.Summary = v
	}
	if v := propText(&ev, ical.PropDescription); v != "" {
		inst.Description = v
	}
	if v := propText(&ev, ical.PropLocation); v != "" {
		inst.Location = v
	}
	return inst, nil
}

// occurrencesBetween returns the set's occurrences in [from, to], stopping
// after maxExpandSteps occurrences from DTSTART or maxExpandInstances in the
// window, whichever comes first.
func occurrencesBetween(set *rrule.Set, from, to time.Time) []time.Time {
	next := set.Iterator()
	var out []time.Time
	for range maxExpandSteps {
		t, ok := next()
		if !ok || t.After(to) || len(out) >= maxExpandInstances {
			break
		}
		if !t.Before(from) {
			out = append(out, t)
		}
	}
	return out
}

// defaultInstance is the occurrence at instUnix with the master's duration.
func defaultInstance(ev Event, instUnix, duration int64) EventInstance {
	return EventInstance{
		Event:             ev,
		InstanceStartUnix: instUnix,
		InstanceEndUnix:   instUnix + duration,
		RecurrenceIDUnix:  instUnix,
	}
}

// overlapsWindow reports whether inst starts in [from, to] or started
// earlier and is still running at from.
func overlapsWindow(inst EventInstance, from, to time.Time) bool {
	if inst.InstanceStartUnix > to.Unix() {
		return false
	}
	return inst.InstanceStartUnix >= from.Unix() || inst.InstanceEndUnix > from.Unix()
}
