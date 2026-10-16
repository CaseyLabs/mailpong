// Reactive state of the full composer. Composer.svelte creates one
// ComposerState; the composer*.ts modules take it as their first argument.
import type { Editor } from '@tiptap/core'
// @ts-ignore - Wails generated imports
import { smtp, account, app } from '../../../../wailsjs/go/models'
import type { ComposerApi } from '$lib/composerApi'
import { getComposerFormat } from '$lib/stores/settings.svelte'
import { SubmissionReview } from './submissionReview.svelte'
import { SendCountdown } from './undoSend.svelte'
import type { ComposerAttachment, InlineImage } from './composerTypes'

/** The composer's props, read lazily so they stay reactive. */
export interface ComposerEnv {
  readonly accountId: string
  /** Pre-populated message from backend (for reply/forward), or null for new message */
  readonly initialMessage: smtp.ComposeMessage | null
  /** Existing draft ID if editing a draft */
  readonly draftId: string | null
  /** Original message ID for reply/forward (needed for pop-out) */
  readonly messageId: string | null
  readonly onClose?: () => void
  readonly onSent?: () => void
  /** Resolved API (prop, context or main window API) */
  readonly api: ComposerApi
  /** Whether this composer is in a detached window (hides pop-out button) */
  readonly isDetached: boolean
  /** Signal from parent (detached window) to trigger close flow */
  readonly closeRequested: boolean
  /** Callback when close request has been handled */
  readonly onCloseHandled?: () => void
  /** Callback when recipient or subject changes (for dynamic window title) */
  readonly onTitleChange?: (to: string, subject: string) => void
  /** Whether remote images were loaded in the viewer before reply/forward */
  readonly imagesLoaded: boolean
  /** Main window: Undo on a queued send hands the message back (with the draft holding it) */
  readonly onUndoSend?: (accountId: string, message: smtp.ComposeMessage, draftId: string) => void
}

export class ComposerState {
  constructor(private readonly env: ComposerEnv) {}

  get accountId() { return this.env.accountId }
  get initialMessage() { return this.env.initialMessage }
  get draftId() { return this.env.draftId }
  get messageId() { return this.env.messageId }
  get onClose() { return this.env.onClose }
  get onSent() { return this.env.onSent }
  get api() { return this.env.api }
  get isDetached() { return this.env.isDetached }
  get closeRequested() { return this.env.closeRequested }
  get onCloseHandled() { return this.env.onCloseHandled }
  get onTitleChange() { return this.env.onTitleChange }
  get imagesLoaded() { return this.env.imagesLoaded }
  get onUndoSend() { return this.env.onUndoSend }

  // State
  allGroups = $state<app.AccountIdentityGroup[]>([])  // All accounts + identities (main window only)
  identities = $state<account.Identity[]>([])  // Flat list of all identities (union)
  selectedIdentityId = $state<string>('')

  // Derive the active account ID from the selected identity's accountId.
  // Falls back to the prop accountId if no identity is selected yet.
  activeAccountId = $derived.by(() => {
    if (!this.selectedIdentityId) return this.accountId
    const identity = this.identities.find(i => i.id === this.selectedIdentityId)
    return identity?.accountId || this.accountId
  })
  toRecipients = $state<smtp.Address[]>([])
  ccRecipients = $state<smtp.Address[]>([])
  bccRecipients = $state<smtp.Address[]>([])
  subject = $state('')
  showCc = $state(false)
  showBcc = $state(false)
  showReplyTo = $state(false)
  // Reply-To is a single *Address in ComposeMessage — the first chip wins
  replyToRecipients = $state<smtp.Address[]>([])
  // The Reply-To address prefilled from the identity or account default, so a
  // From change can swap it without touching a value the user edited.
  prefilledReplyTo = ''
  sending = $state(false)
  poppingOut = $state(false)  // Pop-out in progress
  editorElement = $state<HTMLElement | null>(null)
  editor = $state<Editor | null>(null)

  // Track In-Reply-To and References for threading
  inReplyTo = $state<string | undefined>(undefined)
  references = $state<string[]>([])

  // Attachments
  attachments = $state<ComposerAttachment[]>([])
  isDraggingOver = $state(false)

  // Inline images (embedded in HTML body)
  inlineImages = $state<InlineImage[]>([])
  inlineImageCounter = 0  // Counter for generating unique CIDs

  // Read receipt request
  requestReadReceipt = $state(false)
  showReadReceiptOption = $state(false)  // Show checkbox when policy is 'ask'

  // S/MIME signing
  signMessage = $state(false)
  showSignOption = $state(false)  // Only show if account has a cert

  // S/MIME encryption
  encryptMessage = $state(false)
  showEncryptOption = $state(false)  // Only show if account has a cert
  recipientCertStatus = $state<Record<string, boolean>>({})
  missingCertRecipients = $derived.by(() => {
    if (!this.encryptMessage) return []
    const allRecipients = [...this.toRecipients, ...this.ccRecipients, ...this.bccRecipients]
    return allRecipients
      .map(r => r.address)
      .filter(email => email && this.recipientCertStatus[email] === false)
  })

  // PGP signing
  pgpSignMessage = $state(false)
  showPGPSignOption = $state(false)  // Only show if account has a PGP key

  // PGP encryption
  pgpEncryptMessage = $state(false)
  showPGPEncryptOption = $state(false)  // Only show if account has a PGP key
  recipientPGPKeyStatus = $state<Record<string, boolean>>({})
  missingPGPKeyRecipients = $derived.by(() => {
    if (!this.pgpEncryptMessage) return []
    const allRecipients = [...this.toRecipients, ...this.ccRecipients, ...this.bccRecipients]
    return allRecipients
      .map(r => r.address)
      .filter(email => email && this.recipientPGPKeyStatus[email] === false)
  })

  // Identity-aware cert/key info (for display in security bars)
  smimeCertFingerprint = $state<string>('')  // First 8 hex chars of fingerprint
  pgpKeyId = $state<string>('')  // Last 8 hex chars of fingerprint (short key ID)

  // Security mode for keyboard shortcuts (Alt+P / Alt+S activate, then s/e toggle sign/encrypt)
  securityMode = $state<'pgp' | 'smime' | null>(null)

  // Plain text mode toggle (default from user setting, can be toggled per-message)
  isPlainTextMode = $state(getComposerFormat() === 'plain')
  plainTextContent = $state('')  // Store plain text when in plain text mode
  plainTextRef = $state<{ focus: () => void } | null>(null)  // PlainTextEditor instance (plain text mode)
  initialRichBody = ''  // original reply/forward rich body (images restored), for plain->rich reprocess

  // Component refs
  toolbarRef = $state<{ focus: () => void } | null>(null)
  toInputRef = $state<{ focus: () => void } | null>(null)

  // Draft auto-save state
  currentDraftId = $state<string | null>(null)
  saveStatus = $state<'idle' | 'saving' | 'saved' | 'error'>('idle')

  certCheckTimeout: ReturnType<typeof setTimeout> | null = null
  pgpKeyCheckTimeout: ReturnType<typeof setTimeout> | null = null

  syncStatus = $state<'pending' | 'synced' | 'failed'>('pending') // IMAP sync status
  lastSavedAt = $state<Date | null>(null)
  saveTimeoutId: ReturnType<typeof setTimeout> | null = null
  lastContent = ''  // Track content changes to avoid unnecessary saves

  // Guard to prevent concurrent save requests (which cause orphaned drafts)
  isSaving = false
  discarding = false
  savingComplete: Promise<void> = Promise.resolve()

  // Whether remote images are blocked in the composer's quoted content
  composerImagesBlocked = $state(false)

  submissionReview = new SubmissionReview()
  senderReview = new SubmissionReview()

  // Confirmation dialogs state
  showEmptySubjectDialog = $state(false)
  showMissingAttachmentDialog = $state(false)
  showFlatpakDndDialog = $state(false)
  showCloseConfirm = $state(false)
  closeLoading = $state<'discard' | 'save' | null>(null)

  // Swap the prefilled Reply-To for the new identity's; a user-edited value stays
  replyToSwapSeq = 0

  // Detached window: the countdown before a send goes out, with Undo.
  countdown = new SendCountdown()
  sendDone: Promise<void> = Promise.resolve()

  // A debounced draft save that handleClose cancelled; Keep Editing restarts it.
  closeCancelledSave = false
}
