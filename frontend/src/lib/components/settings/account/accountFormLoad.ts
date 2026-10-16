// Data loading and provider selection for the account form.
import type { EmailProvider } from '$lib/config/providers'
// @ts-ignore - wailsjs path
import { app } from '../../../../../wailsjs/go/models'
// @ts-ignore - wailsjs path
import { GetAccountFoldersForMapping, GetAutoDetectedFolders, GetIdentities, GetAllAccountIdentities } from '../../../../../wailsjs/go/app/App'
import type { AccountFormState } from './accountFormState.svelte'
import { canUseOAuth } from './accountFormOAuth'

export function handleSmtpUseSameAsIncomingChange(s: AccountFormState, v: boolean) {
  s.smtpUseSameAsIncoming = v
  if (v) {
    s.smtpUsername = ''
    s.smtpPassword = ''
  }
}

export async function loadFoldersForMapping(s: AccountFormState) {
  if (!s.editAccount || s.availableFolders.length > 0) return

  s.loadingFolders = true
  try {
    s.availableFolders = await GetAccountFoldersForMapping(s.editAccount.id)
    s.autoDetectedFolders = await GetAutoDetectedFolders(s.editAccount.id)

    // Pre-select: use saved value if exists, otherwise auto-detected
    // @ts-ignore - wailsjs binding will have these fields after regeneration
    s.sentFolderPath = s.editAccount.sentFolderPath || s.autoDetectedFolders.sent || ''
    // @ts-ignore - wailsjs binding will have these fields after regeneration
    s.draftsFolderPath = s.editAccount.draftsFolderPath || s.autoDetectedFolders.drafts || ''
    // @ts-ignore - wailsjs binding will have these fields after regeneration
    s.trashFolderPath = s.editAccount.trashFolderPath || s.autoDetectedFolders.trash || ''
    // @ts-ignore - wailsjs binding will have these fields after regeneration
    s.spamFolderPath = s.editAccount.spamFolderPath || s.autoDetectedFolders.spam || ''
    // @ts-ignore - wailsjs binding will have these fields after regeneration
    s.archiveFolderPath = s.editAccount.archiveFolderPath || s.autoDetectedFolders.archive || ''
    // @ts-ignore - wailsjs binding will have these fields after regeneration
    s.allMailFolderPath = s.editAccount.allMailFolderPath || s.autoDetectedFolders.all || ''
    // @ts-ignore - wailsjs binding will have these fields after regeneration
    s.starredFolderPath = s.editAccount.starredFolderPath || s.autoDetectedFolders.starred || ''
  } catch (err) {
    console.error('Failed to load folders for mapping:', err)
  } finally {
    s.loadingFolders = false
  }
}

export async function loadDisplayName(s: AccountFormState, accountId: string) {
  try {
    const identities = await GetIdentities(accountId)
    const defaultIdentity = identities?.find((id: any) => id.isDefault) || identities?.[0]
    if (defaultIdentity) {
      s.displayName = defaultIdentity.name || ''
    }
  } catch (err) {
    console.error('Failed to load display name:', err)
  }
}

export async function loadIdentityGroups(s: AccountFormState) {
  try {
    const groups = (await GetAllAccountIdentities()) || []
    // Exclude the account being edited (its own identities can't be
    // a "Reply/Forward-with" target when it's marked no-outgoing) and
    // any other no-outgoing accounts (their identities aren't sendable
    // either).
    s.availableIdentityGroups = groups.filter((g: app.AccountIdentityGroup) => g.account?.id !== s.editAccount?.id && !g.account?.noOutgoingServer)
  } catch (err) {
    console.error('Failed to load identity groups for Reply/Forward-with picker:', err)
    s.availableIdentityGroups = []
  }
}

export function selectProvider(s: AccountFormState, provider: EmailProvider) {
  s.microsoftAccountType = ''
  s.selectedProvider = provider
  s.imapHost = provider.imap.host
  s.imapPort = provider.imap.port
  s.imapSecurity = provider.imap.security
  s.smtpHost = provider.smtp.host
  s.smtpPort = provider.smtp.port
  s.smtpSecurity = provider.smtp.security

  // Set auth method based on provider and configuration
  if (canUseOAuth(s, provider)) {
    s.authMethod = 'oauth2'
  } else {
    s.authMethod = 'password'
  }

  // Show advanced for custom provider
  s.showAdvanced = provider.id === 'custom'
  s.step = 'details'
}

export function goBackToProviders(s: AccountFormState) {
  s.step = 'provider'
  s.testResult = null
}
