// Server search under a chat search: asks the server (Gmail through the Gmail
// API, other accounts over IMAP) for matches anywhere in the mailbox and stores
// them locally, so the normal search lists them. The state lives here, not in a component,
// because the list swaps between its empty state and its rows on every reload.
import { LoadOlderGmail, LoadOlderIMAP, SearchGmailServer, SearchIMAPServer } from '../../../wailsjs/go/app/App'
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

  /** Accounts the current scope covers that can search the server: one account, or all in the unified inbox. Graph accounts cannot. */
  readonly targets = $derived.by(() => {
    const searchable = (id: string) => [undefined, 'gmail-api'].includes(mailApiRoutes.active[id])
    if (chatList.isUnified) return accountStore.accounts.map((a) => a.account.id).filter(searchable)
    return chatList.accountId && searchable(chatList.accountId) ? [chatList.accountId] : []
  })

  private isGmail(id: string) {
    return mailApiRoutes.active[id] === 'gmail-api'
  }

  // Accounts whose older mail is fully loaded, and the last Load older result.
  olderDone = $state<string[]>([])
  olderBusy = $state(false)
  olderAdded = $state<number | null>(null)

  readonly olderAvailable = $derived(!chatList.isSearch && this.targets.some((id) => !this.olderDone.includes(id)))
  readonly olderExhausted = $derived(!chatList.isSearch && this.targets.length > 0 && !this.olderAvailable)

  readonly available = $derived(chatList.isSearch && this.targets.length > 0)

  async search() {
    if (this.busy || !this.available) return
    const query = chatList.query.trim()
    this.busy = true
    try {
      let added = 0
      for (const id of this.targets) added += (await (this.isGmail(id) ? SearchGmailServer : SearchIMAPServer)(id, query, LIMIT)).added
      this.added = added
      this.doneQuery = query
      if (added > 0) await chatList.reload(true)
    } catch (err) {
      toasts.error(err instanceof Error ? err.message : String(err))
    } finally {
      this.busy = false
    }
  }

  /** Loads the next page of older mail for each searchable account in scope that has more. */
  async loadOlder() {
    if (this.olderBusy || !this.olderAvailable) return
    this.olderBusy = true
    try {
      let added = 0
      for (const id of this.targets.filter((t) => !this.olderDone.includes(t))) {
        const res = await (this.isGmail(id) ? LoadOlderGmail : LoadOlderIMAP)(id)
        added += res.added
        if (res.done) this.olderDone = [...this.olderDone, id]
      }
      this.olderAdded = added
      if (added > 0) await chatList.reload(true)
    } catch (err) {
      toasts.error(err instanceof Error ? err.message : String(err))
    } finally {
      this.olderBusy = false
    }
  }
}

export const chatServerSearch = new ChatServerSearch()
