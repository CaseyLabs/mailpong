// Reactive state of the account form. AccountForm.svelte creates one
// AccountFormState; the accountForm*.ts modules and child components take it.
import { oauthStore } from '$lib/stores/oauth.svelte'
import type { EmailProvider, OAuthProvider } from '$lib/config/providers'
// @ts-ignore - wailsjs path
import type { account, certificate, app } from '../../../../../wailsjs/go/models'
import type { OAuthCredentials } from './accountFormTypes'

/** The form's props, read lazily so they stay reactive. */
export interface AccountFormEnv {
  /** Account to edit (null for new account) */
  readonly editAccount: account.Account | null
  /** Callback when form is submitted successfully */
  readonly onSubmit?: (config: account.AccountConfig, oauthCredentials?: OAuthCredentials) => Promise<void>
  /** Callback when form is cancelled */
  readonly onCancel?: () => void
}

export class AccountFormState {
  constructor(private readonly env: AccountFormEnv) {}

  get editAccount() { return this.env.editAccount }
  get onSubmit() { return this.env.onSubmit }
  get onCancel() { return this.env.onCancel }

  // Form state
  step = $state<'provider' | 'details'>('provider')
  selectedProvider = $state<EmailProvider | null>(null)
  showAdvanced = $state(false)

  // OAuth state
  authMethod = $state<'password' | 'oauth2'>('password')
  oauthConfigured = $state<Record<OAuthProvider, boolean>>({
    google: false,
    microsoft: false,
  })
  oauthInitialized = $state(false)
  microsoftAccountType = $state('')

  // Custom ("bring your own app") OAuth — for a generic IMAP account whose OAuth
  // provider Mailven does not ship. Primary inputs are the issuer URL + client ID;
  // OIDC discovery resolves the endpoints. Manual endpoint entry is an advanced
  // fallback. IMAP/SMTP server settings come from the advanced section.
  customOAuthIssuer = $state('')
  customOAuthClientID = $state('')
  customOAuthClientSecret = $state('')
  customOAuthScopes = $state('')
  // Resolved endpoints — from discovery, or typed directly in manual mode.
  customOAuthAuthURL = $state('')
  customOAuthTokenURL = $state('')
  customOAuthUserinfoURL = $state('')
  customOAuthManual = $state(false)
  customOAuthDiscovering = $state(false)
  customOAuthDiscoverError = $state('')
  customOAuthDiscovered = $state(false)
  customOAuthReady = $derived(
    this.customOAuthAuthURL.trim() !== '' &&
    this.customOAuthTokenURL.trim() !== '' &&
    this.customOAuthClientID.trim() !== ''
  )
  // Lock the input fields once the flow is mid-sign-in or already succeeded.
  customOAuthInputsLocked = $derived(
    oauthStore.flowState === 'pending' || oauthStore.flowState === 'success'
  )
  // Loopback redirect the user registers in their OAuth app. The port is assigned
  // dynamically per RFC 8252 (this is why Mailven's shipped Google/Microsoft flows
  // also use a dynamic port), so it's shown as a pattern for regex-matching
  // providers; strict-match providers need the fixed-port option instead.
  readonly customOAuthRedirectURI = 'http://localhost:[0-9]+/callback'

  // Form fields
  name = $state('')
  displayName = $state('')
  color = $state('')
  email = $state('')
  prevEmail = $state('')
  username = $state('')
  password = $state('')
  imapHost = $state('')
  imapPort = $state(993)
  imapSecurity = $state<string>('tls')
  imapAuthMechanism = $state('auto')
  smtpHost = $state('')
  smtpPort = $state(587)
  smtpSecurity = $state<string>('starttls')
  noOutgoingServer = $state(false)
  smtpUsername = $state('')
  smtpPassword = $state('')
  smtpAuthMechanism = $state('auto')
  smtpUseSameAsIncoming = $state(true)
  // Auto-mirror IMAP host into SMTP host for new Generic accounts: most
  // providers use the same hostname or a near-identical subdomain swap,
  // so typing the IMAP host pre-fills SMTP. Goes sticky-off the moment
  // the user types directly into the SMTP field — manual edits stick.
  smtpHostMirrorsImap = $state(true)
  replyForwardIdentityID = $state('')
  availableIdentityGroups = $state<app.AccountIdentityGroup[]>([])
  // True only when the user explicitly picked Generic/Custom (or the
  // detector fell back to it). The "Same as incoming server" toggle is
  // gated on this; pre-configured providers always reuse IMAP creds.
  isGenericProvider = $derived(this.selectedProvider?.id === 'custom' || this.selectedProvider?.id === 'generic')
  syncPeriodDays = $state<string>('180')
  syncInterval = $state<string>('30') // Default: 30 minutes
  readReceiptRequestPolicy = $state<string>('never')

  // UI state
  testing = $state(false)
  testResult = $state<{ success: boolean; message: string } | null>(null)
  submitting = $state(false)
  errors = $state<Record<string, string>>({})
  initialized = $state(false)

  // Certificate TOFU state
  showCertDialog = $state(false)
  pendingCertificate = $state<certificate.CertificateInfo | null>(null)

  // Folder mapping state
  showFolderMapping = $state(false)
  loadingFolders = $state(false)
  availableFolders = $state<any[]>([])
  autoDetectedFolders = $state<Record<string, string>>({})

  // Folder mapping values
  sentFolderPath = $state('')
  draftsFolderPath = $state('')
  trashFolderPath = $state('')
  spamFolderPath = $state('')
  archiveFolderPath = $state('')
  allMailFolderPath = $state('')
  starredFolderPath = $state('')

  // Copy-link fallback for OAuth waiting state — used when the browser fails
  // to open and the user needs to paste the URL manually.
  oauthLinkCopied = $state(false)
  oauthCopiedResetTimer: ReturnType<typeof setTimeout> | null = null
}
