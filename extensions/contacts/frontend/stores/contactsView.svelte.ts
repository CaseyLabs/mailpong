// View-local state for the Contacts extension's browse UI. Source selection,
// search query, and selected-row id are intentionally session-local — none of
// these need to survive across app launches.

// @ts-ignore - wailsjs bindings
import {
  Contacts_ListContactsForBrowse as ListContactsForBrowse,
  Contacts_GetContactDetail as GetContactDetail,
  Contacts_UpdateContact as UpdateContact,
  Contacts_DeleteLocalContact as DeleteLocalContact,
  Contacts_CreateContact as CreateContact,
  Contacts_ListAddressbooks as ListAddressbooks,
} from '$wailsjs/go/app/App'
// @ts-ignore - wailsjs bindings
import type { v1 } from '$wailsjs/go/models'
// Responsive (mobile) integration — match mail's pattern of firing
// showViewer/hideSidebar from the consumer's select actions. Layout-store
// calls are self-gating: showViewer is a no-op when not responsive,
// hideSidebar is a no-op when not narrow. Kit primitives (PaneLayout,
// SourceSidebar, DetailPane) handle overlay class application + back
// buttons + scrim; this store handles the "which view do we want to be
// in next" decisions on user actions.
import { isResponsive, showViewer, hideSidebar, registerViewerSelection } from '$lib/stores/layout.svelte'

// Source ID values the sidebar can dispatch:
//   ""                  → merged listing across all sources
//   "local"             → all local contacts (manual + collected)
//   "local:manual"      → user-added local contacts (Add Contact UI)
//   "local:collected"   → auto-collected local contacts (sent-mail recipients)
//   <uuid>              → a specific CardDAV source
let selectedSourceId = $state<string>('')
let searchQuery = $state<string>('')
let selectedContactId = $state<string | null>(null)
let contacts = $state<v1.Contact[]>([])
let detail = $state<v1.Contact | null>(null)
let loading = $state<boolean>(false)
let hasMore = $state<boolean>(false)
let loadingMore = $state<boolean>(false)

// Keep an open contact on screen when the window narrows or the user returns
// to Contacts from another view in a responsive layout.
registerViewerSelection('contacts', () => detail !== null)

export const contactsView = {
  get selectedSourceId(): string {
    return selectedSourceId
  },
  get searchQuery(): string {
    return searchQuery
  },
  get selectedContactId(): string | null {
    return selectedContactId
  },
  get contacts(): v1.Contact[] {
    return contacts
  },
  get detail(): v1.Contact | null {
    return detail
  },
  get loading(): boolean {
    return loading
  },
  get hasMore(): boolean {
    return hasMore
  },
  get loadingMore(): boolean {
    return loadingMore
  },
}

export function selectSource(sourceId: string): void {
  selectedSourceId = sourceId
  selectedContactId = null
  detail = null
  // Dismiss the sidebar overlay on narrow viewports. Self-gating store
  // call — no-op on full/medium.
  hideSidebar()
  // Caller (ContactsPane) decides when to call reloadContacts().
}

export function setSearchQuery(q: string): void {
  searchQuery = q
}

// Browse page size. Pages are appended via loadMoreContacts() — a fixed
// 200-row single fetch silently truncated large addressbooks (#278).
const BROWSE_PAGE_SIZE = 200

// Request sequence numbers. A response whose number is no longer current
// was overtaken by a newer request (new search, source switch, selection
// change) and is dropped so a slow stale reply can't overwrite fresh state.
let listSeq = 0
let detailSeq = 0

export async function reloadContacts(): Promise<void> {
  const seq = ++listSeq
  loading = true
  try {
    const page = await ListContactsForBrowse(searchQuery, selectedSourceId, BROWSE_PAGE_SIZE, 0) || []
    if (seq !== listSeq) return
    contacts = page
    hasMore = page.length === BROWSE_PAGE_SIZE
  } catch (err) {
    if (seq !== listSeq) return
    console.error('Failed to list contacts for browse:', err)
    contacts = []
    hasMore = false
  } finally {
    if (seq === listSeq) loading = false
  }
}

// Fetch the next page and append. No-op while a page is in flight or when
// the last fetch came back short (nothing more to load).
export async function loadMoreContacts(): Promise<void> {
  if (loadingMore || !hasMore) return
  const seq = listSeq
  loadingMore = true
  try {
    const page = await ListContactsForBrowse(searchQuery, selectedSourceId, BROWSE_PAGE_SIZE, contacts.length) || []
    // A reload started meanwhile: this page belongs to the old list.
    if (seq !== listSeq) return
    contacts = [...contacts, ...page]
    hasMore = page.length === BROWSE_PAGE_SIZE
  } catch (err) {
    console.error('Failed to load more contacts:', err)
  } finally {
    loadingMore = false
  }
}

// Focus-vs-activate split mirrors mail's ChatList behavior (and is
// enforced by the kit's ListPane semantics — see ListPane.svelte's onSelect
// and onActivate docstrings):
//
//   focusContact(id)    — j/k navigation. Updates the highlighted row only.
//                         Does NOT load detail, does NOT slide in the viewer.
//   activateContact(id) — Enter key or row click. Loads detail and (on
//                         responsive viewports) reveals the viewer overlay.
//
// Programmatic callers that want the "old" combined behavior (e.g.,
// post-create navigation in ContactsPane.handleCreated) call
// activateContact(id) explicitly.

export function focusContact(id: string | null): void {
  selectedContactId = id
  if (!id) {
    detail = null
  }
  // Intentionally NO detail load and NO showViewer here — focus changes
  // should not move data on/off the network or trigger overlay animations.
}

export async function activateContact(id: string | null): Promise<void> {
  selectedContactId = id
  if (!id) {
    detail = null
    return
  }
  // On responsive viewports, reveal the detail pane overlay. Self-gating
  // store call — no-op on full layout.
  if (isResponsive()) showViewer()
  const seq = ++detailSeq
  try {
    const loaded = await GetContactDetail(id)
    if (seq !== detailSeq || selectedContactId !== id) return
    detail = loaded
  } catch (err) {
    if (seq !== detailSeq || selectedContactId !== id) return
    console.error('Failed to load contact detail:', err)
    detail = null
  }
}


// Update a contact (local or CardDAV) with a multi-field patch. The backend
// dispatches by source — local writes via UpsertRecord, CardDAV PUTs to the
// server. On 412 conflict the backend emits "contacts:conflict" (ContactsPane
// reloads and shows the toast) and this method rejects; callers use
// isConflictError to keep the user's edits without a second error toast.
export async function updateContact(id: string, patch: v1.ContactPatch): Promise<void> {
  await UpdateContact(id, patch)
  // Refresh the list + detail view so changes are visible immediately.
  await reloadContacts()
  if (selectedContactId === id) {
    await activateContact(id)
  }
}

// Reports whether a rejected update/delete was a 412 conflict. Matches the
// text of coreapi.ErrConflict.Error() (internal/core/api/v1/errors.go), which
// is all Wails passes through.
export function isConflictError(err: unknown): boolean {
  const msg = err instanceof Error ? err.message : String(err)
  return msg.startsWith('conflict on contact')
}

// Delete a local (sent-recipient) contact entirely. After deletion the list
// reloads and detail view clears.
export async function deleteLocalContact(email: string): Promise<void> {
  await DeleteLocalContact(email)
  if (selectedContactId === email) {
    selectedContactId = null
    detail = null
  }
  await reloadContacts()
}

// Create a contact. Source dispatch happens in the backend (input.SourceID):
//   - "local:manual" → local manual entry; returns the normalized email
//   - <CardDAV UUID> → server-side PUT to input.AddressbookID (or the
//     source's first enabled addressbook when empty); returns the record UUID
// Throws on conflict — caller (AddContactDialog) translates "already exists"
// strings into a field-level error.
//
// Does NOT reload contacts or change the selected source — the caller
// (ContactsPane.handleCreated) controls the post-create UX so the dialog can
// close before the source switch.
export async function createContact(input: v1.ContactCreateInput): Promise<string> {
  return await CreateContact(input)
}

// Lists the enabled addressbooks for a CardDAV source. Used by the Add
// Contact dialog's addressbook sub-picker. Returns [] for empty / unknown /
// non-CardDAV source IDs (the backend returns nil which the bindings turn
// into undefined; this helper normalizes to []).
export async function listAddressbooks(sourceId: string): Promise<v1.Addressbook[]> {
  if (!sourceId) return []
  try {
    return (await ListAddressbooks(sourceId)) || []
  } catch (err) {
    console.error('Failed to list addressbooks:', err)
    return []
  }
}
