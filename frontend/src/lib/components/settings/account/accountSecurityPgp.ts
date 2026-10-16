// Behavior of the account Security tab (moved from AccountSecurityTab.svelte).
import { addToast } from '$lib/stores/toast'
import { get } from 'svelte/store'
import { _ } from '$lib/i18n'
// @ts-ignore - wailsjs path
import {
  AddPGPKeyServer,
  DeletePGPKey,
  DeletePGPSenderKey,
  ImportPGPKeyFromPath,
  ImportRecipientPGPKey,
  LookupPGPKey,
  PickPGPKeyFile,
  PickRecipientPGPKeyFile,
  RemovePGPKeyServer,
  SetDefaultPGPKey,
  SetPGPEncryptPolicy,
  SetPGPSignPolicy,
} from '../../../../../wailsjs/go/app/App'
import { mapImportError } from './accountSecurityFormat'
import type { AccountSecurityState } from './accountSecurityState.svelte'

// PGP handlers
export async function handlePickAndImportPGP(s: AccountSecurityState) {
  s.pgpImportError = ''
  try {
    const path = await PickPGPKeyFile()
    if (!path) return
    s.pgpImportFilePath = path
    s.showPGPImportDialog = true
  } catch (err) {
    console.error('Failed to pick PGP key file:', err)
  }
}

export async function handleImportPGP(s: AccountSecurityState) {
  if (!s.pgpImportFilePath) return
  s.pgpImporting = true
  s.pgpImportError = ''
  try {
    await ImportPGPKeyFromPath(s.accountId, s.pgpImportFilePath, s.pgpImportPassphrase)
    addToast({ type: 'success', message: get(_)('security.pgpKeyImported') })
    s.showPGPImportDialog = false
    s.pgpImportFilePath = ''
    s.pgpImportPassphrase = ''
    await s.loadData()
  } catch (err) {
    s.pgpImportError = mapImportError(err, 'key')
  } finally {
    s.pgpImporting = false
  }
}

export function handleCancelPGPImport(s: AccountSecurityState) {
  s.showPGPImportDialog = false
  s.pgpImportFilePath = ''
  s.pgpImportPassphrase = ''
  s.pgpImportError = ''
}

export async function handleDeletePGPKey(s: AccountSecurityState, keyId: string) {
  try {
    await DeletePGPKey(keyId)
    addToast({ type: 'success', message: get(_)('security.pgpKeyRemoved') })
    await s.loadData()
  } catch (err) {
    console.error('Failed to remove PGP key:', err)
    addToast({ type: 'error', message: get(_)('security.failedToRemovePGPKey') })
  }
}

export async function handleSetDefaultPGP(s: AccountSecurityState, keyId: string) {
  try {
    await SetDefaultPGPKey(s.accountId, keyId)
    addToast({ type: 'success', message: get(_)('security.defaultPGPKeyUpdated') })
    await s.loadData()
  } catch (err) {
    console.error('Failed to set default PGP key:', err)
    addToast({ type: 'error', message: get(_)('security.failedToSetDefaultPGPKey') })
  }
}

export async function handlePGPSignPolicyChange(s: AccountSecurityState, policy: string) {
  try {
    await SetPGPSignPolicy(s.accountId, policy)
    s.pgpSignPolicy = policy
  } catch (err) {
    console.error('Failed to update PGP signing policy:', err)
    addToast({ type: 'error', message: get(_)('security.failedToUpdatePGPSignPolicy') })
  }
}

export async function handlePGPEncryptPolicyChange(s: AccountSecurityState, policy: string) {
  try {
    await SetPGPEncryptPolicy(s.accountId, policy)
    s.pgpEncryptPolicy = policy
  } catch (err) {
    console.error('Failed to update PGP encryption policy:', err)
    addToast({ type: 'error', message: get(_)('security.failedToUpdatePGPEncryptPolicy') })
  }
}

export async function handlePickPGPRecipientKey(s: AccountSecurityState) {
  s.pgpRecipientImportError = ''
  try {
    const path = await PickRecipientPGPKeyFile()
    if (!path) return
    s.pgpRecipientImportFilePath = path
    s.showPGPRecipientImportDialog = true
  } catch (err) {
    console.error('Failed to pick recipient PGP key file:', err)
  }
}

export async function handleImportPGPRecipientKey(s: AccountSecurityState) {
  if (!s.pgpRecipientImportFilePath || !s.pgpRecipientImportEmail.trim()) {
    s.pgpRecipientImportError = get(_)('security.enterRecipientEmail')
    return
  }
  s.pgpRecipientImporting = true
  s.pgpRecipientImportError = ''
  try {
    await ImportRecipientPGPKey(s.pgpRecipientImportEmail.trim(), s.pgpRecipientImportFilePath)
    addToast({ type: 'success', message: get(_)('security.recipientPGPKeyImported', { values: { email: s.pgpRecipientImportEmail.trim() } }) })
    s.showPGPRecipientImportDialog = false
    s.pgpRecipientImportFilePath = ''
    s.pgpRecipientImportEmail = ''
    await s.loadData()
  } catch (err) {
    s.pgpRecipientImportError = mapImportError(err, 'recipientKey')
  } finally {
    s.pgpRecipientImporting = false
  }
}

export function handleCancelPGPRecipientImport(s: AccountSecurityState) {
  s.showPGPRecipientImportDialog = false
  s.pgpRecipientImportFilePath = ''
  s.pgpRecipientImportEmail = ''
  s.pgpRecipientImportError = ''
}

export async function handleDeletePGPSenderKey(s: AccountSecurityState, keyId: string) {
  try {
    await DeletePGPSenderKey(keyId)
    addToast({ type: 'success', message: get(_)('security.recipientPGPKeyRemoved') })
    await s.loadData()
  } catch (err) {
    console.error('Failed to remove recipient PGP key:', err)
    addToast({ type: 'error', message: get(_)('security.failedToRemoveRecipientPGPKey') })
  }
}

export async function handleAddKeyServer(s: AccountSecurityState) {
  const url = s.newKeyServerURL.trim()
  if (!url) return
  s.addingKeyServer = true
  try {
    await AddPGPKeyServer(url)
    addToast({ type: 'success', message: get(_)('security.keyServerAdded') })
    s.newKeyServerURL = ''
    await s.loadData()
  } catch (err) {
    console.error('Failed to add key server:', err)
    addToast({ type: 'error', message: get(_)('security.failedToAddKeyServer') })
  } finally {
    s.addingKeyServer = false
  }
}

export async function handleRemoveKeyServer(s: AccountSecurityState, id: number) {
  try {
    await RemovePGPKeyServer(id)
    addToast({ type: 'success', message: get(_)('security.keyServerRemoved') })
    await s.loadData()
  } catch (err) {
    console.error('Failed to remove key server:', err)
    addToast({ type: 'error', message: get(_)('security.failedToRemoveKeyServer') })
  }
}

export async function handleKeyLookup(s: AccountSecurityState) {
  if (!s.keyLookupEmail.trim()) return
  s.keyLookupLoading = true
  try {
    const armored = await LookupPGPKey(s.keyLookupEmail.trim())
    if (armored) {
      addToast({ type: 'success', message: get(_)('security.pgpKeyFound', { values: { email: s.keyLookupEmail.trim() } }) })
      s.keyLookupEmail = ''
      await s.loadData()
    } else {
      addToast({ type: 'info', message: get(_)('security.pgpKeyNotFound', { values: { email: s.keyLookupEmail.trim() } }) })
    }
  } catch (err) {
    console.error('Failed to look up PGP key:', err)
    addToast({ type: 'error', message: get(_)('security.keyLookupFailed') })
  } finally {
    s.keyLookupLoading = false
  }
}
