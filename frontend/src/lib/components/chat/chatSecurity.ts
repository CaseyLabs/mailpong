// S/MIME and PGP status lines for one chat message, shared by the banner list
// and the bubble's meta-line badges.
import type { message as messageModels } from '../../../../wailsjs/go/models'
import type { ChatThread } from './chatThread.svelte'

export type Tone = 'info' | 'ok' | 'warn' | 'bad' | 'muted'
export interface SecurityLine {
  tone: Tone
  icon: string
  text: string
  spin?: boolean
  /** Positive state (signed, encrypted): shown as a compact badge, not a banner. */
  badge?: boolean
}
/** Background, border and text classes for each tone. */
export const TONES: Record<Tone, string> = {
  info: 'bg-info/12 border-info/35 text-info-foreground',
  ok: 'bg-success/12 border-success/35 text-success-foreground',
  warn: 'bg-warning/12 border-warning/35 text-warning-foreground',
  bad: 'bg-destructive/12 border-destructive/35 text-foreground [&_svg]:text-destructive',
  muted: 'bg-muted/50 border-border text-muted-foreground',
}

type Translate = (key: string, options?: { values?: Record<string, string> }) => string

// status → [tone, icon, i18n key]; keys taking {email} or {keyId} get them filled in.
const SMIME: Record<string, [Tone, string, string]> = {
  signed: ['ok', 'mdi:shield-check', 'viewer.smimeSignedBy'],
  unknown_signer: ['warn', 'mdi:shield-alert', 'viewer.smimeUnknownSigner'],
  self_signed: ['warn', 'mdi:shield-alert', 'viewer.smimeSelfSigned'],
  expired_cert: ['bad', 'mdi:shield-off', 'viewer.smimeExpiredCert'],
  signer_mismatch: ['warn', 'mdi:shield-alert', 'viewer.smimeSignerMismatch'],
  invalid: ['bad', 'mdi:shield-off', 'viewer.smimeInvalid'],
  decrypt_failed: ['bad', 'mdi:lock-off', 'viewer.smimeDecryptFailed'],
}
const PGP: Record<string, [Tone, string, string]> = {
  signed: ['ok', 'mdi:key-check', 'viewer.pgpSignedBy'],
  unknown_key: ['warn', 'mdi:key-alert', 'viewer.pgpUnknownKey'],
  expired_key: ['warn', 'mdi:key-alert', 'viewer.pgpExpiredKey'],
  revoked_key: ['bad', 'mdi:key-remove', 'viewer.pgpRevokedKey'],
  signer_mismatch: ['warn', 'mdi:key-alert', 'viewer.pgpSignerMismatch'],
  invalid: ['bad', 'mdi:key-remove', 'viewer.pgpInvalid'],
  decrypt_failed: ['bad', 'mdi:lock-off', 'viewer.pgpDecryptFailed'],
}

export function securityStatus(msg: messageModels.Message, thread: ChatThread, t: Translate): SecurityLine[] {
  const out: SecurityLine[] = []
  const unknown = t('viewer.unknown').toLowerCase()

  // S/MIME: on-view result for S/MIME messages, cached status otherwise.
  const sm = thread.smimeResults[msg.id]
  if (thread.smimeLoading.has(msg.id)) out.push({ tone: 'muted', icon: 'mdi:loading', text: t('viewer.processingSMIME'), spin: true })
  if (sm?.smimeEncrypted) out.push({ tone: 'info', icon: 'mdi:lock-check', text: t('viewer.smimeEncryptedWith'), badge: true })
  const smStatus = msg.hasSMIME ? sm?.smimeStatus : msg.smimeStatus
  const smEntry = smStatus ? SMIME[smStatus] : undefined
  if (smEntry) {
    const email = (msg.hasSMIME ? sm?.smimeSignerEmail || sm?.smimeSignerSubject : msg.smimeSignerEmail || msg.smimeSignerSubject) || unknown
    out.push({ tone: smEntry[0], icon: smEntry[1], text: t(smEntry[2], { values: { email } }), badge: smEntry[0] === 'ok' })
  }

  const pg = thread.pgpResults[msg.id]
  if (thread.pgpLoading.has(msg.id)) out.push({ tone: 'muted', icon: 'mdi:loading', text: t('viewer.processingPGP'), spin: true })
  if (pg?.pgpEncrypted) out.push({ tone: 'info', icon: 'mdi:lock-check', text: t('viewer.pgpEncryptedWith'), badge: true })
  const pgEntry = pg?.pgpStatus ? PGP[pg.pgpStatus] : undefined
  if (pgEntry) {
    const values = { email: pg!.pgpSignerEmail || unknown, keyId: pg!.pgpSignerKeyId || '' }
    out.push({ tone: pgEntry[0], icon: pgEntry[1], text: t(pgEntry[2], { values }), badge: pgEntry[0] === 'ok' })
  }
  return out
}
