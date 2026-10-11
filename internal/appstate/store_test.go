package appstate

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/caseylabs/mailpong/internal/database"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db.DB
}

func TestGetUIStateDefault(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)

	state, err := store.GetUIState()
	if err != nil {
		t.Fatalf("GetUIState() error = %v", err)
	}
	if state.SidebarWidth != 240 {
		t.Errorf("SidebarWidth = %d, want 240", state.SidebarWidth)
	}
	if state.ListWidth != 420 {
		t.Errorf("ListWidth = %d, want 420", state.ListWidth)
	}
	if !state.UnifiedInboxExpanded {
		t.Error("UnifiedInboxExpanded = false, want true")
	}
	if state.ExpandedAccounts == nil {
		t.Error("ExpandedAccounts is nil, want initialized map")
	}
	if state.CollapsedFolders == nil {
		t.Error("CollapsedFolders is nil, want initialized map")
	}
	if state.CollapsedSidebars == nil {
		t.Error("CollapsedSidebars is nil, want initialized map")
	}
	if state.PaneWidths == nil {
		t.Error("PaneWidths is nil, want initialized map")
	}
	if state.ChatGroupsExpanded == nil {
		t.Error("ChatGroupsExpanded is nil, want initialized map")
	}
	if state.ChatGroupsShowFewer == nil {
		t.Error("ChatGroupsShowFewer is nil, want initialized map")
	}
}

func TestSaveGetUIState(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)

	saved := &UIState{
		SelectedAccountID:    "acct-1",
		SelectedFolderID:     "folder-1",
		SelectedFolderName:   "Inbox",
		SelectedFolderType:   "inbox",
		SidebarWidth:         300,
		ListWidth:            500,
		ExpandedAccounts:     map[string]bool{"acct-1": true, "acct-2": false},
		UnifiedInboxExpanded: false,
		CollapsedFolders:     map[string]bool{"folder-2": true},
		CollapsedSidebars:    map[string]bool{"mail": true, "calendar": false},
		PaneWidths:           map[string]int{"contacts.sidebar": 280, "contacts.list": 360},
		ChatGroupsExpanded:   map[string]bool{"pinned": false, "low": true},
		ChatGroupsShowFewer:  map[string]bool{"chats": true},
	}

	if err := store.SaveUIState(saved); err != nil {
		t.Fatalf("SaveUIState() error = %v", err)
	}

	got, err := store.GetUIState()
	if err != nil {
		t.Fatalf("GetUIState() error = %v", err)
	}
	if got.SidebarWidth != 300 {
		t.Errorf("SidebarWidth = %d, want 300", got.SidebarWidth)
	}
	if got.ListWidth != 500 {
		t.Errorf("ListWidth = %d, want 500", got.ListWidth)
	}
	if got.SelectedAccountID != "acct-1" {
		t.Errorf("SelectedAccountID = %q, want %q", got.SelectedAccountID, "acct-1")
	}
	if got.SelectedFolderID != "folder-1" {
		t.Errorf("SelectedFolderID = %q, want %q", got.SelectedFolderID, "folder-1")
	}
	if got.UnifiedInboxExpanded {
		t.Error("UnifiedInboxExpanded = true, want false")
	}
	if !got.ExpandedAccounts["acct-1"] {
		t.Error("ExpandedAccounts[acct-1] = false, want true")
	}
	if got.ExpandedAccounts["acct-2"] {
		t.Error("ExpandedAccounts[acct-2] = true, want false")
	}
	if !got.CollapsedFolders["folder-2"] {
		t.Error("CollapsedFolders[folder-2] = false, want true")
	}
	if !got.CollapsedSidebars["mail"] {
		t.Error("CollapsedSidebars[mail] = false, want true")
	}
	if got.CollapsedSidebars["calendar"] || got.CollapsedSidebars["contacts"] {
		t.Error("CollapsedSidebars[calendar|contacts] = true, want false")
	}
	if got.PaneWidths["contacts.sidebar"] != 280 || got.PaneWidths["contacts.list"] != 360 {
		t.Errorf("PaneWidths = %v, want contacts.sidebar=280 contacts.list=360", got.PaneWidths)
	}
	if v, ok := got.ChatGroupsExpanded["pinned"]; !ok || v {
		t.Error("ChatGroupsExpanded[pinned] missing or true, want false")
	}
	if !got.ChatGroupsExpanded["low"] {
		t.Error("ChatGroupsExpanded[low] = false, want true")
	}
	if !got.ChatGroupsShowFewer["chats"] || got.ChatGroupsShowFewer["pinned"] {
		t.Errorf("ChatGroupsShowFewer = %v, want chats=true only", got.ChatGroupsShowFewer)
	}
}

// TestGetUIStateLegacyNoCollapsedSidebars covers state saved before the
// collapsedSidebars field existed: every sidebar must load expanded.
func TestGetUIStateLegacyNoCollapsedSidebars(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)

	if err := store.Set(KeyUIState, `{"sidebarWidth":260,"listWidth":420,"unifiedInboxExpanded":true}`); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	got, err := store.GetUIState()
	if err != nil {
		t.Fatalf("GetUIState() error = %v", err)
	}
	if got.CollapsedSidebars == nil {
		t.Fatal("CollapsedSidebars is nil, want initialized map")
	}
	if len(got.CollapsedSidebars) != 0 {
		t.Errorf("CollapsedSidebars = %v, want empty", got.CollapsedSidebars)
	}
	if got.PaneWidths == nil || len(got.PaneWidths) != 0 {
		t.Errorf("PaneWidths = %v, want initialized empty map", got.PaneWidths)
	}
}

func TestSetGetDelete(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)

	// Set
	if err := store.Set("test_key", "test_value"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Get
	val, err := store.Get("test_key")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if val != "test_value" {
		t.Errorf("Get() = %q, want %q", val, "test_value")
	}

	// Delete
	if err := store.Delete("test_key"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Get after delete returns empty string
	val, err = store.Get("test_key")
	if err != nil {
		t.Fatalf("Get() after Delete() error = %v", err)
	}
	if val != "" {
		t.Errorf("Get() after Delete() = %q, want empty string", val)
	}
}

func TestGetUIStateCorrupt(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)

	// Store invalid JSON under the ui_state key
	if err := store.Set(KeyUIState, "not valid json{{{"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// GetUIState should return defaults gracefully
	state, err := store.GetUIState()
	if err != nil {
		t.Fatalf("GetUIState() error = %v", err)
	}
	if state.SidebarWidth != 240 {
		t.Errorf("SidebarWidth = %d, want 240", state.SidebarWidth)
	}
	if state.ListWidth != 420 {
		t.Errorf("ListWidth = %d, want 420", state.ListWidth)
	}
	if !state.UnifiedInboxExpanded {
		t.Error("UnifiedInboxExpanded = false, want true")
	}
}

func TestSaveUIStateNilMaps(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)

	saved := &UIState{
		SidebarWidth:         280,
		ListWidth:            450,
		UnifiedInboxExpanded: true,
		ExpandedAccounts:     nil,
		CollapsedFolders:     nil,
	}

	if err := store.SaveUIState(saved); err != nil {
		t.Fatalf("SaveUIState() error = %v", err)
	}

	got, err := store.GetUIState()
	if err != nil {
		t.Fatalf("GetUIState() error = %v", err)
	}
	if got.SidebarWidth != 280 {
		t.Errorf("SidebarWidth = %d, want 280", got.SidebarWidth)
	}
	if got.ExpandedAccounts == nil {
		t.Error("ExpandedAccounts is nil, want initialized map")
	}
	if got.CollapsedFolders == nil {
		t.Error("CollapsedFolders is nil, want initialized map")
	}
}
