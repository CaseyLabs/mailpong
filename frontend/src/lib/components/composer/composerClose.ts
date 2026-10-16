// Closing: the confirm dialog and its discard, save and keep-editing outcomes.
import { get } from 'svelte/store'
import { _ } from '$lib/i18n'
import { addToast } from '$lib/stores/toast'
import { scheduleDraftSave, saveDraft } from './composerDraft'
import type { ComposerState } from './composerState.svelte'

export function handleClose(s: ComposerState) {
  // Closing during the send countdown sends now; the window closes once sent.
  if (s.countdown.seconds > 0) {
    s.countdown.sendNow()
    s.onCloseHandled?.()
    return
  }
  // Cancel any pending draft save
  s.closeCancelledSave = s.saveTimeoutId !== null
  if (s.saveTimeoutId) {
    clearTimeout(s.saveTimeoutId)
    s.saveTimeoutId = null
  }

  // Always show confirmation dialog (even for empty content, since a draft may have been saved)
  s.showCloseConfirm = true
}

// Discard: Delete draft from local DB and IMAP, then close
export async function handleDiscardAndClose(s: ComposerState) {
  s.discarding = true
  if (s.saveTimeoutId) {
    clearTimeout(s.saveTimeoutId)
    s.saveTimeoutId = null
  }
  await s.savingComplete
  s.closeLoading = 'discard'
  try {
    if (s.currentDraftId) {
      await s.api.deleteDraft(s.currentDraftId)
      s.currentDraftId = null
    }
  } catch (err) {
    console.error('Failed to delete draft:', err)
    // Still close even if delete fails
  }
  s.showCloseConfirm = false
  s.closeLoading = null
  s.onCloseHandled?.()
  s.onClose?.()
}

// Save & Close: Save current content as draft, then close
export async function handleSaveAndClose(s: ComposerState) {
  s.closeLoading = 'save'
  const saved = await saveDraft(s)
  if (!saved) {
    // Keep the composer open so the message isn't lost.
    s.closeLoading = null
    handleKeepEditing(s)
    addToast({ type: 'error', message: get(_)('composer.failedToSaveDraft') })
    return
  }
  s.showCloseConfirm = false
  s.closeLoading = null
  s.onCloseHandled?.()
  s.onClose?.()
}

// Keep Editing: close the dialog and restart any autosave handleClose cancelled
export function handleKeepEditing(s: ComposerState) {
  s.showCloseConfirm = false
  if (s.closeCancelledSave) {
    s.closeCancelledSave = false
    scheduleDraftSave(s)
  }
  s.onCloseHandled?.()
}
