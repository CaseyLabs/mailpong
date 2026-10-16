<script lang="ts">
  import Icon from '@iconify/svelte'
  // @ts-ignore - wailsjs path
  import { certificate } from '../../../../../wailsjs/go/models'
  // @ts-ignore - wailsjs path
  import { GetAllTrustedCertificates, RemoveTrustedCertificateForHost } from '../../../../../wailsjs/go/app/App'
  import { Button } from '$lib/components/ui/button'
  import { _ } from '$lib/i18n'
  import ConfirmDialog from '$lib/components/ui/confirm-dialog/ConfirmDialog.svelte'

  // Trusted certificates state
  let showTrustedCerts = $state(false)
  let loadingCerts = $state(false)
  let trustedCerts = $state<certificate.CertificateInfo[]>([])
  let confirmRemoveFingerprint = $state<string | null>(null)
  let confirmRemoveHost = $state('')
  let showRemoveConfirm = $state(false)

  function handleTrustedCertsToggle() {
    showTrustedCerts = !showTrustedCerts
    if (showTrustedCerts) {
      loadTrustedCerts()
    }
  }

  async function loadTrustedCerts() {
    loadingCerts = true
    try {
      const result = await GetAllTrustedCertificates()
      trustedCerts = result || []
    } catch (err) {
      console.error('Failed to load trusted certificates:', err)
      trustedCerts = []
    } finally {
      loadingCerts = false
    }
  }

  function handleRemoveCert(host: string, fingerprint: string) {
    confirmRemoveFingerprint = fingerprint
    confirmRemoveHost = host
    showRemoveConfirm = true
  }

  async function confirmRemoveCert() {
    if (!confirmRemoveFingerprint) return
    try {
      await RemoveTrustedCertificateForHost(confirmRemoveHost, confirmRemoveFingerprint)
      trustedCerts = trustedCerts.filter(c => c.host !== confirmRemoveHost || c.fingerprint !== confirmRemoveFingerprint)
    } catch (err) {
      console.error('Failed to remove certificate:', err)
    }
    showRemoveConfirm = false
    confirmRemoveFingerprint = null
  }

  function formatFingerprint(fp: string): string {
    if (!fp) return ''
    const parts: string[] = []
    for (let i = 0; i < fp.length && i < 16; i += 2) {
      parts.push(fp.substring(i, i + 2).toUpperCase())
    }
    return parts.join(':') + '...'
  }

  function formatCertDate(iso: string): string {
    if (!iso) return 'N/A'
    try {
      return new Date(iso).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
    } catch {
      return iso
    }
  }
</script>

  <!-- Trusted Certificates -->
  <div class="space-y-2">
    <button
      type="button"
      class="flex items-center gap-2 text-sm font-medium hover:text-primary transition-colors"
      onclick={handleTrustedCertsToggle}
    >
      <Icon
        icon={showTrustedCerts ? 'mdi:chevron-down' : 'mdi:chevron-right'}
        class="w-4 h-4"
      />
      <Icon icon="mdi:shield-lock-outline" class="w-4 h-4" />
      {$_('account.trustedCertificates')}
    </button>

    {#if showTrustedCerts}
      <div class="space-y-3 pl-6 pt-2 border-l border-border ml-2">
        <p class="text-xs text-muted-foreground">
          {$_('account.trustedCertsHelp')}
        </p>

        {#if loadingCerts}
          <div class="flex items-center gap-2 text-sm text-muted-foreground">
            <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
            {$_('account.loadingCerts')}
          </div>
        {:else if trustedCerts.length === 0}
          <p class="text-sm text-muted-foreground">
            {$_('account.noTrustedCerts')}
          </p>
        {:else}
          <div class="space-y-3">
            {#each trustedCerts as cert (`${cert.host}:${cert.fingerprint}`)}
              <div class="flex items-start justify-between gap-3 rounded-lg border bg-muted/30 p-3">
                <div class="space-y-1 min-w-0">
                  <div class="flex items-center gap-2">
                    <Icon icon="mdi:shield-check-outline" class="w-4 h-4 text-muted-foreground shrink-0" />
                    <span class="text-sm font-medium truncate">{cert.subject}</span>
                  </div>
                  <div class="text-xs text-muted-foreground space-y-0.5 pl-6">
                    <p>{cert.host}</p>
                    <p>{$_('account.certFingerprint')} <span class="font-mono">{formatFingerprint(cert.fingerprint)}</span></p>
                    <p>{$_('account.certExpires')} {formatCertDate(cert.notAfter)}</p>
                  </div>
                </div>
                <Button
                  variant="ghost"
                  size="sm"
                  class="shrink-0 text-destructive hover:text-destructive hover:bg-destructive/10"
                  onclick={() => handleRemoveCert(cert.host ?? '', cert.fingerprint)}
                >
                  {$_('common.remove')}
                </Button>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {/if}
  </div>

<ConfirmDialog
  bind:open={showRemoveConfirm}
  title={$_('account.removeTrustedCert')}
  description={$_('account.removeTrustedCertDescription')}
  confirmLabel={$_('common.remove')}
  onConfirm={confirmRemoveCert}
/>
