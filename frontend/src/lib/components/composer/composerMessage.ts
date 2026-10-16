// Builds the outgoing message from composer state, including inline image handling.
// @ts-ignore - Wails generated imports
import { smtp } from '../../../../wailsjs/go/models'
import { addParagraphStyles } from './composerUtils'
import type { ComposerState } from './composerState.svelte'

// Collect any images in the editor DOM that aren't tracked in inlineImages.
// WebKitGTK doesn't expose pasted screenshots via clipboardData, so TipTap's
// default handler inserts them with a webkit-fake-url:// src. This function
// extracts the pixel data via canvas and registers them for CID conversion.
export function collectUnregisteredInlineImages(s: ComposerState, html: string): string {
  const editorEl = s.editor?.view?.dom
  if (!editorEl) return html

  const imgs = editorEl.querySelectorAll('img')
  let result = html

  for (const img of imgs) {
    const src = img.getAttribute('src') || ''

    // Skip tracked, cid:, http(s):, and blocked remote images
    if (src.startsWith('cid:') || src.startsWith('http://') || src.startsWith('https://')) continue
    if (img.hasAttribute('data-original-src')) continue
    if (s.inlineImages.some(i => i.dataUrl === src)) continue

    // data: URLs — parse and register directly
    if (src.startsWith('data:')) {
      const match = src.match(/^data:([^;]+);base64,(.+)$/)
      if (!match) continue
      const cid = generateCID(s)
      s.inlineImages = [...s.inlineImages, {
        cid,
        dataUrl: src,
        contentType: match[1],
        data: match[2],
        filename: `pasted-image${s.inlineImageCounter}.${match[1].split('/')[1] || 'png'}`,
      }]
      continue
    }

    // webkit-fake-url://, blob:, etc. — extract via canvas
    if (!img.complete || img.naturalWidth === 0) continue
    try {
      const canvas = document.createElement('canvas')
      canvas.width = img.naturalWidth
      canvas.height = img.naturalHeight
      const ctx = canvas.getContext('2d')
      if (!ctx) continue
      ctx.drawImage(img, 0, 0)
      const dataUrl = canvas.toDataURL('image/png')
      const base64Data = dataUrl.split(',')[1]

      // If the same canvas-extracted content was already registered
      // (e.g. user pasted the same screenshot twice), reuse that cid —
      // the viewer is what handles same-cid resolution (see
      // EmailBody.svelte querySelectorAll fix).
      const dup = s.inlineImages.find(i => i.dataUrl === dataUrl)
      if (dup) {
        result = result.replaceAll(src, dup.dataUrl)
        continue
      }

      const cid = generateCID(s)
      s.inlineImages = [...s.inlineImages, {
        cid,
        dataUrl,
        contentType: 'image/png',
        data: base64Data,
        filename: `pasted-image${s.inlineImageCounter}.png`,
      }]
      result = result.replaceAll(src, dataUrl)
    } catch {
      continue
    }
  }

  return result
}

// Convert HTML with data URLs to use CID references for inline images
export function convertDataUrlsToCid(s: ComposerState, html: string): string {
  let result = html

  // For each inline image, replace its data URL with cid: reference
  for (const img of s.inlineImages) {
    result = result.replaceAll(img.dataUrl, `cid:${img.cid}`)
  }

  return result
}

// Build message object from current composer state
export function buildMessage(s: ComposerState): smtp.ComposeMessage {
  const selectedIdentity = s.identities.find(i => i.id === s.selectedIdentityId)

  // Handle plain text vs rich text mode
  let htmlContent: string
  let textContent: string

  if (s.isPlainTextMode) {
    // In plain text mode, we only have plain text
    textContent = s.plainTextContent
    htmlContent = ''  // No HTML version when composing in plain text
  } else {
    // In rich text mode, we have both
    // Collect any untracked pasted images (WebKitGTK webkit-fake-url, etc.),
    // add paragraph styles for email clients, then convert data URLs to CID references
    const rawHtml = collectUnregisteredInlineImages(s, s.editor?.getHTML() || '')
    htmlContent = convertDataUrlsToCid(s, addParagraphStyles(rawHtml))
    textContent = s.editor?.getText() || ''
  }

  // Restore blocked remote images for sending — replace placeholder with original URL
  htmlContent = htmlContent.replace(
    /<img([^>]*)\sdata-original-src="([^"]+)"([^>]*)>/gi,
    (match, _before, originalSrc, _after) => {
      return match
        .replace(/src="[^"]*"/, `src="${originalSrc}"`)
        .replace(/\s*data-original-src="[^"]*"/, '')
    }
  )

  // Convert ComposerAttachment to smtp.Attachment format (regular attachments)
  // Use content_base64 (string) instead of content (number[]) to avoid
  // pathologically slow JSON serialization of large byte arrays through Wails RPC.
  const smtpAttachments: smtp.Attachment[] = s.attachments.map(att => new smtp.Attachment({
    filename: att.filename,
    content_type: att.contentType,
    content_base64: att.data,
    content_id: '',
    inline: false,
  }))

  // Add inline images as inline attachments with Content-ID
  for (const img of s.inlineImages) {
    smtpAttachments.push(new smtp.Attachment({
      filename: img.filename,
      content_type: img.contentType,
      content_base64: img.data,
      content_id: img.cid,
      inline: true,
    }))
  }

  return new smtp.ComposeMessage({
    from: new smtp.Address({
      name: selectedIdentity?.name || '',
      address: selectedIdentity?.email || '',
    }),
    to: s.toRecipients,
    cc: s.ccRecipients,
    bcc: s.bccRecipients,
    reply_to: s.replyToRecipients[0],
    subject: s.subject,
    html_body: htmlContent,
    text_body: textContent,
    attachments: smtpAttachments,
    in_reply_to: s.inReplyTo,
    references: s.references,
    request_read_receipt: s.requestReadReceipt,
    sign_message: s.signMessage,
    encrypt_message: s.encryptMessage,
    pgp_sign_message: s.pgpSignMessage,
    pgp_encrypt_message: s.pgpEncryptMessage,
  })
}

// Generate a unique Content-ID for inline images
export function generateCID(s: ComposerState): string {
  s.inlineImageCounter++
  return `image${s.inlineImageCounter}-${Date.now()}@mailven`
}
