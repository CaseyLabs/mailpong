// Attachments and inline images: file pickers, paste/drop handling and drag-over state.
import { get } from 'svelte/store'
import { _ } from '$lib/i18n'
import { addToast } from '$lib/stores/toast'
import { MAX_INLINE_IMAGE_SIZE, MAX_ATTACHMENT_SIZE } from './composerTypes'
import type { ComposerAttachment, InlineImage } from './composerTypes'
import { parseFileUris, readFileAsBase64, readFileAsDataUrl } from './composerUtils'
import { generateCID } from './composerMessage'
import { scheduleDraftSave } from './composerDraft'
import type { ComposerState } from './composerState.svelte'

// Handle an inline image file (from paste or drop)
export async function handleInlineImageFile(s: ComposerState, file: File) {
  if (file.size > MAX_INLINE_IMAGE_SIZE) {
    addToast({
      type: 'error',
      message: get(_)('composer.imageTooLarge'),
    })
    return
  }

  try {
    const dataUrl = await readFileAsDataUrl(file)

    // Dedup by content (dataUrl): same image pasted twice produces a
    // single inlineImage entry, so the sent MIME has one inline
    // attachment instead of leaving the second cid orphaned. The editor
    // still gets a second <img> with the same dataUrl src — the viewer
    // side is what handles same-cid resolution (see EmailBody.svelte).
    const existing = s.inlineImages.find(i => i.dataUrl === dataUrl)
    if (existing) {
      s.editor?.chain().focus().setImage({ src: existing.dataUrl, alt: existing.filename }).run()
      scheduleDraftSave(s)
      return
    }

    const cid = generateCID(s)

    // Extract base64 data and content type from data URL
    const matches = dataUrl.match(/^data:([^;]+);base64,(.+)$/)
    if (!matches) {
      console.error('Invalid data URL format')
      return
    }

    const contentType = matches[1]
    const base64Data = matches[2]

    // Store the inline image
    const inlineImage: InlineImage = {
      cid,
      dataUrl,
      contentType,
      data: base64Data,
      filename: file.name || `image${s.inlineImageCounter}.${contentType.split('/')[1] || 'png'}`,
    }
    s.inlineImages = [...s.inlineImages, inlineImage]

    // Insert the image into the editor with the data URL (for display)
    // When sending, we'll convert data URLs to cid: references
    s.editor?.chain().focus().setImage({ src: dataUrl, alt: inlineImage.filename }).run()

    scheduleDraftSave(s)
  } catch (err) {
    console.error('Failed to process inline image:', err)
    addToast({
      type: 'error',
      message: get(_)('composer.failedToInsertImage'),
    })
  }
}

// Handle a non-image File dropped on the editor (add as attachment)
export async function handleDroppedFile(s: ComposerState, file: File) {
  if (file.size > MAX_ATTACHMENT_SIZE) {
    addToast({ type: 'error', message: get(_)('composer.attachmentTooLarge') })
    return
  }

  try {
    const data = await readFileAsBase64(file)
    s.attachments = [...s.attachments, {
      filename: file.name,
      contentType: file.type || 'application/octet-stream',
      size: file.size,
      data,
    }]
    scheduleDraftSave(s)
  } catch (err) {
    console.error('Failed to read dropped file:', err)
  }
}

// Handle file paths dropped on the editor (from text/uri-list parsing)
// Images are inserted inline, other files are added as attachments
export async function handleDroppedFilePaths(s: ComposerState, paths: string[]) {
  for (const filePath of paths) {
    try {
      const att = await s.api.readFileAsAttachment(filePath)
      if (!att) continue

      if (att.contentType.startsWith('image/')) {
        // Check size before inserting inline
        const imageBytes = Math.ceil((att.data.length * 3) / 4) // Estimate decoded size from base64
        if (imageBytes > MAX_INLINE_IMAGE_SIZE) {
          addToast({
            type: 'error',
            message: get(_)('composer.imageTooLarge'),
          })
          continue
        }
        // Insert as inline image
        const dataUrl = `data:${att.contentType};base64,${att.data}`

        // Dedup by content; mirrors handleInlineImageFile. Same image
        // dropped twice ⇒ one inline attachment (no orphan cid in MIME).
        const existing = s.inlineImages.find(i => i.dataUrl === dataUrl)
        if (existing) {
          s.editor?.chain().focus().setImage({ src: existing.dataUrl, alt: existing.filename }).run()
          continue
        }

        const cid = generateCID(s)
        s.inlineImages = [...s.inlineImages, {
          cid,
          dataUrl,
          contentType: att.contentType,
          data: att.data,
          filename: att.filename,
        }]
        s.editor?.chain().focus().setImage({ src: dataUrl, alt: att.filename }).run()
        continue
      }
      // Add as regular attachment
      if (att.size > MAX_ATTACHMENT_SIZE) {
        addToast({ type: 'error', message: get(_)('composer.attachmentTooLarge') })
        continue
      }
      s.attachments = [...s.attachments, {
        filename: att.filename,
        contentType: att.contentType,
        size: att.size,
        data: att.data,
      }]
    } catch {
      // Direct read failed — if Flatpak, show permission info dialog
      if (await s.api.isFlatpak()) {
        s.showFlatpakDndDialog = true
      }
      return
    }
  }
  scheduleDraftSave(s)
}

// Attachment handling — uses HTML file input so WebKitGTK routes through
// the FileChooser portal (required for Flatpak sandbox file access)
export function handleAttachFiles(s: ComposerState) {
  // Append to DOM before clicking (required for WebKitGTK to reliably
  // open the file chooser dialog on the first click)
  const input = document.createElement('input')
  input.type = 'file'
  input.multiple = true
  input.style.display = 'none'
  document.body.appendChild(input)
  input.onchange = async (e) => {
    input.remove()
    const fileList = (e.target as HTMLInputElement).files
    if (!fileList || fileList.length === 0) return

    try {
      const newAttachments: typeof s.attachments = []
      for (const file of Array.from(fileList)) {
        if (file.size > MAX_ATTACHMENT_SIZE) {
          addToast({ type: 'error', message: get(_)('composer.attachmentTooLarge') })
          continue
        }
        const dataUrl = await readFileAsDataUrl(file)
        const matches = dataUrl.match(/^data:([^;]+);base64,(.+)$/)
        if (!matches) continue

        newAttachments.push({
          filename: file.name,
          contentType: matches[1],
          size: file.size,
          data: matches[2],
        })
      }
      if (newAttachments.length > 0) {
        s.attachments = [...s.attachments, ...newAttachments]
        scheduleDraftSave(s)
      }
    } catch (err) {
      console.error('Failed to attach files:', err)
      addToast({
        type: 'error',
        message: get(_)('composer.failedToAttachFiles'),
      })
    }
  }
  input.click()
}

export function removeAttachment(s: ComposerState, index: number) {
  s.attachments = s.attachments.filter((_, i) => i !== index)
  scheduleDraftSave(s)
}

// Drag and drop handlers. Ignore recipient-chip drags so the composer doesn't
// claim them as file drops (which would make the chip's dragend think a
// successful move happened and remove it from the source field).
export function isRecipientChipDrag(e: DragEvent): boolean {
  return !!e.dataTransfer?.types.includes('application/x-mailven-recipient')
}

export function handleDragOver(s: ComposerState, e: DragEvent) {
  if (isRecipientChipDrag(e)) return
  e.preventDefault()
  e.stopPropagation()
  s.isDraggingOver = true
}

export function handleDragLeave(s: ComposerState, e: DragEvent) {
  if (isRecipientChipDrag(e)) return
  e.preventDefault()
  e.stopPropagation()
  s.isDraggingOver = false
}

export async function handleDrop(s: ComposerState, e: DragEvent) {
  if (isRecipientChipDrag(e)) return
  e.stopPropagation()
  s.isDraggingOver = false

  // Already handled by TipTap editor's handleDrop
  if (e.defaultPrevented) return
  e.preventDefault()

  // Case 1: File objects from browser-internal drag operations
  const files = e.dataTransfer?.files
  if (files && files.length > 0) {
    const newAttachments: ComposerAttachment[] = []
    for (const file of Array.from(files)) {
      if (file.size > MAX_ATTACHMENT_SIZE) {
        addToast({ type: 'error', message: get(_)('composer.attachmentTooLarge') })
        continue
      }
      try {
        const data = await readFileAsBase64(file)
        newAttachments.push({
          filename: file.name,
          contentType: file.type || 'application/octet-stream',
          size: file.size,
          data,
        })
      } catch (err) {
        console.error('Failed to read dropped file:', err)
      }
    }
    if (newAttachments.length > 0) {
      s.attachments = [...s.attachments, ...newAttachments]
      scheduleDraftSave(s)
    }
    return
  }

  // Case 2: File URIs (drops outside editor — all as attachments)
  const uriList = e.dataTransfer?.getData('text/uri-list')
  const textData = e.dataTransfer?.getData('text/plain')
  const pathData = uriList || textData
  if (pathData) {
    const paths = parseFileUris(pathData)
    if (paths.length > 0) {
      let directReadFailed = false
      for (const filePath of paths) {
        try {
          const att = await s.api.readFileAsAttachment(filePath)
          if (!att) continue
          if (att.size > MAX_ATTACHMENT_SIZE) {
            addToast({ type: 'error', message: get(_)('composer.attachmentTooLarge') })
            continue
          }
          s.attachments = [...s.attachments, {
            filename: att.filename,
            contentType: att.contentType,
            size: att.size,
            data: att.data,
          }]
        } catch {
          directReadFailed = true
          break
        }
      }
      if (directReadFailed) {
        // If Flatpak, show permission info dialog
        if (await s.api.isFlatpak()) {
          s.showFlatpakDndDialog = true
        }
        return
      }
      scheduleDraftSave(s)
    }
  }
}
