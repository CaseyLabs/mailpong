// The docked composer modal of the main window: what it holds and the ways to
// open it (new message, draft, reply, mailto, undo-send). Detached composer
// windows are opened through OpenComposerWindow instead.

import { get } from 'svelte/store'
import { _ } from '$lib/i18n'
import { addToast } from './toast'
import { getComposerMode, getMailtoMode } from './settings.svelte'
import { conversationAccountId, mailSelection, resolveAccountId } from './mailSelection.svelte'
// @ts-ignore - wailsjs path
import { PrepareReply, GetDraft, OpenComposerWindow } from '../../../wailsjs/go/app/App.js'
// @ts-ignore - wailsjs path
import { smtp } from '../../../wailsjs/go/models'

export interface MailtoData {
  to?: string[]
  cc?: string[]
  bcc?: string[]
  subject?: string
  body?: string
}

export const composer = $state({
  show: false,
  accountId: null as string | null,
  initialMessage: null as smtp.ComposeMessage | null,
  draftId: null as string | null,
  imagesLoaded: false,
})

function open(accountId: string, message: smtp.ComposeMessage | null, draftId: string | null) {
  composer.accountId = accountId
  composer.initialMessage = message
  composer.draftId = draftId
  composer.show = true
}

function addresses(list: string[] | undefined) {
  return (list || []).map(addr => new smtp.Address({ name: '', address: addr }))
}

function blankMessage(fields: { to?: string[]; cc?: string[]; bcc?: string[]; subject?: string; body?: string }) {
  return new smtp.ComposeMessage({
    from: new smtp.Address({ name: '', address: '' }),
    to: addresses(fields.to),
    cc: addresses(fields.cc),
    bcc: addresses(fields.bcc),
    subject: fields.subject || '',
    text_body: fields.body || '',
    html_body: '',
    attachments: [],
    request_read_receipt: false,
  })
}

export function closeComposer() {
  composer.show = false
  composer.accountId = null
  composer.initialMessage = null
}

// New message from the selected account, or the first one.
export function openNewMessage() {
  const accountId = resolveAccountId(mailSelection.accountId)
  if (!accountId) return

  if (getComposerMode() === 'detached') {
    OpenComposerWindow(accountId, 'new', '', '', '')
    return
  }
  open(accountId, null, null)
}

// Open an existing draft in the composer.
export async function editDraft(draftId: string) {
  const accountId = conversationAccountId()
  if (!accountId) return

  try {
    const draftMessage = await GetDraft(draftId)
    open(accountId, draftMessage || null, draftId)
  } catch (err) {
    console.error('Failed to load draft:', err)
    addToast({ type: 'error', message: get(_)('composer.failedToLoadDraft') })
  }
}

// Undo on a queued send: reopen the message from its draft. A composer
// already open keeps its place, so the message opens in a window.
export function undoSend(accountId: string, message: smtp.ComposeMessage, draftId: string) {
  if (composer.show) {
    OpenComposerWindow(accountId, '', '', draftId, '')
    return
  }
  open(accountId, message, draftId)
}

// Open a chat draft handed over by the docked chat composer's Expand.
export function expandChatDraft(draftId: string) {
  const accountId = conversationAccountId()
  if (!accountId) return
  if (getComposerMode() === 'detached' || mailSelection.focusMode !== 'off') {
    OpenComposerWindow(accountId, '', '', draftId, '')
    return
  }
  void editDraft(draftId)
}

// Compose to a specific address (mailto: links in emails).
export function composeToAddress(toAddress: string) {
  const accountId = conversationAccountId()
  if (!accountId) return
  open(accountId, blankMessage({ to: [toAddress] }), null)
}

// Compose from mailto: data (command line, second instance).
export function openMailto(data: MailtoData, rawMailtoURL?: string) {
  const accountId = resolveAccountId(mailSelection.accountId)
  if (!accountId) {
    addToast({ type: 'error', message: get(_)('toast.noAccountConfigured') })
    return
  }

  if (getMailtoMode() === 'detached' && rawMailtoURL) {
    OpenComposerWindow(accountId, 'new', '', '', rawMailtoURL)
    return
  }
  open(accountId, blankMessage(data), null)
}

// Reply, reply all or forward, with the backend preparing the message.
export async function openReply(mode: 'reply' | 'reply-all' | 'forward', messageId: string, imagesLoaded?: boolean) {
  const accountId = conversationAccountId()
  if (!accountId) return

  // Focus mode keeps its view, so the composer opens detached.
  if (mailSelection.focusMode !== 'off') {
    OpenComposerWindow(accountId, mode, messageId, '', '')
    return
  }

  try {
    const composeMessage = await PrepareReply(messageId, mode)
    composer.imagesLoaded = imagesLoaded || false
    open(accountId, composeMessage, null)
  } catch (err) {
    console.error(`Failed to prepare ${mode}:`, err)
    addToast({ type: 'error', message: get(_)('toast.failedToPrepare', { values: { mode } }) })
    // Fallback: open blank composer
    open(accountId, null, null)
  }
}
