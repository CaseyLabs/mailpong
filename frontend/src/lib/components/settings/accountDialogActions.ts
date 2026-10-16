import { get } from 'svelte/store'
import { accountStore } from '$lib/stores/accounts.svelte'
import { oauthStore } from '$lib/stores/oauth.svelte'
import { addToast } from '$lib/stores/toast'
import { _ } from '$lib/i18n'
import type { OAuthCredentials } from './AccountForm.svelte'
// @ts-ignore - wailsjs path
import type { account } from '../../../../wailsjs/go/models'

// True when editing a generic-provider account (non-Gmail/Outlook/etc).
// Controls visibility of the SMTP-authentication UI on the Server tab
// and the corresponding hint on the General tab.
const KNOWN_PROVIDER_HOSTS = ['gmail.com', 'googlemail.com', 'outlook.com', 'office365.com', 'yahoo.com', 'aol.com', 'icloud.com', 'me.com', 'mac.com']

export function isGenericProviderHost(imapHost: string | undefined): boolean {
  return (imapHost ?? '') !== ''
    && !KNOWN_PROVIDER_HOSTS.some(h => (imapHost ?? '').includes(h))
}

// Create the account through the store route matching the wizard's auth choice.
export async function createAccountFromWizard(
  config: account.AccountConfig,
  oauthCredentials?: OAuthCredentials,
): Promise<{ result: account.Account; provider: string }> {
  let result: account.Account
  const isOAuth = config.authType === 'oauth2' && !!oauthCredentials
  const provider = isOAuth ? oauthCredentials!.provider : 'imap'

  switch (true) {
    case isOAuth && provider === 'custom':
      // Generic IMAP + user-supplied OAuth: pass the full config so the user's
      // IMAP/SMTP server settings are used (not hardcoded provider servers).
      result = await accountStore.addCustomOAuthAccount(config)
      break
    case isOAuth:
      result = await accountStore.addOAuthAccount(
        provider,
        config,
        oauthCredentials?.microsoftAccountType ?? ''
      )
      break
    default:
      result = await accountStore.addAccount(config)
  }
  return { result, provider }
}

export interface ReauthorizeCallbacks {
  setReauthorizing: (value: boolean) => void
  setSuccess: (value: boolean) => void
}

export async function reauthorizeAccount(accountId: string, accountName: string, cb: ReauthorizeCallbacks) {
  const t = get(_)
  cb.setReauthorizing(true)
  cb.setSuccess(false)
  try {
    await oauthStore.reauthorize(accountId)
    cb.setSuccess(true)
    addToast({
      type: 'success',
      message: t('toast.reauthorized', { values: { name: accountName } }),
      duration: 5000,
    })
    // Trigger a sync to verify the new token works
    await accountStore.syncAccount(accountId)
    addToast({
      type: 'success',
      message: t('toast.syncCompleted', { values: { name: accountName } }),
      duration: 3000,
    })
  } catch (err) {
    console.error('Failed to re-authorize:', err)
    cb.setSuccess(false)
    addToast({
      type: 'error',
      message: t('toast.failedToReauthorize'),
      duration: 8000,
    })
  } finally {
    cb.setReauthorizing(false)
  }
}
