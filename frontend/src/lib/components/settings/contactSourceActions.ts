import { get } from 'svelte/store'
import { addToast } from '$lib/stores/toast'
import { _ } from '$lib/i18n'
import { contactSourcesStore } from '$lib/stores/contactSources.svelte'
// @ts-ignore - wailsjs path
import {
  DiscoverCardDAVAddressbooks,
  DiscoverCardDAVAddressbooksOAuth,
  GetCustomOAuthAccounts,
  AddContactSource,
  UpdateContactSource,
  GetSourceAddressbooks,
} from '../../../../wailsjs/go/app/App.js'
// @ts-ignore - wailsjs path
import type { carddav } from '../../../../wailsjs/go/models'
import type { ContactSourceState } from './contactSourceState.svelte'

// Load linked accounts when switching to OAuth tabs
export async function loadLinkedAccounts(s: ContactSourceState) {
  s.loadingAccounts = true
  try {
    s.linkedAccounts = await contactSourcesStore.getLinkedAccounts()
  } finally {
    s.loadingAccounts = false
  }
}

// Load custom-OAuth mail accounts (provider "custom") whose token can back a
// CardDAV source on the same unified server.
export async function loadCustomOAuthAccounts(s: ContactSourceState) {
  s.loadingCustomAccounts = true
  try {
    s.customOAuthAccounts = (await GetCustomOAuthAccounts()) || []
  } catch (err) {
    console.error('Failed to load custom OAuth accounts:', err)
    s.customOAuthAccounts = []
  } finally {
    s.loadingCustomAccounts = false
  }
}

export async function loadExistingAddressbooks(s: ContactSourceState) {
  const editSource = s.editSource
  if (!editSource) return
  try {
    const addressbooks = await GetSourceAddressbooks(editSource.id)
    if (addressbooks) {
      // Convert to AddressbookInfo format
      s.discoveredAddressbooks = addressbooks.map((ab: carddav.Addressbook) => ({
        path: ab.path,
        name: ab.name,
        description: '',
      }))
      // Select all enabled ones
      s.selectedAddressbooks = new Set(
        addressbooks
          .filter((ab: carddav.Addressbook) => ab.enabled)
          .map((ab: carddav.Addressbook) => ab.path)
      )
      s.hasDiscovered = true
    }
  } catch (err) {
    console.error('Failed to load addressbooks:', err)
  }
}

export async function handleDiscover(s: ContactSourceState) {
  const t = get(_)
  if (s.cardDavIsOAuth && (!s.url || !s.selectedCustomAccountId)) {
    s.discoveryError = t('contactSource.fillUrlAccount')
    return
  }
  if (!s.cardDavIsOAuth && (!s.url || !s.username || !s.password)) {
    s.discoveryError = t('contactSource.fillUrlUserPass')
    return
  }

  s.discovering = true
  s.discoveryError = null
  s.discoveredAddressbooks = []
  s.selectedAddressbooks = new Set()

  let retry = false
  try {
    const addressbooks = s.cardDavIsOAuth
      ? await DiscoverCardDAVAddressbooksOAuth(s.url, s.selectedCustomAccountId)
      : await DiscoverCardDAVAddressbooks(s.url, s.username, s.password)
    if (addressbooks && addressbooks.length > 0) {
      s.discoveredAddressbooks = addressbooks
      s.selectedAddressbooks = new Set(addressbooks.map((ab: carddav.AddressbookInfo) => ab.path))
      s.hasDiscovered = true
      return
    }
    s.discoveryError = t('contactSource.noAddressbooksFound')
  } catch (err) {
    console.error('Discovery failed:', err)
    retry = await s.certPrompt.offerTrust(err)
    if (!retry) s.discoveryError = t('contactSource.discoveryFailed')
  } finally {
    s.discovering = false
  }
  if (retry) await handleDiscover(s)
}

export async function handleStartOAuth(s: ContactSourceState) {
  s.oauthInProgress = true
  try {
    await contactSourcesStore.startOAuthFlow(s.sourceType)
  } catch (err) {
    s.oauthInProgress = false
    console.error('Failed to start OAuth:', err)
    addToast({ type: 'error', message: get(_)('toast.failedToStartOAuth') })
  }
}

export async function handleSave(s: ContactSourceState) {
  const t = get(_)
  const editSource = s.editSource
  s.saving = true

  try {
    if (s.sourceType === 'carddav') {
      // CardDAV source — Basic password, or account-linked OAuth bearer.
      if (!s.name || !s.url) {
        addToast({ type: 'error', message: t('toast.fillRequiredFields') })
        return
      }
      if (s.cardDavIsOAuth && !s.selectedCustomAccountId) {
        addToast({ type: 'error', message: t('toast.selectOAuthAccount') })
        return
      }
      if (!s.cardDavIsOAuth && !s.username) {
        addToast({ type: 'error', message: t('toast.fillRequiredFields') })
        return
      }
      if (!s.cardDavIsOAuth && !editSource && !s.password) {
        addToast({ type: 'error', message: t('toast.passwordRequired') })
        return
      }

      if (s.selectedAddressbooks.size === 0) {
        addToast({ type: 'error', message: t('toast.selectAddressbook') })
        return
      }

      const config = {
        name: s.name,
        type: 'carddav' as const,
        url: s.url,
        username: s.cardDavIsOAuth ? '' : s.username,
        password: s.cardDavIsOAuth ? '' : s.password,
        account_id: s.cardDavIsOAuth ? s.selectedCustomAccountId : '',
        enabled: true,
        // New sources default writable=true — Basic and account-linked OAuth
        // alike, now that the bearer write path is wired. Users toggle off
        // later via Contacts extension settings → Write Access (UpdateSource
        // ignores this field on edits).
        writable: true,
        sync_interval: s.syncInterval,
        enabled_addressbooks: Array.from(s.selectedAddressbooks),
        // Pass the discovered display names along so the stored rows keep
        // the server's names instead of the path's last segment (#366).
        addressbook_names: Object.fromEntries(
          s.discoveredAddressbooks
            .filter((ab) => s.selectedAddressbooks.has(ab.path) && ab.name)
            .map((ab) => [ab.path, ab.name])
        ),
      }

      if (editSource) {
        await UpdateContactSource(editSource.id, config)
        addToast({ type: 'success', message: t('toast.contactSourceUpdated') })
      }
      if (!editSource) {
        await AddContactSource(config)
        addToast({ type: 'success', message: t('toast.contactSourceAdded') })
      }
    }
    if (s.sourceType !== 'carddav') {
      // Google or Microsoft source
      if (editSource && s.isEditingOAuthSource) {
        // Editing existing OAuth source — s.name + sync interval only.
        // Writable is managed in Contacts extension settings, not here.
        const config = {
          name: s.name,
          type: editSource.type as 'google' | 'microsoft',
          url: '',
          username: '',
          password: '',
          enabled: true,
          // Preserve existing writable; UpdateSource ignores this field
          // anyway but keep the config shape correct.
          writable: !!editSource.writable,
          sync_interval: s.syncInterval,
          enabled_addressbooks: [],
        }
        await UpdateContactSource(editSource.id, config)
        addToast({ type: 'success', message: t('toast.contactSourceUpdated') })
      } else if (s.selectedAccountId) {
        // Link to existing email account
        await contactSourcesStore.linkAccount(s.selectedAccountId, s.name, s.syncInterval)
        addToast({ type: 'success', message: t('toast.contactSourceLinked') })
      } else if (s.oauthEmail) {
        // Complete standalone OAuth flow
        await contactSourcesStore.completeOAuthSetup(s.name, s.syncInterval)
        addToast({ type: 'success', message: t('toast.contactSourceCreated') })
      } else {
        addToast({ type: 'error', message: t('toast.linkAccountOrSignIn') })
        return
      }
    }

    s.open = false
    s.onClose?.()
  } catch (err) {
    console.error('Failed to save:', err)
    addToast({ type: 'error', message: t('toast.failedToSave') })
  } finally {
    s.saving = false
  }
}
