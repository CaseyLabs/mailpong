<script lang="ts">
  import Icon from '@iconify/svelte'
  import { Button } from '$lib/components/ui/button'
  import { _ } from '$lib/i18n'
  import type { AccountSecurityState } from './accountSecurityState.svelte'
  import { getFileName } from './accountSecurityFormat'
  import { handleCancelImport, handleCancelRecipientImport, handleImport, handleImportBER, handleImportRecipientCert } from './accountSecuritySmime'
  import { handleCancelPGPImport, handleCancelPGPRecipientImport, handleImportPGP, handleImportPGPRecipientKey } from './accountSecurityPgp'

  let { s }: { s: AccountSecurityState } = $props()
</script>

<!-- Import Dialog -->
{#if s.showImportDialog}
  <div class="fixed inset-0 z-50 flex items-center justify-center">
    <div role="button" tabindex="-1" class="absolute inset-0 bg-black/50" onclick={() => handleCancelImport(s)} onkeydown={(e) => { if (e.key === 'Escape') handleCancelImport(s) }}></div>
    <div class="relative bg-background border border-border rounded-lg shadow-xl p-6 w-full max-w-md mx-4">
      <h3 class="text-lg font-semibold mb-4">{$_('security.importCertificateTitle')}</h3>

      <div class="space-y-4">
        <div>
          <p class="text-sm text-muted-foreground mb-1">{$_('security.fileLabel')}</p>
          <p class="text-sm font-mono bg-muted/50 px-3 py-2 rounded truncate">{getFileName(s.importFilePath)}</p>
        </div>

        <div>
          <label for="cert-password" class="text-sm text-muted-foreground block mb-1">
            {$_('security.certificatePassword')}
          </label>
          <input
            id="cert-password"
            type="password"
            bind:value={s.importPassword}
            placeholder={$_('security.certificatePasswordPlaceholder')}
            class="w-full px-3 py-2 rounded-md border border-border bg-background text-sm focus:outline-hidden focus:ring-2 focus:ring-primary"
            onkeydown={(e) => { if (e.key === 'Enter') handleImport(s) }}
          />
          <p class="text-xs text-muted-foreground mt-1">{$_('security.certificatePasswordHelp')}</p>
        </div>

        {#if s.importError}
          <div class="text-sm text-destructive bg-destructive/10 px-3 py-2 rounded-md">
            {s.importError}
          </div>
        {/if}
      </div>

      <div class="flex items-center justify-end gap-2 mt-6">
        <Button variant="ghost" onclick={() => handleCancelImport(s)} disabled={s.importing}>
          {$_('common.cancel')}
        </Button>
        <Button onclick={() => handleImport(s)} disabled={s.importing}>
          {#if s.importing}
            <Icon icon="mdi:loading" class="w-4 h-4 mr-2 animate-spin" />
          {/if}
          {$_('security.importButton')}
        </Button>
      </div>
    </div>
  </div>
{/if}

<!-- BER Encoding Confirmation Dialog -->
{#if s.showBERConfirmDialog}
  <div class="fixed inset-0 z-50 flex items-center justify-center">
    <div role="button" tabindex="-1" class="absolute inset-0 bg-black/50" onclick={() => { s.showBERConfirmDialog = false }} onkeydown={(e) => { if (e.key === 'Escape') s.showBERConfirmDialog = false }}></div>
    <div class="relative bg-background border border-border rounded-lg shadow-lg p-6 w-full max-w-md">
      <h3 class="text-lg font-semibold mb-4">{$_('security.berConversionTitle')}</h3>
      <p class="text-sm text-muted-foreground mb-6">{$_('security.berConversionMessage')}</p>
      <div class="flex items-center justify-end gap-2">
        <Button variant="ghost" onclick={() => { s.showBERConfirmDialog = false }}>
          {$_('common.cancel')}
        </Button>
        <Button onclick={() => handleImportBER(s)}>
          {#if s.importing}
            <Icon icon="mdi:loading" class="w-4 h-4 mr-2 animate-spin" />
          {/if}
          {$_('security.berConversionConfirm')}
        </Button>
      </div>
    </div>
  </div>
{/if}

<!-- Recipient Cert Import Dialog -->
{#if s.showRecipientImportDialog}
  <div class="fixed inset-0 z-50 flex items-center justify-center">
    <div role="button" tabindex="-1" class="absolute inset-0 bg-black/50" onclick={() => handleCancelRecipientImport(s)} onkeydown={(e) => { if (e.key === 'Escape') handleCancelRecipientImport(s) }}></div>
    <div class="relative bg-background border border-border rounded-lg shadow-xl p-6 w-full max-w-md mx-4">
      <h3 class="text-lg font-semibold mb-4">{$_('security.importRecipientCertTitle')}</h3>

      <div class="space-y-4">
        <div>
          <p class="text-sm text-muted-foreground mb-1">{$_('security.fileLabel')}</p>
          <p class="text-sm font-mono bg-muted/50 px-3 py-2 rounded truncate">{getFileName(s.recipientImportFilePath)}</p>
        </div>

        <div>
          <label for="recipient-email" class="text-sm text-muted-foreground block mb-1">
            {$_('security.recipientEmailAddress')}
          </label>
          <input
            id="recipient-email"
            type="email"
            bind:value={s.recipientImportEmail}
            placeholder="recipient@example.com"
            class="w-full px-3 py-2 rounded-md border border-border bg-background text-sm focus:outline-hidden focus:ring-2 focus:ring-primary"
            onkeydown={(e) => { if (e.key === 'Enter') handleImportRecipientCert(s) }}
          />
          <p class="text-xs text-muted-foreground mt-1">{$_('security.recipientEmailHelp')}</p>
        </div>

        {#if s.recipientImportError}
          <div class="text-sm text-destructive bg-destructive/10 px-3 py-2 rounded-md">
            {s.recipientImportError}
          </div>
        {/if}
      </div>

      <div class="flex items-center justify-end gap-2 mt-6">
        <Button variant="ghost" onclick={() => handleCancelRecipientImport(s)} disabled={s.recipientImporting}>
          {$_('common.cancel')}
        </Button>
        <Button onclick={() => handleImportRecipientCert(s)} disabled={s.recipientImporting}>
          {#if s.recipientImporting}
            <Icon icon="mdi:loading" class="w-4 h-4 mr-2 animate-spin" />
          {/if}
          {$_('security.importButton')}
        </Button>
      </div>
    </div>
  </div>
{/if}

<!-- PGP Key Import Dialog -->
{#if s.showPGPImportDialog}
  <div class="fixed inset-0 z-50 flex items-center justify-center">
    <div role="button" tabindex="-1" class="absolute inset-0 bg-black/50" onclick={() => handleCancelPGPImport(s)} onkeydown={(e) => { if (e.key === 'Escape') handleCancelPGPImport(s) }}></div>
    <div class="relative bg-background border border-border rounded-lg shadow-xl p-6 w-full max-w-md mx-4">
      <h3 class="text-lg font-semibold mb-4">{$_('security.importPGPKeyTitle')}</h3>

      <div class="space-y-4">
        <div>
          <p class="text-sm text-muted-foreground mb-1">{$_('security.fileLabel')}</p>
          <p class="text-sm font-mono bg-muted/50 px-3 py-2 rounded truncate">{getFileName(s.pgpImportFilePath)}</p>
        </div>

        <div>
          <label for="pgp-passphrase" class="text-sm text-muted-foreground block mb-1">
            {$_('security.keyPassphrase')}
          </label>
          <input
            id="pgp-passphrase"
            type="password"
            bind:value={s.pgpImportPassphrase}
            placeholder={$_('security.keyPassphrasePlaceholder')}
            class="w-full px-3 py-2 rounded-md border border-border bg-background text-sm focus:outline-hidden focus:ring-2 focus:ring-primary"
            onkeydown={(e) => { if (e.key === 'Enter') handleImportPGP(s) }}
          />
          <p class="text-xs text-muted-foreground mt-1">{$_('security.keyPassphraseHelp')}</p>
        </div>

        {#if s.pgpImportError}
          <div class="text-sm text-destructive bg-destructive/10 px-3 py-2 rounded-md">
            {s.pgpImportError}
          </div>
        {/if}
      </div>

      <div class="flex items-center justify-end gap-2 mt-6">
        <Button variant="ghost" onclick={() => handleCancelPGPImport(s)} disabled={s.pgpImporting}>
          {$_('common.cancel')}
        </Button>
        <Button onclick={() => handleImportPGP(s)} disabled={s.pgpImporting}>
          {#if s.pgpImporting}
            <Icon icon="mdi:loading" class="w-4 h-4 mr-2 animate-spin" />
          {/if}
          {$_('security.importButton')}
        </Button>
      </div>
    </div>
  </div>
{/if}

<!-- PGP Recipient Key Import Dialog -->
{#if s.showPGPRecipientImportDialog}
  <div class="fixed inset-0 z-50 flex items-center justify-center">
    <div role="button" tabindex="-1" class="absolute inset-0 bg-black/50" onclick={() => handleCancelPGPRecipientImport(s)} onkeydown={(e) => { if (e.key === 'Escape') handleCancelPGPRecipientImport(s) }}></div>
    <div class="relative bg-background border border-border rounded-lg shadow-xl p-6 w-full max-w-md mx-4">
      <h3 class="text-lg font-semibold mb-4">{$_('security.importRecipientPGPKeyTitle')}</h3>

      <div class="space-y-4">
        <div>
          <p class="text-sm text-muted-foreground mb-1">{$_('security.fileLabel')}</p>
          <p class="text-sm font-mono bg-muted/50 px-3 py-2 rounded truncate">{getFileName(s.pgpRecipientImportFilePath)}</p>
        </div>

        <div>
          <label for="pgp-recipient-email" class="text-sm text-muted-foreground block mb-1">
            {$_('security.recipientEmailAddress')}
          </label>
          <input
            id="pgp-recipient-email"
            type="email"
            bind:value={s.pgpRecipientImportEmail}
            placeholder="recipient@example.com"
            class="w-full px-3 py-2 rounded-md border border-border bg-background text-sm focus:outline-hidden focus:ring-2 focus:ring-primary"
            onkeydown={(e) => { if (e.key === 'Enter') handleImportPGPRecipientKey(s) }}
          />
          <p class="text-xs text-muted-foreground mt-1">{$_('security.recipientEmailHelpKey')}</p>
        </div>

        {#if s.pgpRecipientImportError}
          <div class="text-sm text-destructive bg-destructive/10 px-3 py-2 rounded-md">
            {s.pgpRecipientImportError}
          </div>
        {/if}
      </div>

      <div class="flex items-center justify-end gap-2 mt-6">
        <Button variant="ghost" onclick={() => handleCancelPGPRecipientImport(s)} disabled={s.pgpRecipientImporting}>
          {$_('common.cancel')}
        </Button>
        <Button onclick={() => handleImportPGPRecipientKey(s)} disabled={s.pgpRecipientImporting}>
          {#if s.pgpRecipientImporting}
            <Icon icon="mdi:loading" class="w-4 h-4 mr-2 animate-spin" />
          {/if}
          {$_('security.importButton')}
        </Button>
      </div>
    </div>
  </div>
{/if}
