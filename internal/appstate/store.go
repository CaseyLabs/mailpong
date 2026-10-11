package appstate

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/hkdb/aerion/internal/logging"
)

const (
	// KeyUIState is the key for storing UI state
	KeyUIState = "ui_state"
)

// Store handles persistence of application state
type Store struct {
	db  *sql.DB
	log zerolog.Logger
}

// NewStore creates a new app state store
func NewStore(db *sql.DB) *Store {
	return &Store{
		db:  db,
		log: logging.WithComponent("appstate"),
	}
}

// Get retrieves a value by key
func (s *Store) Get(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM app_state WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get app state key %s: %w", key, err)
	}
	return value, nil
}

// Set stores a value by key
func (s *Store) Set(key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO app_state (key, value, updated_at) 
		VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = ?
	`, key, value, time.Now(), value, time.Now())
	if err != nil {
		return fmt.Errorf("failed to set app state key %s: %w", key, err)
	}
	return nil
}

// Delete removes a key from the store
func (s *Store) Delete(key string) error {
	_, err := s.db.Exec("DELETE FROM app_state WHERE key = ?", key)
	if err != nil {
		return fmt.Errorf("failed to delete app state key %s: %w", key, err)
	}
	return nil
}

// defaultUIState returns the UI state used when none is saved or it is unreadable.
func defaultUIState() *UIState {
	return &UIState{
		SidebarWidth:         240,
		ListWidth:            420,
		ExpandedAccounts:     make(map[string]bool),
		UnifiedInboxExpanded: true,
		CollapsedFolders:     make(map[string]bool),
		CollapsedSidebars:    make(map[string]bool),
		PaneWidths:           make(map[string]int),
		ChatGroupsExpanded:   make(map[string]bool),
	}
}

// GetUIState retrieves the saved UI state
func (s *Store) GetUIState() (*UIState, error) {
	value, err := s.Get(KeyUIState)
	if err != nil {
		return nil, err
	}
	if value == "" {
		// Return default state if not set
		return defaultUIState(), nil
	}

	var state UIState
	if err := json.Unmarshal([]byte(value), &state); err != nil {
		s.log.Warn().Err(err).Msg("Failed to parse UI state, returning default")
		return defaultUIState(), nil
	}

	// Ensure maps are initialized (for older saved states)
	if state.ExpandedAccounts == nil {
		state.ExpandedAccounts = make(map[string]bool)
	}
	if state.CollapsedFolders == nil {
		state.CollapsedFolders = make(map[string]bool)
	}
	if state.CollapsedSidebars == nil {
		state.CollapsedSidebars = make(map[string]bool)
	}
	if state.PaneWidths == nil {
		state.PaneWidths = make(map[string]int)
	}
	if state.ChatGroupsExpanded == nil {
		state.ChatGroupsExpanded = make(map[string]bool)
	}

	return &state, nil
}

// SaveUIState persists the UI state
func (s *Store) SaveUIState(state *UIState) error {
	if state == nil {
		return nil
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal UI state: %w", err)
	}

	return s.Set(KeyUIState, string(data))
}
