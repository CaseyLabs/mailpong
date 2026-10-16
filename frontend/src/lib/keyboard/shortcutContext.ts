// What the Mail shortcut handlers need from App.svelte: the pane component
// refs (plain getters, so a handler always sees the current instance) and the
// actions that live outside the keyboard code.

import type Sidebar from '$lib/components/sidebar/Sidebar.svelte'
import type ChatList from '$lib/components/chat/ChatList.svelte'
import type ChatView from '$lib/components/chat/ChatView.svelte'
import type { BulkActions } from './bulkActions'

export interface ShortcutContext {
  sidebar(): Sidebar | null
  list(): ChatList | null
  viewer(): ChatView | null
  bulk: BulkActions
  quit(): void
  focusListSearch(): void
}
