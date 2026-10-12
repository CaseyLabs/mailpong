package backend

import (
	"context"
	"database/sql"
	"errors"
	"github.com/caseylabs/mailven/internal/kit/davutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// noopLogger satisfies coreapi.Logger for tests that don't assert on log output.
type noopLogger struct{}

func (noopLogger) Debug(string) {}
func (noopLogger) Info(string)  {}
func (noopLogger) Warn(string)  {}
func (noopLogger) Error(string) {}

// TestSyncSourceInner_AggregatesPerCalendarFailures locks in the issue #278
// fix: when a per-calendar SyncCalendar fails, syncSourceInner must NOT return
// nil (which previously made SyncSource stamp a false clean last_synced_at and
// clear last_error). It returns an aggregated error naming the failure count
// and the failing calendars, which SyncSource then persists as last_error.
func TestSyncSourceInner_AggregatesPerCalendarFailures(t *testing.T) {
	// CalDAV server that fails every REPORT, so both calendars error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	store := newTestStore(t)
	now := time.Now().Unix()
	const srcID = "src-cd1"
	if err := store.WithTx(func(tx *sql.Tx) error {
		if err := store.CreateSourceTx(tx, Source{
			ID:        srcID,
			Type:      SourceTypeCalDAV,
			Name:      "iCloud",
			URL:       srv.URL,
			Username:  "u",
			Enabled:   true,
			Writable:  true,
			CreatedAt: now,
		}); err != nil {
			return err
		}
		if err := store.CreateCalendarTx(tx, Calendar{
			ID: "cal-a", SourceID: srcID, URL: srv.URL + "/a", DisplayName: "Personal", Visible: true, CreatedAt: now,
		}); err != nil {
			return err
		}
		return store.CreateCalendarTx(tx, Calendar{
			ID: "cal-b", SourceID: srcID, URL: srv.URL + "/b", DisplayName: "Work", Visible: true, CreatedAt: now,
		})
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	s := NewSyncer(store, fakeSecrets{password: "x"}, &recordingEventBus{}, nil, fakeAuth{target: ""}, nil, noopLogger{})

	err := s.syncSourceInner(context.Background(), srcID)
	if err == nil {
		t.Fatal("expected aggregated error; got nil (the #278 false-clean-sync regression)")
	}
	for _, want := range []string{"2 of 2 calendars failed", "Personal", "Work"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q is missing %q", err.Error(), want)
		}
	}
}

type privateSyncFailure struct{}

func (*privateSyncFailure) Error() string {
	return "untrusted certificate [certificate-check:test-reference]"
}
func (*privateSyncFailure) UserMessage() string { return "untrusted certificate" }

type certificateFailureTransport struct{}

func (certificateFailureTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, &privateSyncFailure{}
}

func TestSyncSourcePreservesPrivateCertificateReference(t *testing.T) {
	davutil.SetDefaultBaseTransport(certificateFailureTransport{})
	defer davutil.SetDefaultBaseTransport(nil)
	store := newTestStore(t)
	now := time.Now().Unix()
	if err := store.WithTx(func(tx *sql.Tx) error {
		if err := store.CreateSourceTx(tx, Source{ID: "private-source", Type: SourceTypeCalDAV, Name: "DAV", URL: "https://dav.example.org", Enabled: true, Writable: true, CreatedAt: now}); err != nil {
			return err
		}
		for _, id := range []string{"a", "b"} {
			if err := store.CreateCalendarTx(tx, Calendar{ID: id, SourceID: "private-source", URL: "/" + id, DisplayName: id, Visible: true, CreatedAt: now}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	bus := &recordingEventBus{}
	syncer := NewSyncer(store, fakeSecrets{password: "x"}, bus, nil, fakeAuth{target: ""}, nil, noopLogger{})
	err := syncer.SyncSource(context.Background(), "private-source")
	var private *privateSyncFailure
	if !errors.As(err, &private) {
		t.Fatalf("lost typed calendar failure: %v", err)
	}
	saved, err := store.GetSource("private-source")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(saved.LastError, "certificate-check:") {
		t.Fatalf("private reference persisted: %s", saved.LastError)
	}
	found := false
	for _, event := range bus.events() {
		if event.Name == "calendar:source-error" {
			found = true
			payload := event.Payload.(map[string]any)
			if strings.Contains(payload["message"].(string), "certificate-check:") {
				t.Fatal("private reference displayed")
			}
			if !strings.Contains(payload["certificateFailure"].(string), "certificate-check:") {
				t.Fatal("lookup reference lost")
			}
		}
	}
	if !found {
		t.Fatal("source-error event missing")
	}
}
