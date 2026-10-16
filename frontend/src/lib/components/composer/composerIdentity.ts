// From identity handling: read receipt policy, signature, Reply-To and identity changes.
// @ts-ignore - Wails generated imports
import { account, smtp } from '../../../../wailsjs/go/models'
import { buildSignatureHtml, shouldAppendSignature, insertSignatureIntoContent, removeSignatureFromContent } from './composerSignature'
import { getDisplayMode } from './composerContent'
import { scheduleDraftSave } from './composerDraft'
import { updateSecurityForIdentity } from './composerSecurity'
import type { ComposerState } from './composerState.svelte'

// Apply read receipt policy from account settings
export function applyReadReceiptPolicy(s: ComposerState, policy: string) {
  switch (policy) {
    case 'always':
      s.requestReadReceipt = true
      s.showReadReceiptOption = false
      break
    case 'ask':
      s.requestReadReceipt = false
      s.showReadReceiptOption = true
      break
    default:
      s.requestReadReceipt = false
      s.showReadReceiptOption = false
  }
}

// Select identity based on the From address the backend determined for reply/forward
export function selectIdentityForReply(s: ComposerState): account.Identity | null {
  if (!s.initialMessage) return null

  // PrepareReply already determines the correct From based on the account
  // that owns the message. Match it to a local identity.
  const fromEmail = ((s.initialMessage.from as any)?.address || (s.initialMessage.from as any)?.email || '').toLowerCase()
  if (!fromEmail) return null

  return s.identities.find(identity =>
    identity.email.toLowerCase() === fromEmail
  ) || null
}

// Append signature for the current identity based on compose mode
export function appendSignatureForIdentity(s: ComposerState, identity: account.Identity) {
  if (!s.editor) return

  const mode = getDisplayMode(s)
  if (!shouldAppendSignature(identity, mode)) return

  const signatureHtml = buildSignatureHtml(identity)
  if (!signatureHtml) return

  const content = s.editor.getHTML()
  const newContent = insertSignatureIntoContent(
    content,
    signatureHtml,
    mode,
    identity.signaturePlacement || 'above'
  )

  s.editor.commands.setContent(newContent, { emitUpdate: false })
}

// The Reply-To an identity implies: its own, else its account's default
export async function defaultReplyToFor(s: ComposerState, identity: account.Identity | undefined): Promise<string> {
  if (identity?.replyTo) return identity.replyTo
  const accountDefault = await s.api.getDefaultAddress('replyto', identity?.accountId || s.activeAccountId)
  return accountDefault.split(/[,;]/).map(part => part.trim()).filter(Boolean)[0] || ''
}

// Swap the prefilled Reply-To for the new identity's; a user-edited value stays
export async function swapPrefilledReplyTo(s: ComposerState, identity: account.Identity) {
  const seq = ++s.replyToSwapSeq
  // Clearing a prefilled value is an edit too, so empty counts only when nothing was prefilled
  const untouched = () => s.replyToRecipients.length === 0
    ? s.prefilledReplyTo === ''
    : s.replyToRecipients.length === 1 && s.replyToRecipients[0].address === s.prefilledReplyTo
  if (!untouched()) return
  try {
    const next = await defaultReplyToFor(s, identity)
    // A later From change or a user edit while the lookup ran wins
    if (seq !== s.replyToSwapSeq || !untouched()) return
    s.replyToRecipients = next ? [new smtp.Address({ name: '', address: next })] : []
    s.prefilledReplyTo = next
    if (next) s.showReplyTo = true
  } catch (err) {
    console.error('Failed to load default Reply-To:', err)
  }
}

// Handle identity change from the From dropdown
export function handleIdentityChange(s: ComposerState, newIdentityId: string) {
  if (newIdentityId === s.selectedIdentityId) return

  const newIdentity = s.identities.find(i => i.id === newIdentityId)
  const oldAccountId = s.activeAccountId
  s.selectedIdentityId = newIdentityId

  if (newIdentity) swapPrefilledReplyTo(s, newIdentity)
  if (!s.editor || !newIdentity) return

  // If account changed, reload read receipt policy and migrate draft
  if (newIdentity.accountId !== oldAccountId) {
    s.api.getAccount(newIdentity.accountId).then(acc => {
      applyReadReceiptPolicy(s, acc.readReceiptRequestPolicy || 'never')
    }).catch(err => {
      console.error('Failed to load account settings:', err)
    })

    // Delete old draft (belongs to previous account) and clear ID
    // so the next save creates a fresh draft under the new account
    if (s.currentDraftId) {
      const oldDraftId = s.currentDraftId
      s.currentDraftId = null
      s.lastContent = ''
      s.api.deleteDraft(oldDraftId).catch(err => {
        console.error('Failed to delete old account draft:', err)
      })
    }
  }

  // Update security bars for the new identity (uses activeAccountId which is now updated)
  updateSecurityForIdentity(s, newIdentity.email)

  // Remove old signature and apply new one
  const content = removeSignatureFromContent(s.editor.getHTML())
  s.editor.commands.setContent(content, { emitUpdate: false })

  appendSignatureForIdentity(s, newIdentity)
  scheduleDraftSave(s)
}
