// Undo send for the full composer. In the main window the message is queued
// in the backend and the composer closes, leaving a toast with Undo; a
// detached composer window counts down in place before it sends.
import { get } from 'svelte/store'
import { _ } from '$lib/i18n'
import { addToast, toasts } from '$lib/stores/toast'
import { UNDO_SEND_MS, undoSend, whenSent } from '$lib/stores/sendQueue'
import type { ComposerApi } from '$lib/composerApi'
import type { smtp } from '../../../../wailsjs/go/models'

/**
 * Queues a saved message and shows the Undo toast. Undo hands the message
 * back through onUndo, with the draft that still holds it.
 */
export async function queueWithUndo(
  queue: NonNullable<ComposerApi['queueMessage']>,
  accountId: string,
  message: smtp.ComposeMessage,
  draftId: string,
  onUndo: (accountId: string, message: smtp.ComposeMessage, draftId: string) => void,
): Promise<void> {
  const t = get(_)
  const sendId = await queue(accountId, message, draftId)
  let settled = false
  const toastId = addToast({
    type: 'info',
    message: t('composer.messageQueued'),
    duration: UNDO_SEND_MS,
    actions: [
      {
        label: t('composer.undo'),
        onClick: async () => {
          toasts.remove(toastId)
          if (settled) return
          if (await undoSend(sendId)) {
            settled = true
            onUndo(accountId, message, draftId)
            return
          }
          addToast({ type: 'info', message: t('composer.alreadySent') })
        },
      },
    ],
  })
  void whenSent(sendId).then((r) => {
    settled = true
    toasts.remove(toastId)
    if (r.error) {
      console.error('Failed to send message:', r.error)
      addToast({ type: 'error', message: t('composer.sendFailedDraftKept') })
      return
    }
    addToast({ type: 'success', message: t('composer.messageSent') })
  })
}

/** The detached composer's countdown before a send goes out. */
export class SendCountdown {
  // Seconds left; 0 when not counting down.
  seconds = $state(0)
  #timer: ReturnType<typeof setInterval> | null = null
  #resolve: ((send: boolean) => void) | null = null

  /** Counts down; resolves true to send, false when undone. */
  run(): Promise<boolean> {
    this.#end(false)
    const deadline = Date.now() + UNDO_SEND_MS
    this.seconds = Math.ceil(UNDO_SEND_MS / 1000)
    this.#timer = setInterval(() => {
      this.seconds = Math.max(0, Math.ceil((deadline - Date.now()) / 1000))
      if (this.seconds === 0) this.#end(true)
    }, 250)
    return new Promise((resolve) => (this.#resolve = resolve))
  }

  /** Sends now. */
  sendNow() {
    this.#end(true)
  }

  undo() {
    this.#end(false)
  }

  #end(send: boolean) {
    if (this.#timer) clearInterval(this.#timer)
    this.#timer = null
    this.seconds = 0
    const resolve = this.#resolve
    this.#resolve = null
    resolve?.(send)
  }
}
