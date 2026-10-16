// The account form's reactive effects. Call once during component initialization.
import { detectProvider, getCustomProvider } from '$lib/config/providers'
import { oauthStore } from '$lib/stores/oauth.svelte'
import type { AccountFormState } from './accountFormState.svelte'
import { canUseOAuth, checkOAuthConfiguration } from './accountFormOAuth'
import { loadDisplayName, loadIdentityGroups, selectProvider } from './accountFormLoad'

export function registerAccountFormEffects(s: AccountFormState) {
  // Initialize form when editing (only once)
  $effect(() => {
    if (s.editAccount && !s.initialized) {
      s.initialized = true
      s.step = 'details'
      s.name = s.editAccount.name
      s.email = s.editAccount.email
      s.username = s.editAccount.username
      s.imapHost = s.editAccount.imapHost
      s.imapPort = s.editAccount.imapPort
      s.imapSecurity = s.editAccount.imapSecurity
      s.imapAuthMechanism = s.editAccount.imapAuthMechanism || 'auto'
      s.smtpHost = s.editAccount.smtpHost
      s.smtpPort = s.editAccount.smtpPort
      s.smtpSecurity = s.editAccount.smtpSecurity
      s.smtpAuthMechanism = s.editAccount.smtpAuthMechanism || 'auto'
      s.syncPeriodDays = String(s.editAccount.syncPeriodDays)
      // @ts-ignore - syncInterval from backend
      s.syncInterval = String(s.editAccount.syncInterval ?? 30)
      s.readReceiptRequestPolicy = s.editAccount.readReceiptRequestPolicy || 'never'
      // @ts-ignore - authType from backend
      s.authMethod = s.editAccount.authType === 'oauth2' ? 'oauth2' : 'password'
      // @ts-ignore - color from backend
      s.color = s.editAccount.color || ''

      // Initialize folder mappings (will be populated when section is expanded)
      // @ts-ignore - wailsjs binding will have these fields after regeneration
      s.sentFolderPath = s.editAccount.sentFolderPath || ''
      // @ts-ignore - wailsjs binding will have these fields after regeneration
      s.draftsFolderPath = s.editAccount.draftsFolderPath || ''
      // @ts-ignore - wailsjs binding will have these fields after regeneration
      s.trashFolderPath = s.editAccount.trashFolderPath || ''
      // @ts-ignore - wailsjs binding will have these fields after regeneration
      s.spamFolderPath = s.editAccount.spamFolderPath || ''
      // @ts-ignore - wailsjs binding will have these fields after regeneration
      s.archiveFolderPath = s.editAccount.archiveFolderPath || ''
      // @ts-ignore - wailsjs binding will have these fields after regeneration
      s.allMailFolderPath = s.editAccount.allMailFolderPath || ''
      // @ts-ignore - wailsjs binding will have these fields after regeneration
      s.starredFolderPath = s.editAccount.starredFolderPath || ''

      // Try to detect provider
      s.selectedProvider = detectProvider(s.email) ?? getCustomProvider()
      s.showAdvanced = s.selectedProvider.id === 'custom'

      // Load display name from the default identity
      loadDisplayName(s, s.editAccount.id)
    }
  })

  // Initialize OAuth configuration check
  $effect(() => {
    if (!s.oauthInitialized) {
      s.oauthInitialized = true
      checkOAuthConfiguration(s)
      // Initialize OAuth event listeners
      oauthStore.initEvents()
      // Load sendable identity groups for the Reply/Forward-with picker.
      // Used only when the user toggles "No outgoing server" on; cheap
      // single Wails call so load it up-front for snappier UI.
      loadIdentityGroups(s)
    }
  })

  // Update authMethod when OAuth configuration finishes loading.
  // If a user selects a provider before the async OAuth check completes,
  // this corrects the auth method once we know OAuth is available.
  // Does NOT depend on authMethod — otherwise clicking "App Password"
  // gets immediately reverted back to oauth2.
  $effect(() => {
    const _dep = s.oauthConfigured
    if (!s.editAccount && s.selectedProvider && canUseOAuth(s, s.selectedProvider)) {
      s.authMethod = 'oauth2'
    }
  })

  // Auto-detect provider and auto-fill fields when email changes
  $effect(() => {
    if (!s.email) return

    // Auto-fill username with full email
    if (!s.username || s.username === s.prevEmail) {
      s.username = s.email
    }
    s.prevEmail = s.email

    // Try to detect provider
    const detected = s.editAccount ? null : detectProvider(s.email)
    if (detected && detected.id !== s.selectedProvider?.id) {
      selectProvider(s, detected)
    }

    // Auto-fill name from email if empty
    if (!s.name) {
      const localPart = s.email.split('@')[0]
      if (localPart) {
        s.name = localPart.charAt(0).toUpperCase() + localPart.slice(1)
      }
    }
  })
}
