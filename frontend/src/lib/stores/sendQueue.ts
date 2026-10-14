// Sends queued in the backend wait out an undo delay (app/send_queue.go);
// this tracks their outcomes from the send:finished event.
import { EventsOn } from '../../../wailsjs/runtime/runtime'
// @ts-ignore - wailsjs path
import { UndoSend } from '../../../wailsjs/go/app/App'

/** How long a queued send waits before it goes out (sendDelay in Go). */
export const UNDO_SEND_MS = 10_000

export interface SendResult {
  id: string
  outcome: string
  warnings: string
  error: string
  // Draft keeping a message that failed to send, if any.
  draftId: string
}

const waiters = new Map<string, (r: SendResult) => void>()
// Results that arrived before anyone asked (a send queued while quitting runs at once).
const early = new Map<string, SendResult>()

EventsOn('send:finished', (r: SendResult) => {
  const resolve = waiters.get(r.id)
  if (!resolve) {
    early.set(r.id, r)
    return
  }
  waiters.delete(r.id)
  resolve(r)
})

/** Resolves with a queued send's outcome; never resolves if it is undone. */
export function whenSent(id: string): Promise<SendResult> {
  const r = early.get(id)
  if (r) {
    early.delete(id)
    return Promise.resolve(r)
  }
  return new Promise((resolve) => waiters.set(id, resolve))
}

/** Cancels a queued send; false when it has already started. */
export async function undoSend(id: string): Promise<boolean> {
  const undone: boolean = await UndoSend(id)
  if (undone) waiters.delete(id)
  return undone
}
