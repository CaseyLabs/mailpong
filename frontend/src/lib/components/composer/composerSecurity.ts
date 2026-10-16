// S/MIME and PGP: recipient certificate and key checks, and per-identity availability.
import { get } from 'svelte/store'
import { _ } from '$lib/i18n'
import { addToast } from '$lib/stores/toast'
import type { ComposerState } from './composerState.svelte'

export function checkRecipientCertsDebounced(s: ComposerState, emails: string[]) {
  if (s.certCheckTimeout) clearTimeout(s.certCheckTimeout)
  s.certCheckTimeout = setTimeout(async () => {
    try {
      s.recipientCertStatus = await s.api.checkRecipientCerts(emails)
    } catch (err) {
      console.error('Failed to check recipient certs:', err)
    }
  }, 300)
}

export function checkRecipientPGPKeysDebounced(s: ComposerState, emails: string[]) {
  if (s.pgpKeyCheckTimeout) clearTimeout(s.pgpKeyCheckTimeout)
  s.pgpKeyCheckTimeout = setTimeout(async () => {
    try {
      s.recipientPGPKeyStatus = await s.api.checkRecipientPGPKeys(emails)

      // Auto-discover missing keys via unified WKD+HKP lookup
      const missingEmails = emails.filter(e => !s.recipientPGPKeyStatus[e])
      for (const email of missingEmails) {
        try {
          const armored = await s.api.lookupPGPKey(email)
          if (armored) {
            s.recipientPGPKeyStatus = { ...s.recipientPGPKeyStatus, [email]: true }
          }
        } catch { /* silent — lookup failure is not an error for the user */ }
      }
    } catch (err) {
      console.error('Failed to check recipient PGP keys:', err)
    }
  }, 300)
}

export async function handleImportRecipientCert(s: ComposerState) {
  try {
    const filePath = await s.api.pickRecipientCertFile()
    if (!filePath) return
    // Import for the first missing recipient
    if (s.missingCertRecipients.length > 0) {
      await s.api.importRecipientCert(s.missingCertRecipients[0], filePath)
      addToast({ type: 'success', message: get(_)('composer.certImported', { values: { email: s.missingCertRecipients[0] } }) })
      // Re-check certs
      const allEmails = [...s.toRecipients, ...s.ccRecipients, ...s.bccRecipients]
        .map(r => r.address).filter(Boolean)
      s.recipientCertStatus = await s.api.checkRecipientCerts(allEmails)
    }
  } catch (err) {
    console.error('Failed to import recipient cert:', err)
    addToast({ type: 'error', message: get(_)('composer.failedToImportCert') })
  }
}

export async function handleImportRecipientPGPKey(s: ComposerState) {
  try {
    const filePath = await s.api.pickRecipientPGPKeyFile()
    if (!filePath) return
    if (s.missingPGPKeyRecipients.length > 0) {
      await s.api.importRecipientPGPKey(s.missingPGPKeyRecipients[0], filePath)
      addToast({ type: 'success', message: get(_)('composer.pgpKeyImported', { values: { email: s.missingPGPKeyRecipients[0] } }) })
      const allEmails = [...s.toRecipients, ...s.ccRecipients, ...s.bccRecipients]
        .map(r => r.address).filter(Boolean)
      s.recipientPGPKeyStatus = await s.api.checkRecipientPGPKeys(allEmails)
    }
  } catch (err) {
    console.error('Failed to import recipient PGP key:', err)
    addToast({ type: 'error', message: get(_)('composer.failedToImportPGPKey') })
  }
}

// Update security bar visibility based on the selected identity's email
export async function loadSMIMEForEmail(s: ComposerState, email: string) {
  const acctId = s.activeAccountId
  const cert = await s.api.getSMIMECertificateForEmail(acctId, email)
  if (!cert || cert.isExpired) {
    s.signMessage = false
    s.encryptMessage = false
    return
  }

  s.showSignOption = true
  s.showEncryptOption = true
  s.smimeCertFingerprint = cert.fingerprint ? cert.fingerprint.substring(0, 8).toUpperCase() : ''

  const [signPolicy, encryptPolicy] = await Promise.all([
    s.api.getSMIMESignPolicy(acctId),
    s.api.getSMIMEEncryptPolicy(acctId),
  ])
  s.signMessage = signPolicy === 'always'
  s.encryptMessage = encryptPolicy === 'always'
}

export async function loadPGPForEmail(s: ComposerState, email: string) {
  const acctId = s.activeAccountId
  const key = await s.api.getPGPKeyForEmail(acctId, email)
  if (!key || key.isExpired) {
    s.pgpSignMessage = false
    s.pgpEncryptMessage = false
    return
  }

  s.showPGPSignOption = true
  s.showPGPEncryptOption = true
  s.pgpKeyId = key.fingerprint ? key.fingerprint.slice(-8).toUpperCase() : ''

  const [pgpSignPolicy, pgpEncryptPolicy] = await Promise.all([
    s.api.getPGPSignPolicy(acctId),
    s.api.getPGPEncryptPolicy(acctId),
  ])
  // Only enable PGP defaults if S/MIME is not already active (mutual exclusivity)
  s.pgpSignMessage = !s.signMessage && pgpSignPolicy === 'always'
  s.pgpEncryptMessage = !s.encryptMessage && pgpEncryptPolicy === 'always'
}

export async function updateSecurityForIdentity(s: ComposerState, email: string) {
  // Reset all security state
  s.showSignOption = false
  s.showEncryptOption = false
  s.showPGPSignOption = false
  s.showPGPEncryptOption = false
  s.signMessage = false
  s.encryptMessage = false
  s.pgpSignMessage = false
  s.pgpEncryptMessage = false
  s.smimeCertFingerprint = ''
  s.pgpKeyId = ''

  if (!email) return

  try { await loadSMIMEForEmail(s, email) } catch (err) {
    console.error('Failed to load S/MIME settings:', err)
  }

  try { await loadPGPForEmail(s, email) } catch (err) {
    console.error('Failed to load PGP settings:', err)
  }
}
