// Package contactdata adapts host-owned contact stores to the extension SDK.
// It never opens or writes an extension's per-extension database.
package contactdata

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/caseylabs/mailven/internal/carddav"
	"github.com/caseylabs/mailven/internal/contact"
	coreapi "github.com/caseylabs/mailven/internal/core/api/v1"
)

type Service struct {
	records      *contact.Store
	dav          *carddav.Store
	db           *sql.DB
	sourceClient func(string, []coreapi.AuthScope) (*http.Client, error)
	davClient    func(*carddav.Source) (*carddav.Client, error)
}

func New(records *contact.Store, dav *carddav.Store, db *sql.DB,
	sourceClient func(string, []coreapi.AuthScope) (*http.Client, error),
	davClient func(*carddav.Source) (*carddav.Client, error)) *Service {
	return &Service{records: records, dav: dav, db: db, sourceClient: sourceClient, davClient: davClient}
}

func (s *Service) Records() coreapi.ContactRecords {
	if s.records == nil {
		return nil
	}
	return records{Store: s.records}
}
func (s *Service) DAV() coreapi.ContactDAV {
	if s.dav == nil {
		return nil
	}
	return dav{Store: s.dav, service: s}
}

type records struct{ *contact.Store }

func (s records) Search(query string, limit int) ([]*coreapi.ContactMatch, error) {
	rows, err := s.Store.Search(query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]*coreapi.ContactMatch, 0, len(rows))
	for _, row := range rows {
		out = append(out, &coreapi.ContactMatch{Email: row.Email, DisplayName: row.DisplayName, Source: row.Source, LastUsed: row.LastUsed, CreatedAt: row.CreatedAt})
	}
	return out, nil
}

type dav struct {
	*carddav.Store
	service *Service
}

func (s dav) clientForAddressbook(id string) (*carddav.Client, error) {
	source, err := s.Store.GetSourceForAddressbook(id)
	if err != nil {
		return nil, err
	}
	if source == nil {
		return nil, fmt.Errorf("no source owns addressbook %s", id)
	}
	if !source.Writable {
		return nil, fmt.Errorf("this source is not writable; enable write access in its settings")
	}
	if s.service.davClient == nil {
		return nil, fmt.Errorf("credentials lookup unavailable")
	}
	return s.service.davClient(source)
}
func (s dav) CreateRecord(id string, rec *coreapi.ContactRecord) (string, error) {
	client, err := s.clientForAddressbook(id)
	if err != nil {
		return "", err
	}
	created, err := s.Store.CreateRecord(id, rec, client)
	var pre *carddav.ErrPreconditionFailed
	if errors.As(err, &pre) {
		return "", &coreapi.ErrConflict{Message: "a contact already exists at the generated href; please retry"}
	}
	return created, err
}
func (s dav) UpdateRecord(rec *coreapi.ContactRecord) error {
	client, err := s.clientForAddressbook(rec.SourceRef)
	if err != nil {
		return err
	}
	err = s.Store.UpdateRecord(rec, client)
	return s.conflict(rec.ID, client, err)
}
func (s dav) DeleteRecord(id string) error {
	rec, err := s.service.records.GetRecord(id)
	if err != nil {
		return err
	}
	if rec == nil {
		return fmt.Errorf("contact %s not found", id)
	}
	client, err := s.clientForAddressbook(rec.SourceRef)
	if err != nil {
		return err
	}
	return s.conflict(id, client, s.Store.DeleteRecord(id, client))
}
func (s dav) RefreshRecordFromServer(id string) error {
	rec, err := s.service.records.GetRecord(id)
	if err != nil {
		return err
	}
	if rec == nil {
		return fmt.Errorf("contact %s not found", id)
	}
	client, err := s.clientForAddressbook(rec.SourceRef)
	if err != nil {
		return err
	}
	return s.Store.RefreshRecordFromServer(id, client)
}
func (s dav) conflict(id string, client *carddav.Client, err error) error {
	var pre *carddav.ErrPreconditionFailed
	if errors.As(err, &pre) {
		_ = s.Store.RefreshRecordFromServer(id, client)
		return &coreapi.ErrConflict{ContactID: id, Message: "the contact was modified on the server"}
	}
	return err
}

func (s *Service) SaveProviderRecord(rec *coreapi.ContactRecord, addressbookID, href string, create bool) error {
	if s.db == nil {
		return fmt.Errorf("contact mirror database unavailable")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = contact.UpsertRecordTx(tx, rec); err != nil {
		return err
	}
	if create {
		_, err = tx.Exec(`INSERT INTO carddav_record_state (record_id,addressbook_id,href,etag,synced_at) VALUES (?,?,?,?,?)`, rec.ID, addressbookID, href, "", time.Now())
	} else {
		_, err = tx.Exec(`UPDATE carddav_record_state SET synced_at=? WHERE record_id=?`, time.Now(), rec.ID)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Service) ProviderRecordHref(id string) (string, error) {
	var href string
	if s.db == nil {
		return "", fmt.Errorf("contact mirror database unavailable")
	}
	err := s.db.QueryRow(`SELECT href FROM carddav_record_state WHERE record_id=?`, id).Scan(&href)
	return href, err
}
func (s *Service) SourceHTTPClient(id string, scopes []coreapi.AuthScope) (*http.Client, error) {
	if s.sourceClient == nil {
		return nil, fmt.Errorf("source authentication unavailable")
	}
	return s.sourceClient(id, scopes)
}

var _ coreapi.ContactData = (*Service)(nil)

func (s dav) GetSource(id string) (*coreapi.ContactDAVSource, error) {
	src, err := s.Store.GetSource(id)
	return sourceModel(src), err
}
func (s dav) GetSourceForAddressbook(id string) (*coreapi.ContactDAVSource, error) {
	src, err := s.Store.GetSourceForAddressbook(id)
	return sourceModel(src), err
}
func (s dav) GetAddressbook(id string) (*coreapi.ContactDAVAddressbook, error) {
	ab, err := s.Store.GetAddressbook(id)
	return addressbookModel(ab), err
}
func (s dav) ListAddressbooks(id string) ([]*coreapi.ContactDAVAddressbook, error) {
	abs, err := s.Store.ListAddressbooks(id)
	if err != nil {
		return nil, err
	}
	out := make([]*coreapi.ContactDAVAddressbook, 0, len(abs))
	for _, ab := range abs {
		out = append(out, addressbookModel(ab))
	}
	return out, nil
}
func sourceModel(src *carddav.Source) *coreapi.ContactDAVSource {
	if src == nil {
		return nil
	}
	return &coreapi.ContactDAVSource{ID: src.ID, Name: src.Name, Type: coreapi.ContactSourceType(src.Type), URL: src.URL, Username: src.Username, AccountID: src.AccountID, Enabled: src.Enabled, Writable: src.Writable, SyncInterval: src.SyncInterval, LastSyncedAt: src.LastSyncedAt, LastError: src.LastError, LastErrorAt: src.LastErrorAt, CreatedAt: src.CreatedAt}
}
func addressbookModel(ab *carddav.Addressbook) *coreapi.ContactDAVAddressbook {
	if ab == nil {
		return nil
	}
	return &coreapi.ContactDAVAddressbook{ID: ab.ID, SourceID: ab.SourceID, Path: ab.Path, Name: ab.Name, Enabled: ab.Enabled, SyncToken: ab.SyncToken, LastSyncedAt: ab.LastSyncedAt}
}
