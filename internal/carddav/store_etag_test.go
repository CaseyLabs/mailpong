package carddav

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/caseylabs/mailven/internal/contact"
)

// etaglessServer accepts PUT and DELETE without returning an ETag and
// answers any multiget with /ab/r1.vcf at etag "e1", recording each
// write's If-Match.
type etaglessServer struct {
	mu     sync.Mutex
	writes []string // "METHOD If-Match"
}

func (e *etaglessServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPut, http.MethodDelete:
		e.mu.Lock()
		e.writes = append(e.writes, r.Method+" "+r.Header.Get("If-Match"))
		e.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	case "REPORT":
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusMultiStatus)
		fmt.Fprint(w, `<?xml version="1.0"?><d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:carddav">
<d:response><d:href>/ab/r1.vcf</d:href><d:propstat><d:prop><d:getetag>"e1"</d:getetag>
<c:address-data>BEGIN:VCARD&#13;
VERSION:3.0&#13;
UID:r1&#13;
FN:Jane&#13;
END:VCARD&#13;
</c:address-data></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response></d:multistatus>`)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (e *etaglessServer) takeWrites() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	w := e.writes
	e.writes = nil
	return w
}

func TestCardDAVWritesNeverUnconditional(t *testing.T) {
	db := openCardDAVTestDB(t)
	s := NewStore(db.DB)
	srv := &etaglessServer{}
	ts := httptest.NewServer(srv)
	defer ts.Close()
	client := newTestClient(t, ts.URL)

	src, err := s.CreateSource(&SourceConfig{Name: "T", Type: SourceTypeCardDAV, URL: ts.URL, Enabled: true, SyncInterval: 60})
	if err != nil {
		t.Fatal(err)
	}
	ab, err := s.CreateAddressbook(src.ID, "/ab/", "ab", true)
	if err != nil {
		t.Fatal(err)
	}

	rec := &contact.Record{ID: "r1", Fn: "Jane"}
	if _, err := s.CreateRecord(ab.ID, rec, client); err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}
	if got := srv.takeWrites(); len(got) != 1 || got[0] != "PUT " {
		t.Fatalf("create writes = %q", got)
	}

	// The etag missing from the PUT response was fetched and stored, so
	// the update is conditional.
	rec.Fn = "Janet"
	if err := s.UpdateRecord(rec, client); err != nil {
		t.Fatalf("UpdateRecord: %v", err)
	}
	if got := srv.takeWrites(); len(got) != 1 || got[0] != `PUT "e1"` {
		t.Fatalf("update writes = %q, want conditional PUT", got)
	}

	// With no stored etag, update and delete refuse instead of writing
	// unconditionally.
	if _, err := db.Exec(`UPDATE carddav_record_state SET etag = '' WHERE record_id = ?`, rec.ID); err != nil {
		t.Fatal(err)
	}
	var pre *ErrPreconditionFailed
	if err := s.UpdateRecord(rec, client); !errors.As(err, &pre) {
		t.Errorf("UpdateRecord with no etag = %v, want ErrPreconditionFailed", err)
	}
	if err := s.DeleteRecord(rec.ID, client); !errors.As(err, &pre) {
		t.Errorf("DeleteRecord with no etag = %v, want ErrPreconditionFailed", err)
	}
	if got := srv.takeWrites(); len(got) != 0 {
		t.Errorf("writes with no etag = %q, want none", got)
	}
}
