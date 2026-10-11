package appstate

// UIState represents the persisted UI state across sessions
type UIState struct {
	// View state (what folder/view is shown in sidebar)
	SelectedAccountID  string `json:"selectedAccountId"`  // 'unified' or real account ID
	SelectedFolderID   string `json:"selectedFolderId"`   // 'inbox' (virtual) or real folder ID
	SelectedFolderName string `json:"selectedFolderName"` // Display name
	SelectedFolderType string `json:"selectedFolderType"` // inbox, sent, drafts, etc.

	// Conversation state (what's shown in viewer)
	SelectedThreadID              string `json:"selectedThreadId"`
	SelectedConversationAccountID string `json:"selectedConversationAccountId"` // Real account ID
	SelectedConversationFolderID  string `json:"selectedConversationFolderId"`  // Real folder ID

	// Pane widths
	SidebarWidth int `json:"sidebarWidth"`
	ListWidth    int `json:"listWidth"`

	// Sidebar section expand/collapse states
	ExpandedAccounts     map[string]bool `json:"expandedAccounts"`     // accountID -> isExpanded (default: true)
	UnifiedInboxExpanded bool            `json:"unifiedInboxExpanded"` // Unified Inbox section (default: true)
	CollapsedFolders     map[string]bool `json:"collapsedFolders"`     // folderID -> isCollapsed (default: false/absent)

	// Active extension pane: "mail" (default) or an extension id like "contacts".
	// The frontend uses this to decide whether to render Mail or an extension's
	// pane after the rail is shown. Missing field -> Mail.
	ActiveExtension string `json:"activeExtension,omitempty"`

	// Per-view left sidebar collapse: rail id ("mail", "contacts", "calendar",
	// ...) -> collapsed. Missing entry -> expanded.
	CollapsedSidebars map[string]bool `json:"collapsedSidebars,omitempty"`

	// Per-view pane widths for kit-based extension panes, keyed
	// "<rail id>.<pane>" (e.g. "contacts.sidebar", "contacts.list"). Mail uses
	// SidebarWidth/ListWidth above. Missing entry -> pane default.
	PaneWidths map[string]int `json:"paneWidths,omitempty"`

	// Chat list group expand/collapse: "pinned", "chats" or "low" -> expanded.
	// Missing entry -> the group's default (Low collapsed, the rest expanded).
	ChatGroupsExpanded map[string]bool `json:"chatGroupsExpanded,omitempty"`

	// Chat list groups trimmed to their first few rows ("Show fewer"):
	// "pinned", "chats" or "low" -> true. Missing entry -> every row shows.
	ChatGroupsShowFewer map[string]bool `json:"chatGroupsShowFewer,omitempty"`
}
