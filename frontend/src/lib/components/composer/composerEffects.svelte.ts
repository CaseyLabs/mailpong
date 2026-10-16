// The composer's reactive effects. Call once during component initialization.
import { untrack } from 'svelte'
import type { ComposerState } from './composerState.svelte'
import { scheduleDraftSave } from './composerDraft'
import { checkRecipientCertsDebounced, checkRecipientPGPKeysDebounced } from './composerSecurity'
import { handleClose } from './composerClose'

export function registerComposerEffects(s: ComposerState) {
  // Initialize currentDraftId from prop (runs once on mount)
  $effect(() => {
    if (s.draftId && !s.currentDraftId) {
      s.currentDraftId = s.draftId
    }
  })

  // Check recipient certs when encrypt is toggled on or recipients change
  $effect(() => {
    if (!s.encryptMessage) return
    const allEmails = [...s.toRecipients, ...s.ccRecipients, ...s.bccRecipients]
      .map(r => r.address)
      .filter(Boolean)
    if (allEmails.length === 0) return
    checkRecipientCertsDebounced(s, allEmails)
  })

  // Check recipient PGP keys when encrypt is toggled on or recipients change
  $effect(() => {
    if (!s.pgpEncryptMessage) return
    const allEmails = [...s.toRecipients, ...s.ccRecipients, ...s.bccRecipients]
      .map(r => r.address)
      .filter(Boolean)
    if (allEmails.length === 0) return
    checkRecipientPGPKeysDebounced(s, allEmails)
  })

  // Watch for content changes and trigger auto-save
  $effect(() => {
    // Dependencies to watch
    const _ = [s.toRecipients, s.ccRecipients, s.bccRecipients, s.replyToRecipients, s.subject, s.signMessage, s.encryptMessage, s.pgpSignMessage, s.pgpEncryptMessage]
    // untrack prevents $effect from creating a reactive dependency on saveStatus
    // (which scheduleDraftSave reads), avoiding a circular re-run that causes flash
    untrack(() => scheduleDraftSave(s))
  })

  // Watch for close request from parent (detached window)
  $effect(() => {
    if (s.closeRequested) {
      handleClose(s)
    }
  })

  // Emit title info when recipients or subject change (for dynamic window title)
  $effect(() => {
    if (!s.onTitleChange) return
    const firstTo = s.toRecipients[0]
    const displayTo = firstTo?.name || firstTo?.address || ''
    s.onTitleChange(displayTo, s.subject)
  })
}
