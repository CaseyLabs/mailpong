package contactdata

import (
	"path/filepath"
	"testing"

	"github.com/caseylabs/mailven/internal/carddav"
	"github.com/caseylabs/mailven/internal/contact"
	coreapi "github.com/caseylabs/mailven/internal/core/api/v1"
	"github.com/caseylabs/mailven/internal/database"
)

func TestProviderMirrorAtomicity(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "host.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Migrate(); err != nil {
		t.Fatal(err)
	}
	local := contact.NewStore(db.DB)
	sources := carddav.NewStore(db.DB)
	source, err := sources.CreateSource(&carddav.SourceConfig{Name: "Test", Type: carddav.SourceTypeGoogle, Enabled: true, Writable: true})
	if err != nil {
		t.Fatal(err)
	}
	ab, err := sources.CreateAddressbook(source.ID, "people", "Contacts", true)
	if err != nil {
		t.Fatal(err)
	}
	data := New(local, sources, db.DB, nil, nil)
	rec := &coreapi.ContactRecord{ID: "provider-record", Fn: "Before", Source: "carddav", SourceRef: ab.ID}
	if err = data.SaveProviderRecord(rec, "missing", "people/1", true); err == nil {
		t.Fatal("missing addressbook accepted")
	}
	if got, _ := local.GetRecord(rec.ID); got != nil {
		t.Fatal("record persisted despite failed mirror transaction")
	}
	if err = data.SaveProviderRecord(rec, ab.ID, "people/1", true); err != nil {
		t.Fatal(err)
	}
	rec.Fn = "After"
	if err = data.SaveProviderRecord(rec, ab.ID, "", false); err != nil {
		t.Fatal(err)
	}
	if href, err := data.ProviderRecordHref(rec.ID); err != nil || href != "people/1" {
		t.Fatalf("href=%q,err=%v", href, err)
	}
	if got, err := local.GetRecord(rec.ID); err != nil || got == nil || got.Fn != "After" {
		t.Fatalf("record=%+v,err=%v", got, err)
	}
	if err = data.Records().DeleteRecord(rec.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = data.ProviderRecordHref(rec.ID); err == nil {
		t.Fatal("provider state survived record delete")
	}
}
