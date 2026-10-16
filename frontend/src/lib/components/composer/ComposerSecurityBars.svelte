<script lang="ts">
  import Icon from '@iconify/svelte'
  import Switch from '$lib/components/ui/switch/Switch.svelte'
  import type { ComposerState } from './composerState.svelte'
  import { _ } from '$lib/i18n'

  let { s }: { s: ComposerState } = $props()

</script>

<!-- Security toggles -->
{#if s.showPGPSignOption || s.showPGPEncryptOption}
  <div class="flex items-center px-4 py-3.5 border-b border-border text-xs {s.securityMode === 'pgp' ? 'bg-muted/50' : ''}">
    <div class="flex items-center gap-1.5">
      <Icon icon="mdi:lock-outline" class="w-3.5 h-3.5 text-muted-foreground shrink-0" />
      <span class="text-muted-foreground font-medium">PGP</span>
      {#if s.pgpKeyId}
        <span class="text-muted-foreground">|</span>
        <span class="text-muted-foreground font-mono">{s.pgpKeyId}</span>
      {/if}
    </div>
    <div class="flex items-center gap-3 ml-auto">
      {#if s.securityMode === 'pgp'}
        <span class="text-muted-foreground">{$_('composer.securityModeHint')}</span>
      {/if}
      {#if s.showPGPSignOption}
        <div class="flex items-center gap-1.5" title={$_('composer.pgpSign')}>
          <span>{$_('composer.sign')}</span>
          <Switch bind:checked={s.pgpSignMessage} onCheckedChange={(v) => { if (v) { s.signMessage = false } }} class="scale-75 origin-left" />
        </div>
      {/if}
      {#if s.showPGPEncryptOption}
        <div class="flex items-center gap-1.5" title={$_('composer.pgpEncrypt')}>
          <span>{$_('composer.encrypt')}</span>
          <Switch bind:checked={s.pgpEncryptMessage} onCheckedChange={(v) => { if (v) { s.encryptMessage = false } }} class="scale-75 origin-left" />
        </div>
      {/if}
    </div>
  </div>
{/if}
{#if s.showSignOption || s.showEncryptOption}
  <div class="flex items-center px-4 py-3.5 border-b border-border text-xs {s.securityMode === 'smime' ? 'bg-muted/50' : ''}">
    <div class="flex items-center gap-1.5">
      <Icon icon="mdi:shield-outline" class="w-3.5 h-3.5 text-muted-foreground shrink-0" />
      <span class="text-muted-foreground font-medium">S/MIME</span>
      {#if s.smimeCertFingerprint}
        <span class="text-muted-foreground">|</span>
        <span class="text-muted-foreground font-mono">{s.smimeCertFingerprint}</span>
      {/if}
    </div>
    <div class="flex items-center gap-3 ml-auto">
      {#if s.securityMode === 'smime'}
        <span class="text-muted-foreground">{$_('composer.securityModeHint')}</span>
      {/if}
      {#if s.showSignOption}
        <div class="flex items-center gap-1.5" title={$_('composer.smimeSign')}>
          <span>{$_('composer.sign')}</span>
          <Switch bind:checked={s.signMessage} onCheckedChange={(v) => { if (v) { s.pgpSignMessage = false } }} class="scale-75 origin-left" />
        </div>
      {/if}
      {#if s.showEncryptOption}
        <div class="flex items-center gap-1.5" title={$_('composer.smimeEncrypt')}>
          <span>{$_('composer.encrypt')}</span>
          <Switch bind:checked={s.encryptMessage} onCheckedChange={(v) => { if (v) { s.pgpEncryptMessage = false } }} class="scale-75 origin-left" />
        </div>
      {/if}
    </div>
  </div>
{/if}
