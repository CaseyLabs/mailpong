// UI State persistence store
// Handles saving and loading UI state across app sessions

// @ts-ignore - wailsjs bindings
import { GetUIState, SaveUIState } from '../../../wailsjs/go/app/App'
// @ts-ignore - wailsjs bindings
import { appstate } from '../../../wailsjs/go/models'

export interface UIState {
  selectedAccountId: string | null
  selectedFolderId: string | null
  selectedFolderName: string
  selectedFolderType: string | null
  selectedThreadId: string | null
  selectedConversationAccountId: string | null
  selectedConversationFolderId: string | null
  sidebarWidth: number
  listWidth: number
  // Sidebar section expand/collapse states
  expandedAccounts: Record<string, boolean>  // accountId -> isExpanded (default: true)
  unifiedInboxExpanded: boolean              // Unified Inbox section (default: true)
  collapsedFolders: Record<string, boolean>  // folderId -> isCollapsed (default: true/collapsed, false = explicitly expanded)
  // Active extension pane: 'mail' (default) or an extension id like 'contacts'.
  activeExtension: string
  // Per-view left sidebar collapse: rail id ('mail', 'contacts', ...) -> collapsed.
  collapsedSidebars: Record<string, boolean>
  // Kit pane widths keyed '<rail id>.<pane>' ('contacts.sidebar', ...). Mail
  // uses sidebarWidth/listWidth. Missing entry -> pane default.
  paneWidths: Record<string, number>
  // Chat list groups ('pinned', 'chats', 'low') -> expanded. Missing entry ->
  // the group's default (see isChatGroupExpanded).
  chatGroupsExpanded: Record<string, boolean>
  // Chat list groups trimmed to their first few rows ("Show fewer") -> true.
  // Missing entry -> every row shows.
  chatGroupsShowFewer: Record<string, boolean>
}

// Pane width constraints
const SIDEBAR_MIN = 180
const SIDEBAR_MAX = 400
const LIST_MIN = 280
const LIST_MAX = 600
const SIDEBAR_DEFAULT = 240
const LIST_DEFAULT = 420

// Default state
const defaultState: UIState = {
  selectedAccountId: null,
  selectedFolderId: null,
  selectedFolderName: 'Inbox',
  selectedFolderType: 'inbox',
  selectedThreadId: null,
  selectedConversationAccountId: null,
  selectedConversationFolderId: null,
  sidebarWidth: SIDEBAR_DEFAULT,
  listWidth: LIST_DEFAULT,
  expandedAccounts: {},
  unifiedInboxExpanded: true,
  collapsedFolders: {},
  activeExtension: 'mail',
  collapsedSidebars: {},
  paneWidths: {},
  chatGroupsExpanded: {},
  chatGroupsShowFewer: {},
}

// Current state (in-memory cache)
let currentState: UIState = { ...defaultState }

// Reactive signal to notify when UI state has been loaded
// Sidebar can depend on this to re-initialize expanded states
let uiStateLoadedVersion = $state(0)

// Reactive mirror of activeExtension specifically. The rail and the main pane
// swap depend on this and live in different components, so a $state at the
// module level keeps them in sync without prop drilling. currentState still
// holds the persisted value; this mirror is what consumers read.
let activeExtensionState = $state<string>('mail')

// Reactive mirror of collapsedSidebars (currentState isn't reactive), read by
// the sidebar toggle and the sidebars themselves.
let collapsedSidebarsState = $state<Record<string, boolean>>({})

// Reactive mirror of paneWidths, read by kit panes while they render.
let paneWidthsState = $state<Record<string, number>>({})

// Reactive mirror of chatGroupsExpanded, read by the chat list.
let chatGroupsExpandedState = $state<Record<string, boolean>>({})
// Reactive mirror of chatGroupsShowFewer.
let chatGroupsShowFewerState = $state<Record<string, boolean>>({})

// Clamp a value within bounds
export function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value))
}

// Load state from backend on startup
export async function loadUIState(): Promise<UIState> {
  try {
    const state = await GetUIState()
    if (state) {
      // Map from backend model to frontend interface
      // Backend uses camelCase JSON tags that match our interface
      currentState = {
        selectedAccountId: state.selectedAccountId || null,
        selectedFolderId: state.selectedFolderId || null,
        selectedFolderName: state.selectedFolderName || 'Inbox',
        selectedFolderType: state.selectedFolderType || 'inbox',
        selectedThreadId: state.selectedThreadId || null,
        selectedConversationAccountId: state.selectedConversationAccountId || null,
        selectedConversationFolderId: state.selectedConversationFolderId || null,
        // Validate and clamp pane widths
        sidebarWidth: clamp(state.sidebarWidth || SIDEBAR_DEFAULT, SIDEBAR_MIN, SIDEBAR_MAX),
        listWidth: clamp(state.listWidth || LIST_DEFAULT, LIST_MIN, LIST_MAX),
        // Sidebar expand/collapse states
        expandedAccounts: state.expandedAccounts || {},
        unifiedInboxExpanded: state.unifiedInboxExpanded !== false, // default true
        collapsedFolders: state.collapsedFolders || {},
        activeExtension: state.activeExtension || 'mail',
        collapsedSidebars: state.collapsedSidebars || {},
        paneWidths: state.paneWidths || {},
        chatGroupsExpanded: state.chatGroupsExpanded || {},
        chatGroupsShowFewer: state.chatGroupsShowFewer || {},
      }
      activeExtensionState = currentState.activeExtension
      collapsedSidebarsState = currentState.collapsedSidebars
      paneWidthsState = currentState.paneWidths
      chatGroupsExpandedState = currentState.chatGroupsExpanded
      chatGroupsShowFewerState = currentState.chatGroupsShowFewer
    }
  } catch (err) {
    console.error('Failed to load UI state:', err)
  }
  // Increment version to trigger reactive updates in components waiting for state
  uiStateLoadedVersion++
  return currentState
}

// Get the reactive version number (components can depend on this to re-run effects when state loads)
export function getUIStateVersion(): number {
  return uiStateLoadedVersion
}

// Debounced save
let saveTimer: ReturnType<typeof setTimeout> | null = null

export function saveUIState(updates: Partial<UIState>): void {
  // Merge updates into current state
  currentState = { ...currentState, ...updates }

  // Clamp pane widths if updated
  if (updates.sidebarWidth !== undefined) {
    currentState.sidebarWidth = clamp(updates.sidebarWidth, SIDEBAR_MIN, SIDEBAR_MAX)
  }
  if (updates.listWidth !== undefined) {
    currentState.listWidth = clamp(updates.listWidth, LIST_MIN, LIST_MAX)
  }

  // Debounce: save at most once per second
  if (saveTimer) clearTimeout(saveTimer)
  saveTimer = setTimeout(async () => {
    try {
      // Convert to backend model format
      const backendState: appstate.UIState = {
        selectedAccountId: currentState.selectedAccountId || '',
        selectedFolderId: currentState.selectedFolderId || '',
        selectedFolderName: currentState.selectedFolderName,
        selectedFolderType: currentState.selectedFolderType || '',
        selectedThreadId: currentState.selectedThreadId || '',
        selectedConversationAccountId: currentState.selectedConversationAccountId || '',
        selectedConversationFolderId: currentState.selectedConversationFolderId || '',
        sidebarWidth: currentState.sidebarWidth,
        listWidth: currentState.listWidth,
        expandedAccounts: currentState.expandedAccounts,
        unifiedInboxExpanded: currentState.unifiedInboxExpanded,
        collapsedFolders: currentState.collapsedFolders,
        activeExtension: currentState.activeExtension,
        collapsedSidebars: currentState.collapsedSidebars,
        paneWidths: currentState.paneWidths,
        chatGroupsExpanded: currentState.chatGroupsExpanded,
        chatGroupsShowFewer: currentState.chatGroupsShowFewer,
      }
      await SaveUIState(backendState)
    } catch (err) {
      console.error('Failed to save UI state:', err)
    }
  }, 1000)
}

// Helper to check if an account is expanded (defaults to true if not set)
export function isAccountExpanded(accountId: string): boolean {
  return currentState.expandedAccounts[accountId] !== false
}

// Helper to set account expanded state
export function setAccountExpanded(accountId: string, expanded: boolean): void {
  const newExpandedAccounts = { ...currentState.expandedAccounts, [accountId]: expanded }
  saveUIState({ expandedAccounts: newExpandedAccounts })
}

// Helper to check if unified inbox is expanded
export function isUnifiedInboxExpanded(): boolean {
  return currentState.unifiedInboxExpanded !== false
}

// Helper to set unified inbox expanded state
export function setUnifiedInboxExpanded(expanded: boolean): void {
  saveUIState({ unifiedInboxExpanded: expanded })
}

// Helper to check if a folder is collapsed (defaults to true/collapsed if not set)
export function isFolderCollapsed(folderId: string): boolean {
  return currentState.collapsedFolders[folderId] !== false
}

// Helper to set folder collapsed state
export function setFolderCollapsed(folderId: string, collapsed: boolean): void {
  const newCollapsedFolders = { ...currentState.collapsedFolders, [folderId]: collapsed }
  saveUIState({ collapsedFolders: newCollapsedFolders })
}

// Get current state (synchronous)
export function getUIState(): UIState {
  return currentState
}

// Active extension helpers.
//
// Returns 'mail' by default so the existing mail UI keeps rendering when no
// extension has ever been opened. Switching to an extension only persists the
// name — it does NOT clear the mail selection (selectedFolderId, selectedThreadId),
// so toggling back to Mail restores the previous mail context exactly.
export function getActiveExtension(): string {
  return activeExtensionState
}

export function setActiveExtension(name: string): void {
  const value = name || 'mail'
  activeExtensionState = value
  saveUIState({ activeExtension: value })
}

// Get pane width constraints (for UI components)
export const paneConstraints = {
  sidebar: { min: SIDEBAR_MIN, max: SIDEBAR_MAX, default: SIDEBAR_DEFAULT },
  list: { min: LIST_MIN, max: LIST_MAX, default: LIST_DEFAULT },
}

export type PaneKind = keyof typeof paneConstraints

// Kit pane width helpers. Keyed '<rail id>.<pane>' so each view remembers its
// own widths. Reads clamp to the pane kind's constraints (covering loaded
// state); setPaneWidth updates the live width during a drag and
// savePaneWidths persists once the drag ends.
export function getPaneWidth(key: string, kind: PaneKind): number {
  const { min, max, default: fallback } = paneConstraints[kind]
  return clamp(paneWidthsState[key] || fallback, min, max)
}

export function setPaneWidth(key: string, width: number): void {
  paneWidthsState = { ...paneWidthsState, [key]: Math.round(width) }
}

export function savePaneWidths(): void {
  saveUIState({ paneWidths: paneWidthsState })
}

// Left sidebar collapse helpers. Keyed by rail id so each view (mail and
// every extension) remembers its own state. Missing entry -> expanded, except
// mail, whose chat list scope menu covers inbox switching.
export function isSidebarCollapsed(view: string = activeExtensionState): boolean {
  return collapsedSidebarsState[view] ?? view === 'mail'
}

function setSidebarCollapsed(view: string, collapsed: boolean): void {
  collapsedSidebarsState = { ...collapsedSidebarsState, [view]: collapsed }
  saveUIState({ collapsedSidebars: collapsedSidebarsState })
}

export function toggleSidebarCollapsed(view: string = activeExtensionState): void {
  setSidebarCollapsed(view, !isSidebarCollapsed(view))
}

// Chat list group expand/collapse helpers. Missing entry -> expanded, except
// the Low priority group, which starts collapsed.
export type ChatGroup = 'pinned' | 'chats' | 'low'

export function isChatGroupExpanded(group: ChatGroup): boolean {
  return chatGroupsExpandedState[group] ?? group !== 'low'
}

export function toggleChatGroupExpanded(group: ChatGroup): void {
  chatGroupsExpandedState = { ...chatGroupsExpandedState, [group]: !isChatGroupExpanded(group) }
  saveUIState({ chatGroupsExpanded: chatGroupsExpandedState })
}

// A group set to "Show fewer" lists only its first few rows. Missing entry ->
// every row shows.
export function isChatGroupShowingFewer(group: ChatGroup): boolean {
  return chatGroupsShowFewerState[group] ?? false
}

export function toggleChatGroupShowFewer(group: ChatGroup): void {
  chatGroupsShowFewerState = { ...chatGroupsShowFewerState, [group]: !isChatGroupShowingFewer(group) }
  saveUIState({ chatGroupsShowFewer: chatGroupsShowFewerState })
}
