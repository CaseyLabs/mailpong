// Composer startup: identity, account and security loading, then reply/forward/draft seeding.
// @ts-ignore - Wails generated imports
import { account, smtp } from '../../../../wailsjs/go/models'
// @ts-ignore - Wails runtime for events
import { EventsOn } from '../../../../wailsjs/runtime/runtime.js'
import { stripParagraphStyles } from './composerUtils'
import { hasSignatureMarker } from './composerSignature'
import { isImageAllowedSync } from '$lib/stores/imageAllowlist.svelte'
import { getAlwaysLoadImages } from '$lib/stores/settings.svelte'
import { getDisplayMode, getContentHash } from './composerContent'
import { updateSecurityForIdentity } from './composerSecurity'
import { applyReadReceiptPolicy, selectIdentityForReply, appendSignatureForIdentity, defaultReplyToFor } from './composerIdentity'
import { loadComposerImages, ensureEditor } from './composerEditorMode'
import type { ComposerState } from './composerState.svelte'

export async function initComposer(s: ComposerState) {
  // Load identities — try cross-account first (main window), fall back to single-account (detached)
  try {
    // When replying or forwarding on a no-outgoing account, honor that
    // account's configured Reply/Forward-with identity (if any). Captured
    // from the pre-filter list because no-outgoing accounts are removed
    // before the picker sees them.
    let replyForwardIdentity: account.Identity | null = null

    if (s.api.getAllAccountIdentities) {
      const groups = await s.api.getAllAccountIdentities()
      const sourceGroup = (groups || []).find(g => g.account?.id === s.accountId)
      // Exclude receive-only accounts: their identities can't actually
      // be used as a From address, so they don't belong in the picker.
      s.allGroups = (groups || []).filter(g => !g.account?.noOutgoingServer)
      s.identities = s.allGroups.flatMap(g => g.identities || [])

      const sourceReplyForwardId = sourceGroup?.account?.noOutgoingServer
        ? ((sourceGroup.account as any).replyForwardIdentityId || '')
        : ''
      if (sourceReplyForwardId) {
        replyForwardIdentity = s.identities.find(i => i.id === sourceReplyForwardId) || null
      }
    }
    if (!s.api.getAllAccountIdentities) {
      // Detached window — single account only
      s.identities = await s.api.getIdentities(s.accountId)
    }

    // Select identity: explicit recipient match wins; then the source
    // account's Reply/Forward-with preference (no-outgoing accounts
    // only); then this account's default identity; then the first
    // available identity as the ultimate fallback.
    const matchedIdentity = selectIdentityForReply(s)
    const accountIdentities = s.identities.filter(i => i.accountId === s.accountId)
    const defaultIdentity = accountIdentities.find(i => i.isDefault) || accountIdentities[0]
    const selectedIdentity = matchedIdentity || replyForwardIdentity || defaultIdentity || s.identities[0]
    if (selectedIdentity) {
      s.selectedIdentityId = selectedIdentity.id
    }
  } catch (err) {
    console.error('Failed to load identities:', err)
  }

  // Load account's read receipt request policy (use activeAccountId which derives from selected identity)
  try {
    const acc = await s.api.getAccount(s.activeAccountId)
    applyReadReceiptPolicy(s, acc.readReceiptRequestPolicy || 'never')
  } catch (err) {
    console.error('Failed to load account settings:', err)
  }

  // Load S/MIME and PGP availability for the selected identity's email
  {
    const selectedIdentity = s.identities.find(i => i.id === s.selectedIdentityId)
    if (selectedIdentity) {
      await updateSecurityForIdentity(s, selectedIdentity.email)
    }
  }

  // Initialize TipTap editor (no-op in plain text mode — its element isn't
  // mounted yet; created lazily when the user switches to rich text).
  ensureEditor(s)

  // Initialize from initialMessage if provided (reply/forward)
  if (s.initialMessage) {
    initializeFromMessage(s)
    // Store initial content hash so we don't immediately save
    s.lastContent = getContentHash(s)
  }

  // Prefill the account's default CC, BCC, and Reply-To (#341) —
  // new/reply/forward only; a loaded draft's saved recipients are the truth
  if (!s.draftId) {
    let prefilled = false
    try {
      const defaultCc = await s.api.getDefaultAddress('cc', s.activeAccountId)
      const existing = new Set(
        [...s.toRecipients, ...s.ccRecipients, ...s.bccRecipients]
          .map(r => (r.address || (r as any).email || '').toLowerCase())
      )
      const added = defaultCc
        .split(/[,;]/)
        .map(part => part.trim())
        .filter(addr => addr && !existing.has(addr.toLowerCase()))
        .map(addr => new smtp.Address({ name: '', address: addr }))
      if (added.length > 0) {
        s.ccRecipients = [...s.ccRecipients, ...added]
        s.showCc = true
        prefilled = true
      }
    } catch (err) {
      console.error('Failed to load default CC:', err)
    }
    try {
      const defaultBcc = await s.api.getDefaultAddress('bcc', s.activeAccountId)
      const existing = new Set(
        [...s.toRecipients, ...s.ccRecipients, ...s.bccRecipients]
          .map(r => (r.address || (r as any).email || '').toLowerCase())
      )
      const added = defaultBcc
        .split(/[,;]/)
        .map(part => part.trim())
        .filter(addr => addr && !existing.has(addr.toLowerCase()))
        .map(addr => new smtp.Address({ name: '', address: addr }))
      if (added.length > 0) {
        s.bccRecipients = [...s.bccRecipients, ...added]
        s.showBcc = true
        prefilled = true
      }
    } catch (err) {
      console.error('Failed to load default BCC:', err)
    }
    try {
      const first = await defaultReplyToFor(s, s.identities.find(i => i.id === s.selectedIdentityId))
      if (first && s.replyToRecipients.length === 0) {
        s.replyToRecipients = [new smtp.Address({ name: '', address: first })]
        s.prefilledReplyTo = first
        s.showReplyTo = true
        prefilled = true
      } else if (first && s.replyToRecipients.length === 1 && s.replyToRecipients[0].address === first) {
        // Restored from a draft: still the identity's default, so a From change may swap it
        s.prefilledReplyTo = first
      }
    } catch (err) {
      console.error('Failed to load default Reply-To:', err)
    }
    if (prefilled) {
      // The prefills aren't user edits — don't let them trigger an autosave
      s.lastContent = getContentHash(s)
    }
  }

  // Append signature for the selected identity (after editor is ready)
  // Only if signature doesn't already exist in content (e.g., from loaded draft)
  // Then focus the To field once everything is initialized
  setTimeout(() => {
    const identity = s.identities.find(i => i.id === s.selectedIdentityId)
    if (identity) {
      const content = s.editor?.getHTML() || ''
      // Don't append if signature marker already exists in the user's compose area.
      // Only check content before the quoted section — markers inside quoted history
      // (from previous replies with signatures) should not prevent injection.
      const quoteIdx = content.indexOf('<blockquote')
      const wroteIdx = content.search(/wrote:\s*(<br[^>]*>)?\s*<\/p>/i)
      const fwdIdx = content.indexOf('---------- Forwarded message ----------')
      const boundaries = [quoteIdx, wroteIdx, fwdIdx].filter(i => i > -1)
      const quoteBoundary = boundaries.length > 0 ? Math.min(...boundaries) : content.length
      const preQuoteContent = content.substring(0, quoteBoundary)
      if (!hasSignatureMarker(preQuoteContent)) {
        appendSignatureForIdentity(s, identity)
      }
    }
    // Focus editor body for reply/reply-all, To field for new/forward
    const mode = getDisplayMode(s)
    switch (mode) {
      case 'reply':
      case 'reply-all':
        // Plain text mode shows its own editor (the rich editor is hidden),
        // so focus that — cursor at the top, above the quoted original.
        if (s.isPlainTextMode) {
          s.plainTextRef?.focus()
          break
        }
        s.editor?.commands.focus('start')
        break
      default:
        s.toInputRef?.focus()
    }
  }, 50)

  // Listen for draft sync status changes from backend
  EventsOn('draft:syncStatusChanged', (data: { draftId: string, syncStatus: string, imapUid: number, error: string }) => {
    if (data.draftId === s.currentDraftId) {
      s.syncStatus = data.syncStatus as 'pending' | 'synced' | 'failed'
    }
  })
}

// Helper to ensure proper smtp.Address object (handles both 'address' and 'email' field names)
export function toSmtpAddress(addr: any): smtp.Address {
  if (!addr) return new smtp.Address({ name: '', address: '' })
  return new smtp.Address({
    name: addr.name || '',
    address: addr.address || addr.email || ''
  })
}

// Initialize composer fields from the pre-built message (from backend)
export function initializeFromMessage(s: ComposerState) {
  if (!s.initialMessage) return

  // Set recipients - ensure proper smtp.Address objects
  // The backend returns smtp.Address with 'address' field, but we need to handle
  // any edge cases where plain objects come through
  s.toRecipients = (s.initialMessage.to || []).map(toSmtpAddress)
  s.ccRecipients = (s.initialMessage.cc || []).map(toSmtpAddress)
  s.bccRecipients = (s.initialMessage.bcc || []).map(toSmtpAddress)

  // Show Cc/Bcc fields if they have recipients
  if (s.ccRecipients.length > 0) {
    s.showCc = true
  }
  if (s.bccRecipients.length > 0) {
    s.showBcc = true
  }

  // Restore Reply-To (single address; from a reopened draft)
  if (s.initialMessage.reply_to?.address) {
    s.replyToRecipients = [toSmtpAddress(s.initialMessage.reply_to)]
    s.showReplyTo = true
  }

  // Set subject
  s.subject = s.initialMessage.subject || ''

  // Set threading headers
  s.inReplyTo = s.initialMessage.in_reply_to
  s.references = s.initialMessage.references || []

  // Restore attachments and inline images from draft/reply/forward
  // Go []byte is serialized as base64 string via JSON, but TS type says number[]
  // content_base64 is used for efficient Wails RPC transfer (inline images in replies/forwards)
  let htmlBody = s.initialMessage.html_body || ''
  if (s.initialMessage.attachments?.length > 0) {
    for (const att of s.initialMessage.attachments) {
      const base64Data = att.content_base64 || (att.content as unknown as string)
      if (!base64Data) continue

      if (att.inline && att.content_id) {
        // Inline image - restore to inlineImages array and replace CID with data URL
        const dataUrl = `data:${att.content_type};base64,${base64Data}`
        s.inlineImages = [...s.inlineImages, {
          cid: att.content_id,
          dataUrl,
          contentType: att.content_type,
          data: base64Data,
          filename: att.filename,
        }]
        htmlBody = htmlBody.replaceAll(`cid:${att.content_id}`, dataUrl)
      } else if (!att.inline) {
        // Regular attachment
        s.attachments = [...s.attachments, {
          filename: att.filename,
          contentType: att.content_type,
          size: base64Data.length,
          data: base64Data,
        }]
      }
    }
    // Ensure new inline images get unique CIDs
    s.inlineImageCounter = Math.max(s.inlineImageCounter, s.inlineImages.length)
  }

  // Open drafts in the format they were written in (#420): a plain-text
  // draft saves html_body = '', so seeding the rich editor from it opens a
  // blank body — and autosave would then wipe the real text_body. The
  // reverse (rich draft under a plain setting) opens rich so formatting
  // and inline images survive. Reply/forward keeps following the setting
  // (#285 behavior). Both editor surfaces stay mounted, so flipping the
  // flag is safe in either direction.
  if (s.draftId && !htmlBody && s.initialMessage.text_body) {
    s.isPlainTextMode = true
  }
  if (s.draftId && htmlBody) {
    s.isPlainTextMode = false
  }

  // Set editor content (with restored data URLs for inline images)
  // Strip email-client paragraph styles so TipTap doesn't double-space empty lines
  if (s.editor && htmlBody) {
    s.editor.commands.setContent(stripParagraphStyles(htmlBody), { emitUpdate: false })
    // Move cursor to beginning (before the quoted content)
    s.editor.commands.focus('start')
  }

  // Keep the original rich quote/forward body (images already restored) so a
  // later plain -> rich switch can reprocess the original instead of the
  // lossy plain-text conversion (which permanently drops images).
  s.initialRichBody = htmlBody

  // Plaintext mode shows a textarea bound to plainTextContent (not the editor),
  // so seed it with the backend's plaintext quote — otherwise replies/forwards
  // open with an empty body when the composer defaults to plaintext.
  if (s.isPlainTextMode) {
    s.plainTextContent = s.initialMessage.text_body || ''
  }

  // Check for blocked remote images in quoted content.
  // If the sender is allowlisted or always-load is enabled, unblock immediately.
  if (htmlBody.includes('data-original-src')) {
    s.composerImagesBlocked = true
    const senderEmail = ((s.initialMessage.from as any)?.address || (s.initialMessage.from as any)?.email || '').toLowerCase()
    if (getAlwaysLoadImages() || s.imagesLoaded || (senderEmail && isImageAllowedSync(senderEmail))) {
      // Use setTimeout to ensure editor has rendered the content first
      setTimeout(() => loadComposerImages(s), 0)
    }
  }

  // Restore S/MIME toggles from draft
  if (s.initialMessage.sign_message) {
    s.signMessage = true
  }
  if (s.initialMessage.encrypt_message) {
    s.encryptMessage = true
  }

  // Restore PGP toggles from draft
  if ((s.initialMessage as any).pgp_sign_message) {
    s.pgpSignMessage = true
  }
  if ((s.initialMessage as any).pgp_encrypt_message) {
    s.pgpEncryptMessage = true
  }
}
