// Server search under a chat search: asks Gmail (accounts that read through the
// Gmail API) for matches anywhere in the mailbox and stores them locally, so
// the normal search lists them. The state lives here, not in a component,
// because the list swaps between its empty state and its rows on every reload.
import { SearchGmailServer } from '../../../wailsjs/go/app/App'
import { accountStore } from '$lib/stores/accounts.svelte'
import { chatList } from '$lib/stores/chat.svelte'
import { mailApiRoutes } from '$lib/stores/mailApiRoutes.svelte'
import { toasts } from '$lib/stores/toast'

// Most matches one account's search stores; Gmail's page size is the ceiling.
const LIMIT = 100

class ChatServerSearch {
  busy = $state(false)
  // Query the last search ran for, and how many messages it stored.
  doneQuery = $state('')
  added = $state(0)

  /** Gmail-API accounts the current scope covers: one account, or all in the unified inbox. */
  readonly targets = $derived.by(() => {
    const gmail = (id: string) => mailApiRoutes.active[id] === 'gmail-api'
    if (chatList.isUnified) return accountStore.accounts.map((a) => a.account.id).filter(gmail)
    return chatList.accountId && gmail(chatList.accountId) ? [chatList.accountId] : []
  })

  readonly available = $derived(chatList.isSearch && this.targets.length > 0)

  async search() {
    if (this.busy || !this.available) return
    const query = chatList.query.trim()
    this.busy = true
    try {
      let added = 0
      for (const id of this.targets) added += (await SearchGmailServer(id, query, LIMIT)).added
      this.added = added
      this.doneQuery = query
      if (added > 0) await chatList.reload(true)
    } catch (err) {
      toasts.error(err instanceof Error ? err.message : String(err))
    } finally {
      this.busy = false
    }
  }
}

export const chatServerSearch = new ChatServerSearch()
