// Validation, connection test, certificate handling and submit of the account form.
import { get } from 'svelte/store'
import { oauthStore } from '$lib/stores/oauth.svelte'
import { accountStore } from '$lib/stores/accounts.svelte'
import { _ } from '$lib/i18n'
// @ts-ignore - wailsjs path
import { account } from '../../../../../wailsjs/go/models'
// @ts-ignore - wailsjs path
import { AcceptCertificate } from '../../../../../wailsjs/go/app/App'
import type { AccountFormState } from './accountFormState.svelte'
import type { OAuthCredentials } from './accountFormTypes'

export function buildConfig(s: AccountFormState): account.AccountConfig {
  return new account.AccountConfig({
    name: s.name,
    displayName: s.displayName,
    color: s.color,
    email: s.email,
    username: s.username || s.email,
    password: s.authMethod === 'oauth2' ? '' : s.password,
    imapHost: s.imapHost,
    imapPort: s.imapPort,
    imapSecurity: s.imapSecurity,
    imapAuthMechanism: s.imapAuthMechanism,
    smtpHost: s.smtpHost,
    smtpPort: s.smtpPort,
    smtpSecurity: s.smtpSecurity,
    noOutgoingServer: s.noOutgoingServer,
    smtpUsername: s.smtpUsername,
    smtpPassword: s.smtpPassword,
    smtpAuthMechanism: s.smtpAuthMechanism,
    replyForwardIdentityId: s.replyForwardIdentityID,
    authType: s.authMethod,
    syncPeriodDays: Number(s.syncPeriodDays),
    syncInterval: Number(s.syncInterval),
    readReceiptRequestPolicy: s.readReceiptRequestPolicy,
    // Folder mappings
    sentFolderPath: s.sentFolderPath,
    draftsFolderPath: s.draftsFolderPath,
    trashFolderPath: s.trashFolderPath,
    spamFolderPath: s.spamFolderPath,
    archiveFolderPath: s.archiveFolderPath,
    allMailFolderPath: s.allMailFolderPath,
    starredFolderPath: s.starredFolderPath,
  })
}

export function validate(s: AccountFormState): boolean {
  s.errors = {}

  if (!s.name.trim()) s.errors.name = get(_)('account.accountNameRequired')
  if (!s.displayName.trim()) s.errors.displayName = get(_)('account.displayNameRequired')
  if (!s.email.trim()) s.errors.email = get(_)('account.emailRequired')
  else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(s.email)) s.errors.email = get(_)('account.invalidEmail')

  // Password is only required for password auth on new accounts
  if (s.authMethod === 'password' && !s.password && !s.editAccount) {
    s.errors.password = get(_)('account.passwordRequired')
  }

  // For OAuth, check that the flow completed successfully
  if (s.authMethod === 'oauth2' && !s.editAccount && !oauthStore.isFlowSuccess) {
    s.errors.oauth = get(_)('account.pleaseCompleteSignIn')
  }

  if (!s.imapHost.trim()) s.errors.imapHost = get(_)('account.imapHostRequired')
  if (s.imapPort < 1 || s.imapPort > 65535) s.errors.imapPort = get(_)('account.invalidPort')
  // SMTP host/port checks only when the user wants outgoing.
  if (!s.noOutgoingServer) {
    if (!s.smtpHost.trim()) s.errors.smtpHost = get(_)('account.smtpHostRequired')
    if (s.smtpPort < 1 || s.smtpPort > 65535) s.errors.smtpPort = get(_)('account.invalidPort')
  }
  // Separate SMTP credentials (Generic only, toggle off): username
  // always required; password required on NEW accounts. Blank on EDIT
  // is "keep existing keyring entry."
  if (!s.noOutgoingServer && s.isGenericProvider && !s.smtpUseSameAsIncoming) {
    if (!s.smtpUsername.trim()) s.errors.smtpUsername = get(_)('account.usernameRequired')
    if (!s.editAccount && !s.smtpPassword) s.errors.smtpPassword = get(_)('account.passwordRequired')
  }

  return Object.keys(s.errors).length === 0
}

export async function handleTestConnection(s: AccountFormState) {
  if (!validate(s)) return

  s.testing = true
  s.testResult = null

  try {
    const result = await accountStore.testConnection(buildConfig(s))
    if (result.success) {
      s.testResult = { success: true, message: get(_)('account.connectionSuccessful') }
    } else if (result.certificateRequired && result.certificate) {
      s.pendingCertificate = result.certificate
      s.showCertDialog = true
    } else {
      s.testResult = { success: false, message: result.error || get(_)('account.connectionFailed') }
    }
  } catch (err) {
    console.error('Connection test failed:', err)
    s.testResult = {
      success: false,
      message: get(_)('account.connectionTestFailed'),
    }
  } finally {
    s.testing = false
  }
}

export async function handleCertAcceptOnce(s: AccountFormState) {
  if (!s.pendingCertificate) return
  await AcceptCertificate(s.imapHost, s.pendingCertificate, false)
  s.showCertDialog = false
  s.pendingCertificate = null
  handleTestConnection(s)
}

export async function handleCertAcceptPermanently(s: AccountFormState) {
  if (!s.pendingCertificate) return
  await AcceptCertificate(s.imapHost, s.pendingCertificate, true)
  s.showCertDialog = false
  s.pendingCertificate = null
  handleTestConnection(s)
}

export function handleCertDecline(s: AccountFormState) {
  s.showCertDialog = false
  s.pendingCertificate = null
  s.testResult = { success: false, message: get(_)('account.certificateDeclined') }
}

export async function handleSubmit(s: AccountFormState, e: Event) {
  e.preventDefault()
  if (!validate(s)) return

  s.submitting = true
  s.testResult = null

  try {
    // Build OAuth credentials if using OAuth
    let oauthCredentials: OAuthCredentials | undefined
    if (s.authMethod === 'oauth2' && oauthStore.isFlowSuccess && oauthStore.flowResult) {
      // Note: The actual tokens are stored in the backend during the OAuth flow
      // We just pass the metadata so the parent can complete the account setup
      oauthCredentials = {
        provider: oauthStore.flowResult.provider,
        microsoftAccountType: s.microsoftAccountType,
        accessToken: '', // Tokens are handled by backend
        refreshToken: '', // Tokens are handled by backend
        expiresIn: oauthStore.flowResult.expiresIn,
      }
    }

    if (!s.editAccount && oauthCredentials?.provider === 'microsoft' && !s.microsoftAccountType) {
      s.errors.oauth = get(_)('account.microsoftChooseType')
      return
    }
    await s.onSubmit?.(buildConfig(s), oauthCredentials)

    // Reset OAuth state on success
    if (s.authMethod === 'oauth2') {
      oauthStore.reset()
    }
  } catch (err) {
    console.error('Account save failed:', err)
    // All add paths (manual AddAccount, OAuth, custom OAuth) surface the store's
    // ErrAccountExists ("account with this email already exists") on a duplicate
    // email — show the specific message instead of the generic save failure.
    const isDuplicate = String(err).toLowerCase().includes('already exists')
    s.testResult = {
      success: false,
      message: isDuplicate ? get(_)('toast.accountEmailExists') : get(_)('account.saveFailed'),
    }
  } finally {
    s.submitting = false
  }
}
