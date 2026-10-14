package credentials

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// refreshKey identifies one account's tokens under one client config.
type refreshKey struct{ accountID, clientConfigID string }

// LockOAuthRefresh serializes token refreshes for one owner/client-config slot
// across callers, Store instances, and processes. Its advisory lock file is
// persistent: unlinking it could let concurrent callers lock different
// inodes. Callers must re-read tokens after locking and skip refresh when
// another holder already rotated them. A lock setup failure is returned so
// callers can fail closed.
func (s *Store) LockOAuthRefresh(accountID, clientConfigID string) (unlock func(), err error) {
	ownerID, err := s.OAuthTokenOwnerID(accountID)
	if err != nil {
		return nil, err
	}
	key := refreshKey{ownerID, clientConfigID}
	localUnlock := s.refreshLocks.Lock(key)
	lockDir := filepath.Join(s.dataDir, "oauth-refresh-locks")
	if err := os.MkdirAll(lockDir, 0700); err != nil {
		localUnlock()
		return nil, fmt.Errorf("create OAuth refresh lock directory: %w", err)
	}
	digest := sha256.Sum256([]byte(ownerID + "\x00" + clientConfigID))
	lockPath := filepath.Join(lockDir, hex.EncodeToString(digest[:])+".lock")
	fileUnlock, err := acquireOAuthFileLock(lockPath)
	if err != nil {
		localUnlock()
		return nil, fmt.Errorf("acquire OAuth refresh lock: %w", err)
	}
	return func() {
		if err := fileUnlock(); err != nil {
			s.log.Warn().Err(err).Str("account_id", ownerID).Str("client_config_id", clientConfigID).Msg("Failed to release OAuth refresh file lock")
		}
		localUnlock()
	}, nil
}

// OAuthTokenOwnerID resolves linked shared mailboxes to the account whose
// OAuth grant authorizes access. Other accounts own their own grant.
func (s *Store) OAuthTokenOwnerID(accountID string) (string, error) {
	var parentID sql.NullString
	err := s.db.QueryRow("SELECT shared_mailbox_parent_id FROM accounts WHERE id = ?", accountID).Scan(&parentID)
	if err != nil {
		return "", fmt.Errorf("resolve OAuth token owner for account %s: %w", accountID, err)
	}
	if parentID.Valid && parentID.String != "" {
		return parentID.String, nil
	}
	return accountID, nil
}
