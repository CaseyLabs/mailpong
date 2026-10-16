// Keyboard shortcuts for the composer window.
import { isDialogGuardActive } from '$lib/stores/dialogGuard'
import { handleAttachFiles } from './composerAttachments'
import { handleSend, handlePopOut } from './composerSend'
import { handleClose } from './composerClose'
import type { ComposerState } from './composerState.svelte'

// Keyboard shortcuts
export function handleKeyDown(s: ComposerState, e: KeyboardEvent) {
  // Security mode key handling (must be early in handleKeyDown)
  if (s.securityMode) {
    if (e.key === 'Escape') {
      e.preventDefault()
      s.securityMode = null
      return
    }
    if (e.key === 's' || e.key === 'S') {
      e.preventDefault()
      if (s.securityMode === 'pgp' && s.showPGPSignOption) {
        s.pgpSignMessage = !s.pgpSignMessage
        if (s.pgpSignMessage) s.signMessage = false
      } else if (s.securityMode === 'smime' && s.showSignOption) {
        s.signMessage = !s.signMessage
        if (s.signMessage) s.pgpSignMessage = false
      }
      return
    }
    if (e.key === 'e' || e.key === 'E') {
      e.preventDefault()
      if (s.securityMode === 'pgp' && s.showPGPEncryptOption) {
        s.pgpEncryptMessage = !s.pgpEncryptMessage
        if (s.pgpEncryptMessage) s.encryptMessage = false
      } else if (s.securityMode === 'smime' && s.showEncryptOption) {
        s.encryptMessage = !s.encryptMessage
        if (s.encryptMessage) s.pgpEncryptMessage = false
      }
      return
    }
    // Any other key exits security mode
    s.securityMode = null
  }

  if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
    e.preventDefault()
    handleSend(s)
  }
  if (e.key === 'd' && (e.ctrlKey || e.metaKey)) {
    e.preventDefault()
    handlePopOut(s)
  }
  // Alt+T to focus toolbar (hint mode)
  if (e.key === 't' && e.altKey) {
    e.preventDefault()
    s.toolbarRef?.focus()
  }
  // Alt+A to attach files
  if (e.key === 'a' && e.altKey) {
    e.preventDefault()
    handleAttachFiles(s)
  }
  // Alt+P / Alt+S to toggle security mode
  if (e.altKey && (e.key === 'p' || e.key === 's')) {
    if (e.key === 'p' && (s.showPGPSignOption || s.showPGPEncryptOption)) {
      e.preventDefault()
      s.securityMode = s.securityMode === 'pgp' ? null : 'pgp'
      return
    }
    if (e.key === 's' && (s.showSignOption || s.showEncryptOption)) {
      e.preventDefault()
      s.securityMode = s.securityMode === 'smime' ? null : 'smime'
      return
    }
  }
  if (e.key === 'Escape') {
    // Anything modal stacked on top owns Escape — don't also close the
    // composer underneath it (which would pop the save-draft prompt, or
    // re-open the one the user just dismissed).
    //
    // bits-ui's escape layer listens on `document` and calls
    // preventDefault() without stopping propagation, so the event still
    // reaches this window-level handler afterwards. defaultPrevented is the
    // reliable signal: it persists across listeners in the same dispatch.
    // A DOM check for [role=dialog] does NOT work here — a microtask
    // checkpoint runs between listener callbacks, so Svelte has already
    // flushed the dialog out of the DOM by the time this runs.
    //
    // This also covers layers that deliberately ignore Escape: bits-ui
    // preventDefaults those too, and the composer shouldn't close under a
    // modal that is refusing to.
    if (e.defaultPrevented) return
    if (isDialogGuardActive()) return
    handleClose(s)
  }
}
