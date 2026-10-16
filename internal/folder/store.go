package folder

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/caseylabs/mailven/internal/database"
	"github.com/caseylabs/mailven/internal/logging"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Store provides folder persistence operations
type Store struct {
	db  *database.DB
	log zerolog.Logger
}

// NewStore creates a new folder store
func NewStore(db *database.DB) *Store {
	return &Store{
		db:  db,
		log: logging.WithComponent("folder-store"),
	}
}

// List returns all folders for an account
func (s *Store) List(accountID string) ([]*Folder, error) {
	return s.list(`account_id = ? AND folder_type NOT LIKE 'hidden:%'`, accountID)
}

// ListHidden returns the account's folders hidden as the `rows` set (RowsIMAP
// or RowsAPI) with their original type and path, as a swap back would restore
// them. An IMAP account on an active API route keeps its folder mappings in
// these rows.
func (s *Store) ListHidden(accountID, rows string) ([]*Folder, error) {
	typePrefix := hiddenPrefix + rows + ":"
	pathPrefix := pathMarker + rows + pathMarker
	folders, err := s.list(`account_id = ? AND substr(folder_type, 1, ?) = ? AND substr(path, 1, ?) = ?`,
		accountID, len(typePrefix), typePrefix, len(pathPrefix), pathPrefix)
	if err != nil {
		return nil, err
	}
	for _, f := range folders {
		f.Type = Type(strings.TrimPrefix(string(f.Type), typePrefix))
		f.Path = strings.TrimPrefix(f.Path, pathPrefix)
	}
	return folders, nil
}

// list returns the folders matching where, ordered by name.
func (s *Store) list(where string, args ...any) ([]*Folder, error) {
	query := `
		SELECT id, account_id, name, path, folder_type, parent_id,
		       uid_validity, uid_next, highest_mod_seq,
		       total_count, unread_count, last_sync, subscribed
		FROM folders
		WHERE ` + where + `
		ORDER BY name
	`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query folders: %w", err)
	}
	defer rows.Close()

	var folders []*Folder
	for rows.Next() {
		f := &Folder{}
		var parentID sql.NullString
		var lastSync sql.NullTime
		var uidValidity, uidNext sql.NullInt64
		var highestModSeq sql.NullInt64

		err := rows.Scan(
			&f.ID, &f.AccountID, &f.Name, &f.Path, &f.Type, &parentID,
			&uidValidity, &uidNext, &highestModSeq,
			&f.TotalCount, &f.UnreadCount, &lastSync, &f.Subscribed,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan folder: %w", err)
		}

		if parentID.Valid {
			f.ParentID = parentID.String
		}
		if lastSync.Valid {
			f.LastSync = &lastSync.Time
		}
		if uidValidity.Valid {
			f.UIDValidity = uint32(uidValidity.Int64)
		}
		if uidNext.Valid {
			f.UIDNext = uint32(uidNext.Int64)
		}
		if highestModSeq.Valid {
			f.HighestModSeq = uint64(highestModSeq.Int64)
		}

		folders = append(folders, f)
	}

	return folders, nil
}

// Get returns a folder by ID
func (s *Store) Get(id string) (*Folder, error) {
	query := `
		SELECT id, account_id, name, path, folder_type, parent_id,
		       uid_validity, uid_next, highest_mod_seq,
		       total_count, unread_count, last_sync, subscribed
		FROM folders
		WHERE id = ?
	`

	f := &Folder{}
	var parentID sql.NullString
	var lastSync sql.NullTime
	var uidValidity, uidNext sql.NullInt64
	var highestModSeq sql.NullInt64

	err := s.db.QueryRow(query, id).Scan(
		&f.ID, &f.AccountID, &f.Name, &f.Path, &f.Type, &parentID,
		&uidValidity, &uidNext, &highestModSeq,
		&f.TotalCount, &f.UnreadCount, &lastSync, &f.Subscribed,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get folder: %w", err)
	}

	if parentID.Valid {
		f.ParentID = parentID.String
	}
	if lastSync.Valid {
		f.LastSync = &lastSync.Time
	}
	if uidValidity.Valid {
		f.UIDValidity = uint32(uidValidity.Int64)
	}
	if uidNext.Valid {
		f.UIDNext = uint32(uidNext.Int64)
	}
	if highestModSeq.Valid {
		f.HighestModSeq = uint64(highestModSeq.Int64)
	}

	return f, nil
}

// GetByPath returns a folder by account ID and path
func (s *Store) GetByPath(accountID, path string) (*Folder, error) {
	query := `
		SELECT id, account_id, name, path, folder_type, parent_id,
		       uid_validity, uid_next, highest_mod_seq,
		       total_count, unread_count, last_sync, subscribed
		FROM folders
		WHERE account_id = ? AND path = ? AND folder_type NOT LIKE 'hidden:%'
	`

	f := &Folder{}
	var parentID sql.NullString
	var lastSync sql.NullTime
	var uidValidity, uidNext sql.NullInt64
	var highestModSeq sql.NullInt64

	err := s.db.QueryRow(query, accountID, path).Scan(
		&f.ID, &f.AccountID, &f.Name, &f.Path, &f.Type, &parentID,
		&uidValidity, &uidNext, &highestModSeq,
		&f.TotalCount, &f.UnreadCount, &lastSync, &f.Subscribed,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get folder: %w", err)
	}

	if parentID.Valid {
		f.ParentID = parentID.String
	}
	if lastSync.Valid {
		f.LastSync = &lastSync.Time
	}
	if uidValidity.Valid {
		f.UIDValidity = uint32(uidValidity.Int64)
	}
	if uidNext.Valid {
		f.UIDNext = uint32(uidNext.Int64)
	}
	if highestModSeq.Valid {
		f.HighestModSeq = uint64(highestModSeq.Int64)
	}

	return f, nil
}

// Create creates a new folder
func (s *Store) Create(f *Folder) error {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}

	query := `
		INSERT INTO folders (id, account_id, name, path, folder_type, parent_id,
		                     uid_validity, uid_next, highest_mod_seq,
		                     total_count, unread_count, last_sync, subscribed)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var parentID interface{}
	if f.ParentID != "" {
		parentID = f.ParentID
	}

	var lastSync interface{}
	if f.LastSync != nil {
		lastSync = f.LastSync
	}

	_, err := s.db.Exec(query,
		f.ID, f.AccountID, f.Name, f.Path, f.Type, parentID,
		f.UIDValidity, f.UIDNext, f.HighestModSeq,
		f.TotalCount, f.UnreadCount, lastSync, f.Subscribed,
	)
	if err != nil {
		return fmt.Errorf("failed to create folder: %w", err)
	}

	s.log.Debug().
		Str("id", f.ID).
		Str("path", f.Path).
		Msg("Created folder")

	return nil
}

// Update updates an existing folder
func (s *Store) Update(f *Folder) error {
	query := `
		UPDATE folders SET
			name = ?,
			folder_type = ?,
			parent_id = ?,
			uid_validity = ?,
			uid_next = ?,
			highest_mod_seq = ?,
			total_count = ?,
			unread_count = ?,
			last_sync = ?,
			subscribed = ?
		WHERE id = ?
	`

	var parentID interface{}
	if f.ParentID != "" {
		parentID = f.ParentID
	}

	var lastSync interface{}
	if f.LastSync != nil {
		lastSync = f.LastSync
	}

	_, err := s.db.Exec(query,
		f.Name, f.Type, parentID,
		f.UIDValidity, f.UIDNext, f.HighestModSeq,
		f.TotalCount, f.UnreadCount, lastSync, f.Subscribed,
		f.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update folder: %w", err)
	}

	return nil
}

// UpdateSyncState updates only the sync-related fields
func (s *Store) UpdateSyncState(id string, uidValidity, uidNext uint32, highestModSeq uint64, totalCount, unreadCount int) error {
	now := time.Now()
	query := `
		UPDATE folders SET
			uid_validity = ?,
			uid_next = ?,
			highest_mod_seq = ?,
			total_count = ?,
			unread_count = ?,
			last_sync = ?
		WHERE id = ?
	`

	_, err := s.db.Exec(query, uidValidity, uidNext, highestModSeq, totalCount, unreadCount, now, id)
	if err != nil {
		return fmt.Errorf("failed to update sync state: %w", err)
	}

	return nil
}

// UpdateCounts updates only the message counts
func (s *Store) UpdateCounts(id string, totalCount, unreadCount int) error {
	query := `UPDATE folders SET total_count = ?, unread_count = ? WHERE id = ?`
	_, err := s.db.Exec(query, totalCount, unreadCount, id)
	if err != nil {
		return fmt.Errorf("failed to update counts: %w", err)
	}
	return nil
}

// Delete deletes a folder
func (s *Store) Delete(id string) error {
	_, err := s.db.Exec("DELETE FROM folders WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete folder: %w", err)
	}
	return nil
}

// DeleteByAccount deletes all folders for an account
func (s *Store) DeleteByAccount(accountID string) error {
	_, err := s.db.Exec("DELETE FROM folders WHERE account_id = ?", accountID)
	if err != nil {
		return fmt.Errorf("failed to delete folders: %w", err)
	}
	return nil
}

// Upsert creates or updates a folder based on account_id and path
func (s *Store) Upsert(f *Folder) error {
	existing, err := s.GetByPath(f.AccountID, f.Path)
	if err != nil {
		return err
	}

	if existing != nil {
		// Update existing
		f.ID = existing.ID
		return s.Update(f)
	}

	// Create new
	return s.Create(f)
}

// GetByType returns a folder by account ID and folder type
func (s *Store) GetByType(accountID string, folderType Type) (*Folder, error) {
	query := `
		SELECT id, account_id, name, path, folder_type, parent_id,
		       uid_validity, uid_next, highest_mod_seq,
		       total_count, unread_count, last_sync, subscribed
		FROM folders
		WHERE account_id = ? AND folder_type = ?
		LIMIT 1
	`

	f := &Folder{}
	var parentID sql.NullString
	var lastSync sql.NullTime
	var uidValidity, uidNext sql.NullInt64
	var highestModSeq sql.NullInt64

	err := s.db.QueryRow(query, accountID, folderType).Scan(
		&f.ID, &f.AccountID, &f.Name, &f.Path, &f.Type, &parentID,
		&uidValidity, &uidNext, &highestModSeq,
		&f.TotalCount, &f.UnreadCount, &lastSync, &f.Subscribed,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get folder by type: %w", err)
	}

	if parentID.Valid {
		f.ParentID = parentID.String
	}
	if lastSync.Valid {
		f.LastSync = &lastSync.Time
	}
	if uidValidity.Valid {
		f.UIDValidity = uint32(uidValidity.Int64)
	}
	if uidNext.Valid {
		f.UIDNext = uint32(uidNext.Int64)
	}
	if highestModSeq.Valid {
		f.HighestModSeq = uint64(highestModSeq.Int64)
	}

	return f, nil
}

// ListIDsByType returns the IDs of every account's folders of folderType.
func (s *Store) ListIDsByType(folderType Type) ([]string, error) {
	rows, err := s.db.Query(`SELECT id FROM folders WHERE folder_type = ?`, folderType)
	if err != nil {
		return nil, fmt.Errorf("failed to list folders by type: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan folder id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListSubscribed returns only subscribed folders for an account.
// Core folders (Inbox, Drafts, Sent) are always included regardless of subscription state.
func (s *Store) ListSubscribed(accountID string) ([]*Folder, error) {
	query := `
		SELECT id, account_id, name, path, folder_type, parent_id,
		       uid_validity, uid_next, highest_mod_seq,
		       total_count, unread_count, last_sync, subscribed
		FROM folders
		WHERE account_id = ? AND folder_type NOT LIKE 'hidden:%' AND (subscribed = 1 OR folder_type IN ('inbox', 'drafts', 'sent'))
		ORDER BY name
	`

	rows, err := s.db.Query(query, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to query subscribed folders: %w", err)
	}
	defer rows.Close()

	var folders []*Folder
	for rows.Next() {
		f := &Folder{}
		var parentID sql.NullString
		var lastSync sql.NullTime
		var uidValidity, uidNext sql.NullInt64
		var highestModSeq sql.NullInt64

		err := rows.Scan(
			&f.ID, &f.AccountID, &f.Name, &f.Path, &f.Type, &parentID,
			&uidValidity, &uidNext, &highestModSeq,
			&f.TotalCount, &f.UnreadCount, &lastSync, &f.Subscribed,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan folder: %w", err)
		}

		if parentID.Valid {
			f.ParentID = parentID.String
		}
		if lastSync.Valid {
			f.LastSync = &lastSync.Time
		}
		if uidValidity.Valid {
			f.UIDValidity = uint32(uidValidity.Int64)
		}
		if uidNext.Valid {
			f.UIDNext = uint32(uidNext.Int64)
		}
		if highestModSeq.Valid {
			f.HighestModSeq = uint64(highestModSeq.Int64)
		}

		folders = append(folders, f)
	}

	return folders, nil
}

// UpdateSubscribed updates the IMAP subscription state for a folder.
func (s *Store) UpdateSubscribed(folderID string, subscribed bool) error {
	_, err := s.db.Exec(`UPDATE folders SET subscribed = ? WHERE id = ?`, subscribed, folderID)
	if err != nil {
		return fmt.Errorf("failed to update folder subscription: %w", err)
	}
	return nil
}

// Row sets that can own an account's folders. Only one is visible at a time.
const (
	RowsIMAP = "imap"
	RowsAPI  = "api"

	hiddenPrefix = "hidden:"
	pathMarker   = "\x1f"
)

// SwapHidden hides every visible folder of the account and then restores the
// folders previously hidden as the `show` row set, in one transaction. Hidden
// folders keep their messages but vanish from List, ListSubscribed, GetByPath
// and the type lookups, so IMAP and API rows are never mixed. Nothing is
// deleted; swapping back restores the original type and path exactly. `hide`
// tags the folders being hidden (RowsIMAP or RowsAPI). It returns the number
// of folders restored.
func (s *Store) SwapHidden(accountID, hide, show string) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin folder swap: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Hide visible folders: type becomes hidden:<tag>:<type>, path gets a
	// tag-specific prefix so a restored set cannot collide with a new one.
	if _, err := tx.Exec(`UPDATE folders
		SET folder_type = ? || ? || ':' || folder_type, path = ? || ? || ? || path
		WHERE account_id = ? AND folder_type NOT LIKE 'hidden:%'`,
		hiddenPrefix, hide, pathMarker, hide, pathMarker, accountID); err != nil {
		return 0, fmt.Errorf("failed to hide folders: %w", err)
	}
	typePrefix := hiddenPrefix + show + ":"
	pathPrefix := pathMarker + show + pathMarker
	res, err := tx.Exec(`UPDATE folders
		SET folder_type = substr(folder_type, ?), path = substr(path, ?)
		WHERE account_id = ? AND substr(folder_type, 1, ?) = ? AND substr(path, 1, ?) = ?`,
		len(typePrefix)+1, len(pathPrefix)+1, accountID,
		len(typePrefix), typePrefix, len(pathPrefix), pathPrefix)
	if err != nil {
		return 0, fmt.Errorf("failed to restore folders: %w", err)
	}
	n, _ := res.RowsAffected()
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit folder swap: %w", err)
	}
	return int(n), nil
}

// TouchSync records a completed sync without touching UID or count state.
func (s *Store) TouchSync(id string) error {
	if _, err := s.db.Exec(`UPDATE folders SET last_sync = ? WHERE id = ?`, time.Now().UTC(), id); err != nil {
		return fmt.Errorf("failed to touch folder sync: %w", err)
	}
	return nil
}
