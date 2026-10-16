import { get } from 'svelte/store'
import { addToast } from '$lib/stores/toast'
import { _ } from '$lib/i18n'
// @ts-ignore - wailsjs runtime
import { EventsOn, EventsOff } from '../../../../wailsjs/runtime/runtime'
import type { SourceType, ContactSourceState } from './contactSourceState.svelte'
import { loadLinkedAccounts, loadCustomOAuthAccounts, loadExistingAddressbooks } from './contactSourceActions'

/** Registers the dialog's $effects; call once during component init, in order. */
export function registerContactSourceEffects(s: ContactSourceState) {
  $effect(() => {
    if (s.open && s.editSource) {
      s.name = s.editSource.name || ''
      s.url = s.editSource.url || ''
      s.username = s.editSource.username || ''
      s.password = '' // Don't load password
      s.syncInterval = s.editSource.sync_interval || 60
      s.hasDiscovered = false
      s.discoveredAddressbooks = []
      s.selectedAddressbooks = new Set()
      s.discoveryError = null
      s.sourceType = s.editSource.type as SourceType || 'carddav'

      // Load existing addressbooks for CardDAV
      if (s.editSource.type === 'carddav') {
        loadExistingAddressbooks(s)
      }

      // CardDAV edit: account-linked sources authenticate via OAuth.
      s.cardDavAuthMethod = s.editSource.account_id ? 'oauth' : 'password'
      s.selectedCustomAccountId = s.editSource.account_id || ''
      loadCustomOAuthAccounts(s)
    } else if (s.open && !s.editSource) {
      // Reset for new source.
      s.name = ''
      s.url = ''
      s.username = ''
      s.password = ''
      s.syncInterval = 60
      s.hasDiscovered = false
      s.discoveredAddressbooks = []
      s.selectedAddressbooks = new Set()
      s.discoveryError = null
      s.sourceType = 'carddav'
      s.selectedAccountId = ''
      s.oauthInProgress = false
      s.oauthEmail = ''
      s.cardDavAuthMethod = 'password'
      s.selectedCustomAccountId = ''

      // Linked accounts power Google/MS tabs; custom-OAuth accounts power the
      // CardDAV "OAuth account" option.
      loadLinkedAccounts(s)
      loadCustomOAuthAccounts(s)
    }
  })

  // Set up OAuth event listeners
  $effect(() => {
    if (s.open) {
      // Listen for OAuth started — captures the auth URL so the user can copy
      // it as a fallback if the browser doesn't s.open automatically.
      EventsOn('contact-source-oauth:started', (data: { provider: string; authURL?: string }) => {
        s.oauthAuthURL = data.authURL ?? null
      })

      // Listen for OAuth success
      EventsOn('contact-source-oauth:success', (data: { provider: string; email: string }) => {
        s.oauthInProgress = false
        s.oauthAuthURL = null
        s.oauthEmail = data.email
        if (!s.name) {
          s.name = `${data.provider === 'google' ? 'Google' : 'Microsoft'} Contacts (${data.email})`
        }
      })

      // Listen for OAuth error
      EventsOn('contact-source-oauth:error', (data: { error: string }) => {
        s.oauthInProgress = false
        s.oauthAuthURL = null
        console.error('OAuth failed:', data.error)
        addToast({ type: 'error', message: get(_)('toast.oauthFailed') })
      })

      // Listen for OAuth cancelled
      EventsOn('contact-source-oauth:cancelled', () => {
        s.oauthInProgress = false
        s.oauthAuthURL = null
      })

      return () => {
        EventsOff('contact-source-oauth:started')
        EventsOff('contact-source-oauth:success')
        EventsOff('contact-source-oauth:error')
        EventsOff('contact-source-oauth:cancelled')
      }
    }
  })
}
