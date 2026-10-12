package carddav

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/caseylabs/mailven/internal/certificate"
	"github.com/caseylabs/mailven/internal/database"
	"github.com/caseylabs/mailven/internal/kit/davutil"
)

type testPasswords struct{}

func (testPasswords) GetPassword(string) (string, error) { return "pass", nil }

func TestBackgroundAddressbookCertificateFailure(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "host.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Migrate(); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()
	certs := certificate.NewStore(db.DB)
	transport := certificate.NewTransport(certs)
	defer transport.CloseIdleConnections()
	davutil.SetDefaultBaseTransport(transport)
	defer davutil.SetDefaultBaseTransport(nil)
	store := NewStore(db.DB)
	source, err := store.CreateSource(&SourceConfig{Name: "Test", Type: SourceTypeCardDAV, URL: srv.URL, Username: "user", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/one/", "/two/"} {
		if _, err = store.CreateAddressbook(source.ID, path, path, true); err != nil {
			t.Fatal(err)
		}
	}
	syncer := NewSyncer(store, testPasswords{})
	var reported error
	syncer.SetSyncErrorHandler(func(id string, err error) {
		if id != source.ID {
			t.Errorf("source=%s", id)
		}
		reported = err
	})
	err = syncer.SyncSource(source.ID)
	var failure *certificate.Error
	if !errors.As(err, &failure) || !errors.As(reported, &failure) || failure.Host != "127.0.0.1" {
		t.Fatalf("error=%v,reported=%v", err, reported)
	}
	saved, err := store.GetSource(source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(saved.LastError, "certificate-check:") {
		t.Fatalf("internal reference persisted: %s", saved.LastError)
	}
}
