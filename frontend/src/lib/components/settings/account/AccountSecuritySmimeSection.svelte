<script lang="ts">
  import SMIMERevocationSetting from './SMIMERevocationSetting.svelte'
  import Icon from '@iconify/svelte'
  import { Button } from '$lib/components/ui/button'
  import { _ } from '$lib/i18n'
  import type { AccountSecurityState } from './accountSecurityState.svelte'
  import { formatDate } from './accountSecurityFormat'
  import { handleDeleteCert, handleDeleteSenderCert, handleEncryptPolicyChange, handlePickAndImport, handlePickRecipientCert, handleSetDefault, handleSignPolicyChange } from './accountSecuritySmime'

  let { s }: { s: AccountSecurityState } = $props()
</script>

<div class="space-y-4 mt-8 pt-6 border-t border-border">
  <button
    class="w-full flex items-center gap-2 text-sm font-semibold text-foreground hover:text-primary transition-colors text-left"
    onclick={() => s.smimeCollapsed = !s.smimeCollapsed}
  >
    <Icon icon={s.smimeCollapsed ? 'mdi:chevron-right' : 'mdi:chevron-down'} class="w-4 h-4 shrink-0" />
    <Icon icon="mdi:shield-lock-outline" class="w-4 h-4" />
    {$_('security.smime')}
    {#if s.certificates.length > 0}
      <span class="text-2xs px-1.5 py-0.5 rounded bg-muted text-muted-foreground font-medium">{$_('security.certsCount', { values: { count: s.certificates.length } })}</span>
    {/if}
  </button>

  {#if !s.smimeCollapsed}
  <SMIMERevocationSetting />
  <!-- Your Certificates -->
  <div class="space-y-3">
    <div class="flex items-center justify-between">
      <h4 class="text-xs font-medium text-muted-foreground uppercase tracking-wider">{$_('security.yourCertificates')}</h4>
      <Button variant="outline" size="sm" onclick={() => handlePickAndImport(s)}>
        <Icon icon="mdi:certificate" class="w-4 h-4 mr-1" />
        {$_('security.importP12')}
      </Button>
    </div>

    {#if s.certificates.length === 0}
      <p class="text-sm text-muted-foreground py-2">{$_('security.noSMIMECertsHelp')}</p>
    {:else}
      <div class="space-y-2">
        {#each s.certificates as cert (cert.id)}
          <div class="flex items-start gap-3 p-3 rounded-md border border-border bg-card">
            <div class="shrink-0 mt-0.5">
              {#if cert.isExpired}
                <Icon icon="mdi:certificate-outline" class="w-5 h-5 text-destructive" />
              {:else}
                <Icon icon="mdi:certificate" class="w-5 h-5 text-green-600 dark:text-green-400" />
              {/if}
            </div>
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2">
                <span class="text-sm font-medium truncate">{cert.email}</span>
                {#if cert.isDefault}
                  <span class="text-2xs px-1.5 py-0.5 rounded bg-primary/10 text-primary font-medium">{$_('security.defaultBadge')}</span>
                {/if}
                {#if cert.isExpired}
                  <span class="text-2xs px-1.5 py-0.5 rounded bg-destructive/10 text-destructive font-medium">{$_('security.expiredBadge')}</span>
                {/if}
                {#if cert.isSelfSigned}
                  <span class="text-2xs px-1.5 py-0.5 rounded bg-amber-500/10 text-amber-600 dark:text-amber-400 font-medium">{$_('security.selfSignedBadge')}</span>
                {/if}
              </div>
              <p class="text-xs text-muted-foreground truncate mt-0.5">{cert.subject}</p>
              <p class="text-xs text-muted-foreground mt-0.5">
                {$_('security.issuerLabel')} {cert.issuer}
              </p>
              <p class="text-xs text-muted-foreground">
                {$_('security.validLabel')} {formatDate(cert.notBefore)} - {formatDate(cert.notAfter)}
              </p>
            </div>
            <div class="flex items-center gap-1 shrink-0">
              {#if !cert.isDefault}
                <Button variant="ghost" size="sm" onclick={() => handleSetDefault(s, cert.id)} title={$_('security.setAsDefault')}>
                  <Icon icon="mdi:star-outline" class="w-4 h-4" />
                </Button>
              {/if}
              <Button variant="ghost" size="sm" onclick={() => handleDeleteCert(s, cert.id)} title={$_('security.removeCertificate')}>
                <Icon icon="mdi:delete-outline" class="w-4 h-4 text-destructive" />
              </Button>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>

  <!-- Signing Policy -->
  {#if s.certificates.length > 0}
    <div class="space-y-2">
      <h4 class="text-xs font-medium text-muted-foreground uppercase tracking-wider">{$_('security.signingPolicy')}</h4>
      <div class="flex items-center gap-4">
        <label class="flex items-center gap-2 text-sm cursor-pointer">
          <input
            type="radio"
            name="signPolicy"
            value="never"
            checked={s.signPolicy === 'never'}
            onchange={() => handleSignPolicyChange(s, 'never')}
            class="accent-primary"
          />
          {$_('security.neverSignByDefault')}
        </label>
        <label class="flex items-center gap-2 text-sm cursor-pointer">
          <input
            type="radio"
            name="signPolicy"
            value="always"
            checked={s.signPolicy === 'always'}
            onchange={() => handleSignPolicyChange(s, 'always')}
            class="accent-primary"
          />
          {$_('security.alwaysSignByDefault')}
        </label>
      </div>
      <p class="text-xs text-muted-foreground">{$_('security.policyOverrideHint')}</p>
    </div>

    <!-- Encryption Policy -->
    <div class="space-y-2">
      <h4 class="text-xs font-medium text-muted-foreground uppercase tracking-wider">{$_('security.encryptionPolicy')}</h4>
      <div class="flex items-center gap-4">
        <label class="flex items-center gap-2 text-sm cursor-pointer">
          <input
            type="radio"
            name="encryptPolicy"
            value="never"
            checked={s.encryptPolicy === 'never'}
            onchange={() => handleEncryptPolicyChange(s, 'never')}
            class="accent-primary"
          />
          {$_('security.neverEncryptByDefault')}
        </label>
        <label class="flex items-center gap-2 text-sm cursor-pointer">
          <input
            type="radio"
            name="encryptPolicy"
            value="always"
            checked={s.encryptPolicy === 'always'}
            onchange={() => handleEncryptPolicyChange(s, 'always')}
            class="accent-primary"
          />
          {$_('security.alwaysEncryptByDefault')}
        </label>
      </div>
      <p class="text-xs text-muted-foreground">{$_('security.encryptRequiresRecipientCerts')}</p>
    </div>
  {/if}

  <!-- Sender Certificates (auto-collected) -->
  <div class="space-y-3">
    <div class="flex items-center justify-between">
      <h4 class="text-xs font-medium text-muted-foreground uppercase tracking-wider">{$_('security.recipientCertificates')}</h4>
      <Button variant="outline" size="sm" onclick={() => handlePickRecipientCert(s)}>
        <Icon icon="mdi:certificate" class="w-4 h-4 mr-1" />
        {$_('security.importButton')}
      </Button>
    </div>
    {#if s.senderCerts.length === 0}
      <p class="text-sm text-muted-foreground py-2">{$_('security.noRecipientCertsHelp')}</p>
    {:else}
      <div class="space-y-2">
        {#each s.senderCerts as cert (cert.id)}
          <div class="flex items-center gap-3 p-2 rounded-md border border-border">
            <Icon icon="mdi:account-key-outline" class="w-4 h-4 text-muted-foreground shrink-0" />
            <div class="flex-1 min-w-0">
              <span class="text-sm truncate block">{cert.email}</span>
              <span class="text-xs text-muted-foreground truncate block">{cert.subject}</span>
            </div>
            <span class="text-xs text-muted-foreground shrink-0">
              {formatDate(cert.lastSeenAt)}
            </span>
            <Button variant="ghost" size="sm" onclick={() => handleDeleteSenderCert(s, cert.id)} title={$_('security.removeButton')}>
              <Icon icon="mdi:close" class="w-3.5 h-3.5" />
            </Button>
          </div>
        {/each}
      </div>
    {/if}
  </div>
  {/if}
</div>
