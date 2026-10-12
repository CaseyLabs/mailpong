// Package message provides message management functionality
package message

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/caseylabs/mailven/internal/logging"
)

// FTSIndexer handles background indexing of messages for full-text search
type FTSIndexer struct {
	db       *sql.DB
	mu       sync.Mutex
	indexing map[string]bool // folderID -> currently indexing

	// Callbacks for progress reporting
	onProgress func(folderID string, indexed, total int)
	onComplete func(folderID string)
}

// NewFTSIndexer creates a new FTS indexer
func NewFTSIndexer(db *sql.DB) *FTSIndexer {
	return &FTSIndexer{
		db:       db,
		indexing: make(map[string]bool),
	}
}

// SetProgressCallback sets the callback for progress updates
func (f *FTSIndexer) SetProgressCallback(cb func(folderID string, indexed, total int)) {
	f.onProgress = cb
}

// SetCompleteCallback sets the callback for indexing completion
func (f *FTSIndexer) SetCompleteCallback(cb func(folderID string)) {
	f.onComplete = cb
}

// IndexAllFolders indexes all folders across all accounts
// This should be called in the background after app startup
func (f *FTSIndexer) IndexAllFolders(ctx context.Context) error {
	logging.Info().Msg("Starting background FTS indexing for all folders")

	// Get all folder IDs
	rows, err := f.db.QueryContext(ctx, `SELECT id FROM folders`)
	if err != nil {
		return fmt.Errorf("failed to get folders: %w", err)
	}
	defer rows.Close()

	var folderIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("failed to scan folder ID: %w", err)
		}
		folderIDs = append(folderIDs, id)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating folders: %w", err)
	}

	logging.Info().Int("folderCount", len(folderIDs)).Msg("Found folders to index")

	// Index each folder
	for _, folderID := range folderIDs {
		select {
		case <-ctx.Done():
			logging.Info().Msg("FTS indexing cancelled")
			return ctx.Err()
		default:
		}

		if err := f.IndexFolder(ctx, folderID); err != nil {
			logging.Error().Err(err).Str("folderID", folderID).Msg("Failed to index folder")
			// Continue with other folders
		}
	}

	logging.Info().Msg("Background FTS indexing completed for all folders")
	return nil
}

// IndexFolder indexes all messages in a folder
// This is idempotent - it will skip already indexed messages
func (f *FTSIndexer) IndexFolder(ctx context.Context, folderID string) error {
	f.mu.Lock()
	if f.indexing[folderID] {
		f.mu.Unlock()
		return nil // Already indexing this folder
	}
	f.indexing[folderID] = true
	f.mu.Unlock()

	defer func() {
		f.mu.Lock()
		delete(f.indexing, folderID)
		f.mu.Unlock()
	}()

	// Check if folder is already fully indexed
	status, err := f.GetIndexStatus(folderID)
	if err != nil {
		return fmt.Errorf("failed to get index status: %w", err)
	}

	if status != nil && status.IsComplete {
		// Check if message count has changed
		var currentCount int
		err := f.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages WHERE folder_id = ?`, folderID).Scan(&currentCount)
		if err != nil {
			return fmt.Errorf("failed to count messages: %w", err)
		}

		if currentCount == status.TotalCount {
			logging.Debug().Str("folderID", folderID).Msg("Folder already fully indexed")
			return nil
		}

		// Message count changed, need to re-index
		logging.Info().Str("folderID", folderID).
			Int("previousCount", status.TotalCount).
			Int("currentCount", currentCount).
			Msg("Message count changed, re-indexing folder")
	}

	// Get total message count for this folder
	var totalCount int
	err = f.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages WHERE folder_id = ?`, folderID).Scan(&totalCount)
	if err != nil {
		return fmt.Errorf("failed to count messages: %w", err)
	}

	if totalCount == 0 {
		// Mark as complete even if empty
		if err := f.updateIndexStatus(ctx, folderID, 0, 0, true); err != nil {
			logging.Warn().Err(err).Str("folderID", folderID).Msg("Failed to update index status")
		}
		return nil
	}

	logging.Info().Str("folderID", folderID).Int("totalCount", totalCount).Msg("Starting FTS indexing for folder")

	// Initialize status
	if err := f.updateIndexStatus(ctx, folderID, 0, totalCount, false); err != nil {
		logging.Warn().Err(err).Str("folderID", folderID).Msg("Failed to initialize index status")
	}

	// Index in batches to avoid blocking
	const batchSize = 200
	indexed := 0

	for offset := 0; offset < totalCount; offset += batchSize {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Get batch of message rowids that need indexing
		rows, err := f.db.QueryContext(ctx, `
			SELECT m.rowid, m.subject, m.from_name, m.from_email, m.to_list, m.cc_list, m.snippet, m.body_text
			FROM messages m
			WHERE m.folder_id = ?
			ORDER BY m.rowid
			LIMIT ? OFFSET ?
		`, folderID, batchSize, offset)
		if err != nil {
			return fmt.Errorf("failed to get messages for indexing: %w", err)
		}

		// Insert into FTS table
		tx, err := f.db.BeginTx(ctx, nil)
		if err != nil {
			rows.Close()
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		batchCount := 0
		for rows.Next() {
			var rowid int64
			var subject, fromName, fromEmail, toList, ccList, snippet, bodyText sql.NullString

			if err := rows.Scan(&rowid, &subject, &fromName, &fromEmail, &toList, &ccList, &snippet, &bodyText); err != nil {
				rows.Close()
				_ = tx.Rollback()
				return fmt.Errorf("failed to scan message: %w", err)
			}

			// Check if already in FTS index (use INSERT OR REPLACE pattern)
			// First delete if exists, then insert
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO messages_fts(messages_fts, rowid, subject, from_name, from_email, to_list, cc_list, snippet, body_text)
				SELECT 'delete', ?, ?, ?, ?, ?, ?, ?, ?
				WHERE EXISTS (SELECT 1 FROM messages_fts WHERE rowid = ?)
			`, rowid, subject.String, fromName.String, fromEmail.String, toList.String, ccList.String, snippet.String, bodyText.String, rowid); err != nil {
				logging.Debug().Err(err).Int64("rowid", rowid).Msg("FTS pre-delete failed (row may not exist)")
			}

			_, err = tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO messages_fts(rowid, subject, from_name, from_email, to_list, cc_list, snippet, body_text)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			`, rowid, subject.String, fromName.String, fromEmail.String, toList.String, ccList.String, snippet.String, bodyText.String)
			if err != nil {
				rows.Close()
				_ = tx.Rollback()
				return fmt.Errorf("failed to insert into FTS: %w", err)
			}

			batchCount++
		}
		rows.Close()

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit FTS batch: %w", err)
		}

		indexed += batchCount

		// Update progress
		if err := f.updateIndexStatus(ctx, folderID, indexed, totalCount, false); err != nil {
			logging.Warn().Err(err).Str("folderID", folderID).Msg("Failed to update index status")
		}

		if f.onProgress != nil {
			f.onProgress(folderID, indexed, totalCount)
		}

		logging.Debug().
			Str("folderID", folderID).
			Int("indexed", indexed).
			Int("total", totalCount).
			Msg("FTS indexing progress")

		// Small delay to avoid blocking other operations
		time.Sleep(10 * time.Millisecond)
	}

	// Mark as complete
	if err := f.updateIndexStatus(ctx, folderID, indexed, totalCount, true); err != nil {
		logging.Warn().Err(err).Str("folderID", folderID).Msg("Failed to mark index as complete")
	}

	if f.onComplete != nil {
		f.onComplete(folderID)
	}

	logging.Info().Str("folderID", folderID).Int("indexed", indexed).Msg("FTS indexing completed for folder")
	return nil
}

// GetIndexStatus returns the indexing status for a folder
func (f *FTSIndexer) GetIndexStatus(folderID string) (*FTSIndexStatus, error) {
	var status FTSIndexStatus
	var lastIndexedAt sql.NullString

	err := f.db.QueryRow(`
		SELECT folder_id, indexed_count, total_count, is_complete, last_indexed_at
		FROM fts_index_status
		WHERE folder_id = ?
	`, folderID).Scan(&status.FolderID, &status.IndexedCount, &status.TotalCount, &status.IsComplete, &lastIndexedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get index status: %w", err)
	}

	if lastIndexedAt.Valid {
		status.LastIndexedAt = lastIndexedAt.String
	}

	return &status, nil
}

// GetAllIndexStatuses returns indexing status for all folders
func (f *FTSIndexer) GetAllIndexStatuses() (map[string]*FTSIndexStatus, error) {
	rows, err := f.db.Query(`
		SELECT folder_id, indexed_count, total_count, is_complete, last_indexed_at
		FROM fts_index_status
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get index statuses: %w", err)
	}
	defer rows.Close()

	statuses := make(map[string]*FTSIndexStatus)
	for rows.Next() {
		var status FTSIndexStatus
		var lastIndexedAt sql.NullString

		if err := rows.Scan(&status.FolderID, &status.IndexedCount, &status.TotalCount, &status.IsComplete, &lastIndexedAt); err != nil {
			return nil, fmt.Errorf("failed to scan index status: %w", err)
		}

		if lastIndexedAt.Valid {
			status.LastIndexedAt = lastIndexedAt.String
		}

		statuses[status.FolderID] = &status
	}

	return statuses, rows.Err()
}

// IsIndexComplete checks if a folder is fully indexed
func (f *FTSIndexer) IsIndexComplete(folderID string) bool {
	status, err := f.GetIndexStatus(folderID)
	if err != nil || status == nil {
		return false
	}
	return status.IsComplete
}

// IsAnyIndexing returns true if any folder is currently being indexed
func (f *FTSIndexer) IsAnyIndexing() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.indexing) > 0
}

// GetIndexingFolders returns the list of folders currently being indexed
func (f *FTSIndexer) GetIndexingFolders() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	folders := make([]string, 0, len(f.indexing))
	for folderID := range f.indexing {
		folders = append(folders, folderID)
	}
	return folders
}

// updateIndexStatus updates the indexing status in the database
func (f *FTSIndexer) updateIndexStatus(ctx context.Context, folderID string, indexed, total int, complete bool) error {
	_, err := f.db.ExecContext(ctx, `
		INSERT INTO fts_index_status (folder_id, indexed_count, total_count, is_complete, last_indexed_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(folder_id) DO UPDATE SET
			indexed_count = excluded.indexed_count,
			total_count = excluded.total_count,
			is_complete = excluded.is_complete,
			last_indexed_at = excluded.last_indexed_at
	`, folderID, indexed, total, complete)
	return err
}

// RebuildIndex forces a complete rebuild of the FTS index for a folder
func (f *FTSIndexer) RebuildIndex(ctx context.Context, folderID string) error {
	logging.Info().Str("folderID", folderID).Msg("Rebuilding FTS index for folder")

	// Clear existing index status to force re-index
	_, err := f.db.ExecContext(ctx, `DELETE FROM fts_index_status WHERE folder_id = ?`, folderID)
	if err != nil {
		return fmt.Errorf("failed to clear index status: %w", err)
	}

	// Re-index the folder
	return f.IndexFolder(ctx, folderID)
}

// RebuildAllIndexes forces a complete rebuild of all FTS indexes
func (f *FTSIndexer) RebuildAllIndexes(ctx context.Context) error {
	logging.Info().Msg("Rebuilding all FTS indexes")

	// Clear all index statuses
	_, err := f.db.ExecContext(ctx, `DELETE FROM fts_index_status`)
	if err != nil {
		return fmt.Errorf("failed to clear all index statuses: %w", err)
	}

	// Clear the FTS table using the special 'delete-all' command
	_, err = f.db.ExecContext(ctx, `INSERT INTO messages_fts(messages_fts) VALUES('delete-all')`)
	if err != nil {
		// Table might not exist yet or be empty, ignore
		logging.Warn().Err(err).Msg("Failed to clear FTS table (may be expected if table is new)")
	}

	// Re-index all folders
	return f.IndexAllFolders(ctx)
}
