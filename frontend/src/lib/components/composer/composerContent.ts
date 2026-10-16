// Read-only views of the composer content: user text, display mode, change detection.
import { textMentionsAttachment } from './composerUtils'
import type { ComposerState } from './composerState.svelte'

// Get only the user-composed text, excluding quoted/forwarded content
export function getUserComposedText(s: ComposerState): string {
  const FWD_SEPARATOR = '---------- Forwarded message ----------'
  if (s.isPlainTextMode) {
    // Reply: citation line ending in "wrote:"
    const wroteIndex = s.plainTextContent.search(/^.*wrote:\s*$/m)
    // Forward: separator line
    const fwdIndex = s.plainTextContent.indexOf(FWD_SEPARATOR)
    // Take the earliest match
    const cutoff = [wroteIndex, fwdIndex].filter(i => i > -1)
    if (cutoff.length > 0) return s.plainTextContent.substring(0, Math.min(...cutoff))
    return s.plainTextContent
  }
  // In rich text, find the earliest of <blockquote (reply) or the forwarded message separator
  const html = s.editor?.getHTML() || ''
  const blockquoteIndex = html.indexOf('<blockquote')
  const fwdIndex = html.indexOf(FWD_SEPARATOR)
  const cutoffs = [blockquoteIndex, fwdIndex].filter(i => i > -1)
  const userHtml = cutoffs.length > 0 ? html.substring(0, Math.min(...cutoffs)) : html
  const tmp = document.createElement('div')
  tmp.innerHTML = userHtml
  return tmp.textContent || ''
}

// Check if the email body contains keywords that suggest an attachment should be present
export function bodyMentionsAttachment(s: ComposerState): boolean {
  const combinedText = getUserComposedText(s) + ' ' + s.subject
  return textMentionsAttachment(combinedText)
}

// Determine display mode from initialMessage
export function getDisplayMode(s: ComposerState): 'new' | 'reply' | 'reply-all' | 'forward' {
  if (!s.initialMessage) return 'new'
  if (s.initialMessage.subject?.startsWith('Fwd:')) return 'forward'
  if (s.initialMessage.in_reply_to) {
    // reply-all if there are multiple To recipients or any Cc
    if ((s.initialMessage.to?.length || 0) > 1 || (s.initialMessage.cc?.length || 0) > 0) {
      return 'reply-all'
    }
    return 'reply'
  }
  return 'new'
}

// Check if the composer has any meaningful content worth saving
export function hasContent(s: ComposerState): boolean {
  const bodyText = s.isPlainTextMode ? s.plainTextContent.trim() : (s.editor?.getText()?.trim() || '')
  return s.toRecipients.length > 0 || s.ccRecipients.length > 0 || s.bccRecipients.length > 0 ||
         s.subject.trim() !== '' || bodyText !== '' || s.attachments.length > 0
}

// Get a content hash to detect meaningful changes
export function getContentHash(s: ComposerState): string {
  const bodyContent = s.isPlainTextMode ? s.plainTextContent : (s.editor?.getHTML() || '')
  const attachmentNames = s.attachments.map(a => a.filename).join(',')
  return `${s.toRecipients.length}|${s.ccRecipients.length}|${s.bccRecipients.length}|${s.replyToRecipients[0]?.address || ''}|${s.subject}|${bodyContent}|${attachmentNames}|${s.isPlainTextMode}`
}
