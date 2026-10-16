// Behavior of the account Security tab (moved from AccountSecurityTab.svelte).
import { addToast } from '$lib/stores/toast'
import { get } from 'svelte/store'
import { _ } from '$lib/i18n'
// @ts-ignore - wailsjs path
import {
  DeleteSMIMECertificate,
  DeleteSenderCert,
  ImportRecipientCert,
  ImportSMIMECertificateFromPath,
  ImportSMIMECertificateFromPathBER,
  PickRecipientCertFile,
  PickSMIMECertificateFile,
  SetDefaultSMIMECertificate,
  SetSMIMEEncryptPolicy,
  SetSMIMESignPolicy,
} from '../../../../../wailsjs/go/app/App'
import { mapImportError } from './accountSecurityFormat'
import type { AccountSecurityState } from './accountSecurityState.svelte'

export async function handlePickAndImport(s: AccountSecurityState) {
  s.importError = ''
  try {
    const path = await PickSMIMECertificateFile()
    if (!path) return

    s.importFilePath = path
    s.showImportDialog = true
  } catch (err) {
    console.error('Failed to pick certificate file:', err)
  }
}

export async function handleImport(s: AccountSecurityState) {
  if (!s.importFilePath) return

  s.importing = true
  s.importError = ''
  try {
    const result = await ImportSMIMECertificateFromPath(s.accountId, s.importFilePath, s.importPassword)
    addToast({
      type: 'success',
      message: get(_)('security.certImported', { values: { count: result.chainLength } }),
    })
    s.showImportDialog = false
    s.importFilePath = ''
    s.importPassword = ''
    await s.loadData()
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err)
    if (msg.includes('indefinite length')) {
      s.showBERConfirmDialog = true
      s.importing = false
      return
    }
    s.importError = mapImportError(err, 'cert')
  } finally {
    s.importing = false
  }
}

export async function handleImportBER(s: AccountSecurityState) {
  s.showBERConfirmDialog = false
  s.importing = true
  s.importError = ''
  try {
    const result = await ImportSMIMECertificateFromPathBER(s.accountId, s.importFilePath, s.importPassword)
    addToast({
      type: 'success',
      message: get(_)('security.certImported', { values: { count: result.chainLength } }),
    })
    s.showImportDialog = false
    s.importFilePath = ''
    s.importPassword = ''
    await s.loadData()
  } catch (err) {
    s.importError = mapImportError(err, 'cert')
  } finally {
    s.importing = false
  }
}

export function handleCancelImport(s: AccountSecurityState) {
  s.showImportDialog = false
  s.importFilePath = ''
  s.importPassword = ''
  s.importError = ''
}

export async function handleDeleteCert(s: AccountSecurityState, certId: string) {
  try {
    await DeleteSMIMECertificate(certId)
    addToast({ type: 'success', message: get(_)('security.certRemoved') })
    await s.loadData()
  } catch (err) {
    console.error('Failed to remove certificate:', err)
    addToast({ type: 'error', message: get(_)('security.failedToRemoveCert') })
  }
}

export async function handleSetDefault(s: AccountSecurityState, certId: string) {
  try {
    await SetDefaultSMIMECertificate(s.accountId, certId)
    addToast({ type: 'success', message: get(_)('security.defaultCertUpdated') })
    await s.loadData()
  } catch (err) {
    console.error('Failed to set default certificate:', err)
    addToast({ type: 'error', message: get(_)('security.failedToSetDefaultCert') })
  }
}

export async function handleSignPolicyChange(s: AccountSecurityState, policy: string) {
  try {
    await SetSMIMESignPolicy(s.accountId, policy)
    s.signPolicy = policy
  } catch (err) {
    console.error('Failed to update signing policy:', err)
    addToast({ type: 'error', message: get(_)('security.failedToUpdateSignPolicy') })
  }
}

export async function handleEncryptPolicyChange(s: AccountSecurityState, policy: string) {
  try {
    await SetSMIMEEncryptPolicy(s.accountId, policy)
    s.encryptPolicy = policy
  } catch (err) {
    console.error('Failed to update encryption policy:', err)
    addToast({ type: 'error', message: get(_)('security.failedToUpdateEncryptPolicy') })
  }
}

export async function handlePickRecipientCert(s: AccountSecurityState) {
  s.recipientImportError = ''
  try {
    const path = await PickRecipientCertFile()
    if (!path) return
    s.recipientImportFilePath = path
    s.showRecipientImportDialog = true
  } catch (err) {
    console.error('Failed to pick recipient certificate file:', err)
  }
}

export async function handleImportRecipientCert(s: AccountSecurityState) {
  if (!s.recipientImportFilePath || !s.recipientImportEmail.trim()) {
    s.recipientImportError = get(_)('security.enterRecipientEmail')
    return
  }

  s.recipientImporting = true
  s.recipientImportError = ''
  try {
    await ImportRecipientCert(s.recipientImportEmail.trim(), s.recipientImportFilePath)
    addToast({ type: 'success', message: get(_)('security.recipientCertImported', { values: { email: s.recipientImportEmail.trim() } }) })
    s.showRecipientImportDialog = false
    s.recipientImportFilePath = ''
    s.recipientImportEmail = ''
    await s.loadData()
  } catch (err) {
    s.recipientImportError = mapImportError(err, 'recipientCert')
  } finally {
    s.recipientImporting = false
  }
}

export function handleCancelRecipientImport(s: AccountSecurityState) {
  s.showRecipientImportDialog = false
  s.recipientImportFilePath = ''
  s.recipientImportEmail = ''
  s.recipientImportError = ''
}

export async function handleDeleteSenderCert(s: AccountSecurityState, certId: string) {
  try {
    await DeleteSenderCert(certId)
    addToast({ type: 'success', message: get(_)('security.senderCertRemoved') })
    await s.loadData()
  } catch (err) {
    console.error('Failed to remove sender certificate:', err)
    addToast({ type: 'error', message: get(_)('security.failedToRemoveSenderCert') })
  }
}
