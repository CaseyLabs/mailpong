// Formatting and error mapping helpers for the account Security tab.
import { get } from 'svelte/store'
import { _ } from '$lib/i18n'

export function formatDate(dateStr: any): string {
  if (!dateStr) return 'N/A'
  try {
    const d = new Date(dateStr)
    return d.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
  } catch {
    return 'N/A'
  }
}

export function mapImportError(err: unknown, type: 'cert' | 'key' | 'recipientCert' | 'recipientKey'): string {
  const msg = err instanceof Error ? err.message : String(err)
  console.error(`Import ${type} error:`, msg)
  if (msg.includes('does not match this account')) {
    return type.includes('key') ? get(_)('security.keyEmailMismatch') : get(_)('security.certEmailMismatch')
  }
  // Include the actual error detail for diagnosis
  const detail = msg.length > 0 ? `: ${msg}` : ''
  switch (type) {
    case 'cert': return get(_)('security.certImportFailed') + detail
    case 'key': return get(_)('security.keyImportFailed') + detail
    case 'recipientCert': return get(_)('security.recipientCertImportFailed') + detail
    case 'recipientKey': return get(_)('security.recipientKeyImportFailed') + detail
  }
}

export function getFileName(path: string): string {
  return path.split('/').pop() || path.split('\\').pop() || path
}
