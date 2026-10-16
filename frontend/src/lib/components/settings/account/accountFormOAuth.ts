// OAuth sign-in logic of the account form (shipped providers and custom OAuth).
import { get } from 'svelte/store'
import { oauthStore } from '$lib/stores/oauth.svelte'
import { toasts } from '$lib/stores/toast'
import { _ } from '$lib/i18n'
import {
  isOAuthProvider,
  supportsCustomOAuth,
  getOAuthProviderType,
  type EmailProvider,
} from '$lib/config/providers'
// @ts-ignore - wailsjs path
import { DiscoverOAuthProvider } from '../../../../../wailsjs/go/app/App'
import type { AccountFormState } from './accountFormState.svelte'

export function chooseMicrosoftAccountType(s: AccountFormState, value: string) {
  s.microsoftAccountType = value
  if (s.smtpHost === 'smtp.office365.com' || s.smtpHost === 'smtp-mail.outlook.com') {
    s.smtpHost = value === 'personal' ? 'smtp-mail.outlook.com' : 'smtp.office365.com'
  }
}

export async function discoverCustomOAuth(s: AccountFormState) {
  const issuer = s.customOAuthIssuer.trim()
  if (!issuer) return
  s.customOAuthDiscovering = true
  s.customOAuthDiscoverError = ''
  try {
    const doc = await DiscoverOAuthProvider(issuer)
    s.customOAuthAuthURL = doc.authorizationEndpoint
    s.customOAuthTokenURL = doc.tokenEndpoint
    s.customOAuthUserinfoURL = doc.userinfoEndpoint
    s.customOAuthDiscovered = true
  } catch (err) {
    s.customOAuthDiscoverError = err instanceof Error ? err.message : String(err)
    s.customOAuthDiscovered = false
  } finally {
    s.customOAuthDiscovering = false
  }
}

export async function checkOAuthConfiguration(s: AccountFormState) {
  try {
    const [googleConfigured, microsoftConfigured] = await Promise.all([
      oauthStore.isProviderConfigured('google'),
      oauthStore.isProviderConfigured('microsoft'),
    ])
    s.oauthConfigured = {
      google: googleConfigured,
      microsoft: microsoftConfigured,
    }
  } catch (err) {
    console.error('Failed to check OAuth configuration:', err)
  }
}

export function canUseOAuth(s: AccountFormState, provider: EmailProvider | null): boolean {
  if (!provider) return false
  if (!isOAuthProvider(provider)) return false
  const oauthType = getOAuthProviderType(provider)
  if (!oauthType) return false
  return s.oauthConfigured[oauthType] ?? false
}

export async function startOAuthFlow(s: AccountFormState) {
  if (s.selectedProvider && getOAuthProviderType(s.selectedProvider) === 'microsoft' && !s.microsoftAccountType) return
  if (!s.selectedProvider) return

  // Custom ("bring your own app") provider: build the flow from user-entered
  // endpoints/creds rather than a shipped provider type.
  if (supportsCustomOAuth(s.selectedProvider) && s.authMethod === 'oauth2') {
    if (!s.customOAuthReady) return
    const scopes = s.customOAuthScopes
      .split(/[\s,]+/)
      .map((scope) => scope.trim())
      .filter(Boolean)
    try {
      await oauthStore.startCustomFlow(
        s.customOAuthAuthURL.trim(),
        s.customOAuthTokenURL.trim(),
        s.customOAuthUserinfoURL.trim(),
        scopes,
        s.customOAuthClientID.trim(),
        s.customOAuthClientSecret.trim()
      )
    } catch (err) {
      console.error('Failed to start custom OAuth flow:', err)
    }
    return
  }

  const oauthType = getOAuthProviderType(s.selectedProvider)
  if (!oauthType) return

  try {
    await oauthStore.startFlow(oauthType)
  } catch (err) {
    console.error('Failed to start OAuth flow:', err)
  }
}

export function cancelOAuthFlow() {
  oauthStore.cancelFlow()
}

export async function handleCopyOAuthLink(s: AccountFormState) {
  if (!oauthStore.authURL) return
  try {
    await navigator.clipboard.writeText(oauthStore.authURL)
    s.oauthLinkCopied = true
    if (s.oauthCopiedResetTimer) clearTimeout(s.oauthCopiedResetTimer)
    s.oauthCopiedResetTimer = setTimeout(() => { s.oauthLinkCopied = false }, 1500)
  } catch {
    toasts.error(get(_)('viewer.failedToCopy'))
  }
}

export function getOAuthButtonIcon(provider: EmailProvider | null): string {
  if (!provider) return 'mdi:login'
  const oauthType = getOAuthProviderType(provider)
  if (oauthType === 'google') return 'logos:google-icon'
  if (oauthType === 'microsoft') return 'logos:microsoft-icon'
  return 'mdi:login'
}
