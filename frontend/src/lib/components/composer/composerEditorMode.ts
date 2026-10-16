// Editor lifecycle and the rich/plain text mode switch.
import { stripParagraphStyles, htmlToPlainText, plainTextToHtml } from './composerUtils'
import { createComposerEditor } from './composerEditor'
import { getUserComposedText } from './composerContent'
import { scheduleDraftSave } from './composerDraft'
import { handleInlineImageFile, handleDroppedFile, handleDroppedFilePaths } from './composerAttachments'
import type { ComposerState } from './composerState.svelte'

// Load blocked remote images in the composer editor
export function loadComposerImages(s: ComposerState) {
  if (!s.editor) return
  const imgs = s.editor.view.dom.querySelectorAll('img[data-original-src]')
  imgs.forEach(img => {
    const originalSrc = img.getAttribute('data-original-src')
    if (originalSrc) {
      img.setAttribute('src', originalSrc)
      img.removeAttribute('data-original-src')
    }
  })
  s.composerImagesBlocked = false
}

// Insert image via file picker
export function insertImage(s: ComposerState) {
  // Create a hidden file input, append to DOM (required for WebKitGTK),
  // then click it to open the file picker
  const input = document.createElement('input')
  input.type = 'file'
  input.accept = 'image/*'
  input.style.display = 'none'
  document.body.appendChild(input)
  input.onchange = async (e) => {
    input.remove()
    const file = (e.target as HTMLInputElement).files?.[0]
    if (file) {
      await handleInlineImageFile(s, file)
    }
  }
  input.click()
}

// Create the TipTap editor on demand. No-op if it already exists or its
// element isn't mounted yet — it isn't while the composer is in plain text
// mode, since the editor <div> lives in the {:else} branch.
export function ensureEditor(s: ComposerState) {
  if (s.editor || !s.editorElement) return
  s.editor = createComposerEditor(s.editorElement, {
    onUpdate: () => scheduleDraftSave(s),
    onPasteImage: (file) => handleInlineImageFile(s, file),
    onDropImage: (file) => handleInlineImageFile(s, file),
    onDropFile: (file) => handleDroppedFile(s, file),
    onDropFilePaths: (paths) => handleDroppedFilePaths(s, paths),
    onShiftTab: () => document.getElementById('composer-subject')?.focus(),
  })
}

// Toggle between rich text and plain text mode. Both surfaces stay mounted
// (the template toggles visibility), so the editor is always live and
// setContent lands reliably — no async, no teardown, no race.
export function togglePlainTextMode(s: ComposerState) {
  // Rich -> plain
  if (!s.isPlainTextMode) {
    s.plainTextContent = htmlToPlainText(s.editor?.getHTML() || '')
    s.isPlainTextMode = true
    scheduleDraftSave(s)
    return
  }
  // Plain -> rich. Plain text can't carry images, and converting the degraded
  // plain text back to HTML would drop them permanently. So for a FRESH
  // reply/forward, REPROCESS the original: keep the user's typed intro but
  // restore the original rich quote/forward body (with images). Skipped for
  // drafts (whose saved body already includes the intro) and when the quote
  // boundary is gone, to avoid duplicating the body — those fall back to a
  // straight plain-text conversion.
  const FWD_SEPARATOR = '---------- Forwarded message ----------'
  const hasQuote = /^.*wrote:\s*$/m.test(s.plainTextContent) || s.plainTextContent.includes(FWD_SEPARATOR)
  let html: string
  if (!s.draftId && s.initialRichBody && hasQuote) {
    const intro = getUserComposedText(s).trim()
    html = (intro ? plainTextToHtml(intro) : '') + s.initialRichBody
  } else {
    html = plainTextToHtml(s.plainTextContent)
  }
  s.editor?.commands.setContent(stripParagraphStyles(html), { emitUpdate: false })
  s.isPlainTextMode = false
  s.editor?.commands.focus('start')
  scheduleDraftSave(s)
}
