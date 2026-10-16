import { contactSourcesStore, type LinkedAccountInfo } from '$lib/stores/contactSources.svelte'
import type ServerCertificatePrompt from '$lib/components/kit/ServerCertificatePrompt.svelte'
// @ts-ignore - wailsjs path
import type { carddav } from '../../../../wailsjs/go/models'
import { addToast } from '$lib/stores/toast'
import { get } from 'svelte/store'
import { _ } from '$lib/i18n'

export type SourceType = 'carddav' | 'google' | 'microsoft'

export interface ContactSourceProps {
  getOpen: () => boolean
  setOpen: (v: boolean) => void
  getEditSource: () => carddav.Source | null
  getOnClose: () => (() => void) | undefined
}

/** Shared state for ContactSourceDialog and its child sections. */
export class ContactSourceState {
  private props: ContactSourceProps

  constructor(props: ContactSourceProps) {
    this.props = props
  }

  // Lazy prop getters
  get open(): boolean {
    return this.props.getOpen()
  }
  set open(v: boolean) {
    this.props.setOpen(v)
  }
  get editSource(): carddav.Source | null {
    return this.props.getEditSource()
  }
  get onClose(): (() => void) | undefined {
    return this.props.getOnClose()
  }

  // Source type selection
  sourceType = $state<SourceType>('carddav')

  // CardDAV form state
  name = $state('')
  url = $state('')
  username = $state('')
  password = $state('')
  syncInterval = $state(60)

  // CardDAV authentication method: Basic password, or reuse a custom-OAuth mail
  // account's bearer token (unified-grant servers like Stalwart).
  cardDavAuthMethod = $state<'password' | 'oauth'>('password')
  customOAuthAccounts = $state<LinkedAccountInfo[]>([])
  selectedCustomAccountId = $state<string>('')
  loadingCustomAccounts = $state(false)
  cardDavIsOAuth = $derived(this.cardDavAuthMethod === 'oauth')

  // Discovery state
  discovering = $state(false)
  discoveredAddressbooks = $state<carddav.AddressbookInfo[]>([])
  selectedAddressbooks = $state<Set<string>>(new Set())
  discoveryError = $state<string | null>(null)
  certPrompt!: ServerCertificatePrompt
  hasDiscovered = $state(false)

  // OAuth state
  linkedAccounts = $state<LinkedAccountInfo[]>([])
  selectedAccountId = $state<string>('')
  loadingAccounts = $state(false)
  oauthInProgress = $state(false)
  oauthEmail = $state<string>('')

  // Save state
  saving = $state(false)

  // Convert between number state and string Select value
  syncIntervalStr = $derived(String(this.syncInterval))

  handleSyncIntervalChange = (value: string) => {
    this.syncInterval = parseInt(value, 10)
  }

  // Computed: is editing an OAuth source
  isEditingOAuthSource = $derived(
    this.editSource && (this.editSource.type === 'google' || this.editSource.type === 'microsoft')
  )

  // Switch CardDAV auth method, resetting discovery (different auth → re-discover).
  setCardDavAuth = (method: 'password' | 'oauth') => {
    if (this.cardDavAuthMethod === method) return
    this.cardDavAuthMethod = method
    this.hasDiscovered = false
    this.discoveredAddressbooks = []
    this.selectedAddressbooks = new Set()
    this.discoveryError = null
  }

  // Get available accounts for the selected provider (not already linked)
  availableAccounts = $derived(
    this.linkedAccounts.filter(acc => acc.provider === this.sourceType && !acc.isLinked)
  )

  // Copy-link fallback for OAuth waiting state
  oauthAuthURL = $state<string | null>(null)
  oauthLinkCopied = $state(false)
  private oauthCopiedResetTimer: ReturnType<typeof setTimeout> | null = null
  handleCopyOAuthLink = async () => {
    if (!this.oauthAuthURL) return
    try {
      await navigator.clipboard.writeText(this.oauthAuthURL)
      this.oauthLinkCopied = true
      if (this.oauthCopiedResetTimer) clearTimeout(this.oauthCopiedResetTimer)
      this.oauthCopiedResetTimer = setTimeout(() => { this.oauthLinkCopied = false }, 1500)
    } catch {
      addToast({ type: 'error', message: get(_)('viewer.failedToCopy') })
    }
  }

  toggleAddressbook = (path: string) => {
    const newSet = new Set(this.selectedAddressbooks)
    if (newSet.has(path)) {
      newSet.delete(path)
    } else {
      newSet.add(path)
    }
    this.selectedAddressbooks = newSet
  }

  handleCancel = () => {
    if (this.oauthInProgress) {
      contactSourcesStore.cancelOAuthFlow()
    }
    this.open = false
    this.onClose?.()
  }

  handleOpenChange = (isOpen: boolean) => {
    this.open = isOpen
    if (!isOpen) {
      if (this.oauthInProgress) {
        contactSourcesStore.cancelOAuthFlow()
      }
      this.onClose?.()
    }
  }

  handleTabChange = (value: string) => {
    this.sourceType = value as SourceType
    // Reset OAuth state when switching tabs
    this.selectedAccountId = ''
    this.oauthEmail = ''
    this.oauthInProgress = false
  }

  // Check if save is enabled
  canSave = $derived(() => {
    if (this.sourceType === 'carddav') {
      return this.hasDiscovered && this.selectedAddressbooks.size > 0 && this.name
    }
    if (this.editSource && this.isEditingOAuthSource) {
      return !!this.name
    }
    return (this.selectedAccountId || this.oauthEmail) && this.name
  })
}
