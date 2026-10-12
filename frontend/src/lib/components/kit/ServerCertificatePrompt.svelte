<script lang="ts">
  // Certificate-accept prompt for setup flows that connect over HTTPS
  // (CardDAV, CalDAV). Mount once, then call offerTrust() from the flow's
  // error handler: when the failure was an untrusted certificate it shows
  // CertificateDialog and pins trust to the exact host that presented the
  // certificate (which may be a redirect target, not the URL's host).
  import CertificateDialog from '$lib/components/settings/CertificateDialog.svelte'
  import { AcceptCertificate, GetServerCertificateFailure } from '$wailsjs/go/app/App'
  import type { certificate } from '$wailsjs/go/models'

  let open = $state(false)
  let cert = $state<certificate.CertificateInfo | null>(null)
  let host = ''
  let resolveChoice: ((accepted: boolean) => void) | null = null

  /**
   * Resolves true once the user accepted the server's certificate, so the
   * caller should retry; false when the error was not a certificate problem,
   * no untrusted certificate was found, or the user declined.
   */
  export async function offerTrust(err: unknown): Promise<boolean> {
    const msg = err instanceof Error ? err.message : String(err)
    let check
    try {
      check = await GetServerCertificateFailure(msg)
    } catch (e) {
      console.error('Server certificate check failed:', e)
      return false
    }
    if (!check.certificateRequired || !check.certificate || !check.host) return false
    // A newer prompt replaces one still open; settle the earlier caller as
    // declined so its await doesn't hang.
    resolveChoice?.(false)
    host = check.host
    cert = check.certificate
    open = true
    return new Promise((resolve) => (resolveChoice = resolve))
  }

  async function finish(permanent: boolean | null) {
    const resolve = resolveChoice
    const pending = cert
    resolveChoice = null
    open = false
    cert = null
    if (!resolve) return
    if (permanent === null || !pending) {
      resolve(false)
      return
    }
    try {
      await AcceptCertificate(host, pending, permanent)
      resolve(true)
    } catch (e) {
      console.error('Failed to accept certificate:', e)
      resolve(false)
    }
  }

  // Closing the dialog without a button (Escape) counts as declining.
  $effect(() => {
    if (!open && resolveChoice) finish(null)
  })
</script>

<CertificateDialog
  bind:open
  certificate={cert}
  onAcceptOnce={() => finish(false)}
  onAcceptPermanently={() => finish(true)}
  onDecline={() => finish(null)}
/>
