// Resolves the reader blocks' cid: pictures of a decrypted or verified
// message from its inline attachments (content ID → data URL). Nothing is
// requested: a picture whose content ID is unknown, whose data is not a
// raster data URL or that exceeds the budget is dropped.
import type { message } from '../../../../wailsjs/go/models'

// The same bounds as the backend applies to data: sources in stored mail.
export const READER_MAX_DATA_URI = 256 << 10
export const READER_MAX_DATA_TOTAL = 1 << 20

const RASTER = /^data:image\/(png|jpeg|gif|webp);base64,[A-Za-z0-9+/=]+$/i

export function resolveReaderCids(
  blocks: message.ReaderBlock[],
  inline: Record<string, string> | undefined,
): message.ReaderBlock[] {
  if (!blocks.some((b) => b.kind === 'image' && b.src?.startsWith('cid:'))) return blocks
  const byId = new Map<string, string>()
  for (const [id, url] of Object.entries(inline ?? {})) byId.set(id.replace(/^<|>$/g, '').toLowerCase(), url)
  let total = 0
  const out: message.ReaderBlock[] = []
  for (const block of blocks) {
    if (block.kind !== 'image' || !block.src?.startsWith('cid:')) {
      out.push(block)
      continue
    }
    const url = byId.get(block.src.slice(4).toLowerCase())
    if (!url || !RASTER.test(url) || url.length > READER_MAX_DATA_URI || total + url.length > READER_MAX_DATA_TOTAL) continue
    total += url.length
    out.push({ ...block, src: url } as message.ReaderBlock)
  }
  return out
}
