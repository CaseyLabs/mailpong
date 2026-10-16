// Sending: validation, confirmation dialogs, the send pipeline and pop-out.
import { get } from 'svelte/store'
import { _ } from '$lib/i18n'
import { addToast } from '$lib/stores/toast'
import { queueWithUndo } from './undoSend.svelte'
import { bodyMentionsAttachment, getDisplayMode } from './composerContent'
import { buildMessage } from './composerMessage'
import { scheduleDraftSave, saveDraft, deleteDraft } from './composerDraft'
import type { ComposerState } from './composerState.svelte'

// Pre-send validation - returns true if we should proceed, false if waiting for confirmation
export function validateBeforeSend(s: ComposerState): boolean {
  // Block send if encrypt is on but recipients are missing certs
  if (s.encryptMessage && s.missingCertRecipients.length > 0) {
    addToast({
      type: 'error',
      message: get(_)('composer.cannotEncryptMissingCert', { values: { emails: s.missingCertRecipients.join(', ') } }),
    })
    return false
  }

  // Block send if PGP encrypt is on but recipients are missing keys
  if (s.pgpEncryptMessage && s.missingPGPKeyRecipients.length > 0) {
    addToast({
      type: 'error',
      message: get(_)('composer.cannotEncryptMissingPGPKey', { values: { emails: s.missingPGPKeyRecipients.join(', ') } }),
    })
    return false
  }

  // Check for missing attachment
  if (s.attachments.length === 0 && bodyMentionsAttachment(s)) {
    s.showMissingAttachmentDialog = true
    return false
  }

  // Check for empty subject
  if (!s.subject.trim()) {
    s.showEmptySubjectDialog = true
    return false
  }

  return true
}

export async function handleSend(s: ComposerState) {
  if (s.sending || s.poppingOut) return
  if (s.toRecipients.length === 0) {
    addToast({
      type: 'error',
      message: get(_)('composer.noRecipients'),
    })
    return
  }

  const selectedIdentity = s.identities.find(i => i.id === s.selectedIdentityId)
  if (!selectedIdentity) {
    addToast({
      type: 'error',
      message: get(_)('composer.selectSenderIdentity'),
    })
    return
  }

  // Run validations that may show confirmation dialogs
  if (!validateBeforeSend(s)) {
    return
  }

  await doSend(s)
}

// Actually send the message (called directly or after confirmation)
export function doSend(s: ComposerState): Promise<void> {
  // Claim the send before any await so a second Ctrl+Enter or click can't
  // start another one.
  if (s.sending) return s.sendDone
  s.sending = true
  s.sendDone = sendClaimed(s)
  return s.sendDone
}

export async function sendClaimed(s: ComposerState) {
  // Cancel any pending draft save
  if (s.saveTimeoutId) {
    clearTimeout(s.saveTimeoutId)
    s.saveTimeoutId = null
  }

  try {
    if (s.isDetached && !(await s.countdown.run())) {
      scheduleDraftSave(s) // edits made during the countdown
      return
    }
    // Wait for any in-flight draft save to complete before sending
    await s.savingComplete
    const message = buildMessage(s)
    const evidence = await s.api.getSenderEvidence(s.activeAccountId, message.from.address)
    if (evidence.action === 'block') throw new Error(evidence.guidance)
    if (evidence.action === 'review') {
      const details = `${message.from.address} — ${evidence.state}
${evidence.source}
${evidence.route}
${evidence.observedAt || get(_)('composer.senderNotObserved')}
${evidence.guidance}`
      if (!(await s.senderReview.ask(details))) { scheduleDraftSave(s); return }
      message.sender_review_token = evidence.reviewToken
    }
    message.submission_draft_id = s.currentDraftId || ''
    const previous = await s.api.getSubmissionReview(s.activeAccountId, message, s.currentDraftId || '')
    if (previous) {
      if (!(await s.submissionReview.ask(previous.outcome))) { scheduleDraftSave(s); return }
      message.review_attempt_id = previous.id
    }
    // Main window: queue behind the undo delay. That needs a draft to hold
    // the message; without one, send at once.
    if (s.api.queueMessage && s.onUndoSend && (await saveDraft(s, true)) && s.currentDraftId) {
      await queueWithUndo(s.api.queueMessage, s.activeAccountId, message, s.currentDraftId, s.onUndoSend)
      // The queue owns the draft now; closing must not touch it.
      s.currentDraftId = null
      s.onSent?.()
      s.onClose?.()
      return
    }
    await s.api.sendMessage(s.activeAccountId, message)
    // Reading cleanup status cannot turn accepted mail into a failed send.
    try {
      const accepted = await s.api.getSubmissionReview(s.activeAccountId, message, s.currentDraftId || '')
      if (accepted?.warnings) addToast({ type: 'info', message: accepted.warnings + ' ' + get(_)('composer.acceptedCleanupWarning') })
    } catch (err) { console.error('Failed to read submission cleanup status:', err) }

    // Delete the draft on successful send (fire-and-forget - don't block UI)
    if (s.currentDraftId) {
      deleteDraft(s).catch(err => console.error('Failed to delete draft after send:', err))
    }

    addToast({
      type: 'success',
      message: get(_)('composer.messageSent'),
    })

    s.onSent?.()
    s.onClose?.()
  } catch (err) {
    console.error('Failed to send message:', err)
    scheduleDraftSave(s) // review/preflight may have cancelled the pending autosave
    addToast({
      type: 'error',
      message: String(err),
    })
  } finally {
    s.sending = false
  }
}

// Handlers for confirmation dialogs
export function handleConfirmEmptySubject(s: ComposerState) {
  s.showEmptySubjectDialog = false
  // Check for missing attachment next (if applicable)
  if (s.attachments.length === 0 && bodyMentionsAttachment(s)) {
    s.showMissingAttachmentDialog = true
  } else {
    doSend(s)
  }
}

export function handleConfirmMissingAttachment(s: ComposerState) {
  s.showMissingAttachmentDialog = false
  doSend(s)
}

// Pop out to detached window
export async function handlePopOut(s: ComposerState) {
  if (!s.api.openComposerWindow) {
    // Not available in detached windows
    return
  }

  s.poppingOut = true

  try {
    // Save draft first to get a draft ID
    const message = buildMessage(s)
    const result = await s.api.saveDraft(s.activeAccountId, message, s.currentDraftId || '')
    const savedDraftId = result.id

    // Open detached composer window with the active account
    await s.api.openComposerWindow(
      s.activeAccountId,
      getDisplayMode(s),
      s.messageId || '',
      savedDraftId
    )

    // Close this modal/inline composer
    s.onClose?.()
  } catch (err) {
    console.error('Failed to pop out composer:', err)
    addToast({
      type: 'error',
      message: get(_)('composer.failedToOpenComposer'),
    })
    s.poppingOut = false
  }
}
