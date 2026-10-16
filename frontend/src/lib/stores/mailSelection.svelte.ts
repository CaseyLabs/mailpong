// Mail view selection: the folder the list shows, the open conversation and
// the viewer's focus mode. App.svelte, the global shortcuts and the composer
// controller share this one state. Every change that the user would expect to
// survive a restart goes through the functions below, which persist it.

import { accountStore } from './accounts.svelte'
import { saveUIState, type UIState } from './uiState.svelte'
import { hideSidebar, showViewer } from './layout.svelte'

export type SelectionSource = 'unified' | 'account' | null
export type FocusMode = 'off' | 'thread' | 'message'

// Starts on All Inboxes, so the Accounts column highlights what the list shows.
export const mailSelection = $state({
  accountId: 'unified' as string | null,
  folderId: 'inbox' as string | null,
  folderName: 'All Inboxes',
  folderType: 'inbox' as string | null,
  // Where the folder was picked: the unified section or an account tree.
  source: 'unified' as SelectionSource,
  threadId: null as string | null,
  conversationFolderId: null as string | null,
  conversationAccountId: null as string | null,
  // Focus mode: the viewer (or one message) takes the whole window. Resets on
  // conversation change, Esc, the back arrow and app reload.
  focusMode: 'off' as FocusMode,
  focusedMessageId: null as string | null,
})

export function clearConversation() {
  mailSelection.threadId = null
  mailSelection.conversationFolderId = null
  mailSelection.conversationAccountId = null
}

export function exitFocusMode() {
  mailSelection.focusMode = 'off'
  mailSelection.focusedMessageId = null
}

export function toggleThreadFocus() {
  if (mailSelection.focusMode === 'thread') {
    exitFocusMode()
    return
  }
  mailSelection.focusMode = 'thread'
  mailSelection.focusedMessageId = null
}

export function focusMessage(messageId: string) {
  mailSelection.focusMode = 'message'
  mailSelection.focusedMessageId = messageId
}

// Resolve an account ID that may be 'unified' to a real account ID: the first
// real account when the input is 'unified' or falsy.
export function resolveAccountId(id: string | null): string | undefined {
  if (id && id !== 'unified') return id
  return accountStore.accounts[0]?.account.id
}

// The account a conversation action applies to: the open conversation's (it
// matters in the unified inbox), else the selected account, else the first.
export function conversationAccountId(): string | undefined {
  return resolveAccountId(mailSelection.conversationAccountId) || resolveAccountId(mailSelection.accountId)
}

// Select a folder and clear the open conversation.
export function selectFolder(
  accountId: string,
  folderId: string,
  folderName: string,
  folderType: string,
  source: 'unified' | 'account'
) {
  mailSelection.accountId = accountId
  mailSelection.folderId = folderId
  mailSelection.folderName = folderName
  mailSelection.folderType = folderType
  mailSelection.source = source
  clearConversation()
  hideSidebar()

  saveUIState({
    selectedAccountId: accountId,
    selectedFolderId: folderId,
    selectedFolderName: folderName,
    selectedFolderType: folderType,
    selectedThreadId: null,
    selectedConversationAccountId: null,
    selectedConversationFolderId: null,
  })
}

export function selectUnifiedInbox() {
  selectFolder('unified', 'inbox', 'All Inboxes', 'inbox', 'unified')
}

export function selectConversation(threadId: string, folderId: string, accountId: string) {
  mailSelection.threadId = threadId
  mailSelection.conversationFolderId = folderId
  mailSelection.conversationAccountId = accountId
  showViewer()

  saveUIState({
    selectedThreadId: threadId,
    selectedConversationAccountId: accountId,
    selectedConversationFolderId: folderId,
  })
}

// Show a conversation from a notification click: the folder is highlighted
// under the unified section and the conversation opens. Unlike
// selectConversation it does not touch the responsive view.
export function selectFromNotification(accountId: string, folderId: string, threadId: string, folderInfo: { name: string; type: string } | null) {
  mailSelection.accountId = accountId
  mailSelection.folderId = folderId
  mailSelection.folderName = folderInfo?.name || 'Inbox'
  mailSelection.folderType = folderInfo?.type || 'inbox'
  mailSelection.source = 'unified'
  mailSelection.threadId = threadId
  mailSelection.conversationAccountId = accountId
  mailSelection.conversationFolderId = folderId

  saveUIState({
    selectedAccountId: accountId,
    selectedFolderId: folderId,
    selectedFolderName: folderInfo?.name || 'Inbox',
    selectedFolderType: folderInfo?.type || 'inbox',
    selectedThreadId: threadId,
    selectedConversationAccountId: accountId,
    selectedConversationFolderId: folderId,
  })
}

// Restore the persisted selection at startup when its account still exists.
export function restoreSelection(uiState: UIState) {
  if (!uiState.selectedAccountId || !uiState.selectedFolderId) return
  const isUnified = uiState.selectedAccountId === 'unified'
  const accountExists = isUnified || accountStore.accounts.some(
    a => a.account.id === uiState.selectedAccountId
  )
  if (!accountExists) return

  mailSelection.accountId = uiState.selectedAccountId
  mailSelection.folderId = uiState.selectedFolderId
  mailSelection.folderName = uiState.selectedFolderName || 'Inbox'
  mailSelection.folderType = uiState.selectedFolderType
  // The saved state has no source; an account folder highlights in its tree.
  mailSelection.source = isUnified ? 'unified' : 'account'

  if (uiState.selectedThreadId) {
    mailSelection.threadId = uiState.selectedThreadId
    mailSelection.conversationAccountId = uiState.selectedConversationAccountId
    mailSelection.conversationFolderId = uiState.selectedConversationFolderId
  }
}
