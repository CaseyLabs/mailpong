package v1

import (
	"errors"
	"net/http"
)

var ErrContactExists = errors.New("contact already exists")

// ContactRecords exposes host-owned mirror rows, never an extension database.
type ContactRecords interface {
	Search(query string, limit int) ([]*ContactMatch, error)
	GetRecord(id string) (*ContactRecord, error)
	GetRecordByEmail(email string) (*ContactRecord, error)
	ListRecords(filter ContactRecordFilter) ([]*ContactRecord, error)
	Create(email, name string) error
	UpsertRecord(record *ContactRecord) error
	DeleteRecord(id string) error
}

// ContactDAV exposes host-owned source metadata and DAV mirror operations.
// The host authenticates requests; extension code never receives credentials.
type ContactDAV interface {
	GetSource(id string) (*ContactDAVSource, error)
	GetSourceForAddressbook(id string) (*ContactDAVSource, error)
	GetSourceIDForRecord(id string) (string, error)
	GetAddressbook(id string) (*ContactDAVAddressbook, error)
	ListAddressbooks(sourceID string) ([]*ContactDAVAddressbook, error)
	ListRecordIDsForSource(sourceID, query string, offset, limit int) ([]string, error)
	CreateRecord(addressbookID string, record *ContactRecord) (string, error)
	UpdateRecord(record *ContactRecord) error
	DeleteRecord(id string) error
	RefreshRecordFromServer(id string) error
}

// ContactData gives Contacts access to shared host data through operations,
// keeping atomic provider mirror updates and source authentication in the host.
// Provider ETags and field sidecars remain extension-owned.
type ContactData interface {
	Records() ContactRecords
	DAV() ContactDAV
	SaveProviderRecord(record *ContactRecord, addressbookID, href string, create bool) error
	ProviderRecordHref(recordID string) (string, error)
	SourceHTTPClient(sourceID string, scopes []AuthScope) (*http.Client, error)
}
