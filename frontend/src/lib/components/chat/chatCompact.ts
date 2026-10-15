// chatCompact — conversation compaction: runs of older, already-read messages
// fold into one expandable row. The latest KEEP_RECENT messages, unread
// messages, drafts and subject-heading messages never fold.

import type { ThreadItem } from './chatFormat'

export const KEEP_RECENT = 5
export const MIN_RUN = 3

export type StreamEntry =
  | { kind: 'message'; item: ThreadItem }
  // id is the first message of the run; open runs are followed by their messages.
  | { kind: 'run'; id: string; count: number; open: boolean }

// compactItems folds runs of at least MIN_RUN collapsible messages. `keep`
// holds IDs that stay visible (messages that were unread when opened and have
// since been read); `expanded` holds the IDs of runs the user opened.
export function compactItems(items: ThreadItem[], expanded: ReadonlySet<string>, keep: ReadonlySet<string> = new Set()): StreamEntry[] {
  const limit = items.length - KEEP_RECENT
  const collapsible = (item: ThreadItem, i: number) =>
    i < limit && item.msg.isRead && !item.msg.isDraft && !item.subject && !keep.has(item.msg.id)
  const out: StreamEntry[] = []
  let i = 0
  while (i < items.length) {
    let end = i
    while (end < items.length && collapsible(items[end], end)) end++
    if (end - i < MIN_RUN) {
      out.push({ kind: 'message', item: items[i] })
      i++
      continue
    }
    const id = items[i].msg.id
    const open = expanded.has(id)
    out.push({ kind: 'run', id, count: end - i, open })
    if (open) {
      for (let j = i; j < end; j++) out.push({ kind: 'message', item: items[j] })
      i = end
      continue
    }
    // The message after a folded run starts a new run: show its sender.
    if (end < items.length) out.push({ kind: 'message', item: { ...items[end], groupStart: true } })
    i = end + 1
  }
  return out
}
