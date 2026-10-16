// Shared types and limits for the full composer (Composer.svelte and its modules).

// Attachment type from backend
export interface ComposerAttachment {
  filename: string
  contentType: string
  size: number
  data: string // base64 encoded
}

// Inline image type - for images pasted/dropped into the editor
export interface InlineImage {
  cid: string  // Content-ID (e.g., "image1@mailven")
  dataUrl: string  // Full data URL for display in editor
  contentType: string
  data: string  // Base64 data only (without data URL prefix)
  filename: string
}

// 10-second debounce like Geary
export const DRAFT_SAVE_DELAY = 10000

// Max inline image size (10 MB) — larger files should be added as regular attachments
export const MAX_INLINE_IMAGE_SIZE = 10 * 1024 * 1024

// Local per-file attachment cap (100 MB); actual message limits vary by account/server.
export const MAX_ATTACHMENT_SIZE = 100 * 1024 * 1024
