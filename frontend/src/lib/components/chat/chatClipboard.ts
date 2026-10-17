// Clipboard copy with the shared success and failure toasts, for the chat
// bubble menu and the participants menu.

import { get } from 'svelte/store'
import { _ } from '$lib/i18n'
import { toasts } from '$lib/stores/toast'

export async function copyText(text: string, label: string): Promise<void> {
  const t = get(_)
  try {
    await navigator.clipboard.writeText(text)
    toasts.success(t('viewer.copiedToClipboard', { values: { label } }))
  } catch {
    toasts.error(t('viewer.failedToCopy'))
  }
}
