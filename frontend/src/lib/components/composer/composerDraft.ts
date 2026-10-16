// Draft autosave: debounced scheduling, serialized saves and deletion.
import { DRAFT_SAVE_DELAY } from './composerTypes'
import { hasContent, getContentHash } from './composerContent'
import { buildMessage } from './composerMessage'
import type { ComposerState } from './composerState.svelte'

// Schedule a draft save (debounced)
// Note: All expensive operations (hasContent, getContentHash) are inside the timeout
// to avoid lag on every keystroke
export function scheduleDraftSave(s: ComposerState) {
  // Clear any pending save
  if (s.saveTimeoutId) {
    clearTimeout(s.saveTimeoutId)
  }

  // Reset indicator immediately when content changes (makes it disappear on input)
  if (s.saveStatus === 'saved') {
    s.saveStatus = 'idle'
  }

  s.saveTimeoutId = setTimeout(async () => {
    // Only save if there's content
    if (!hasContent(s)) {
      return
    }

    // Check if content actually changed
    const currentHash = getContentHash(s)
    if (currentHash === s.lastContent) {
      return
    }

    await saveDraft(s)
  }, DRAFT_SAVE_DELAY)
}

// Actually save the draft. Returns false only when the save failed.
// forSend saves while a send is claimed, so the queued message has a draft.
export async function saveDraft(s: ComposerState, forSend = false): Promise<boolean> {
  // Wait out an in-flight save rather than dropping this one, so the
  // newest edits still get saved. No await sits between this loop and
  // isSaving = true, so concurrent callers can't both start a save.
  while (s.isSaving) await s.savingComplete

  if (s.discarding || (s.sending && !forSend)) return true
  if (!hasContent(s)) return true

  // Check again for content changes before saving
  const currentHash = getContentHash(s)
  if (currentHash === s.lastContent && s.currentDraftId) {
    return true  // No changes since last save
  }

  let resolveSaving: () => void
  s.savingComplete = new Promise<void>(resolve => { resolveSaving = resolve })

  s.isSaving = true
  s.saveStatus = 'saving'
  try {
    const message = buildMessage(s)
    const result = await s.api.saveDraft(s.activeAccountId, message, s.currentDraftId || '')
    s.currentDraftId = result.id
    s.lastContent = currentHash
    s.saveStatus = 'saved'
    s.syncStatus = result.syncStatus as 'pending' | 'synced' | 'failed'
    s.lastSavedAt = new Date()
    return true
  } catch (err) {
    console.error('Failed to save draft:', err)
    s.saveStatus = 'error'
    return false
  } finally {
    s.isSaving = false
    resolveSaving!()
  }
}

// Delete the current draft
export async function deleteDraft(s: ComposerState) {
  if (!s.currentDraftId) return

  try {
    await s.api.deleteDraft(s.currentDraftId)
    s.currentDraftId = null
  } catch (err) {
    console.error('Failed to delete draft:', err)
  }
}
