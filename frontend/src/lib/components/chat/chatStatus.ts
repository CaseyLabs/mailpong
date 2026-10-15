// The one status glyph a chat row shows, by priority: a reply on its way out,
// then snoozed, then unread. (A "needs reply" state would slot between sending
// and snoozed; it needs a latest-sender field the chat summary does not have.)

export type RowStatus = 'sending' | 'snoozed' | 'unread'

export function rowStatus(chat: { unreadCount: number; snoozedUntil: Date | null }, sending: boolean): RowStatus | null {
  if (sending) return 'sending'
  if (chat.snoozedUntil) return 'snoozed'
  return chat.unreadCount > 0 ? 'unread' : null
}
