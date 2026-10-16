<script lang="ts">
  import Icon from '@iconify/svelte'
  import { Button } from '$lib/components/ui/button'
  import { _ } from '$lib/i18n'
  import type { AccountSecurityState } from './accountSecurityState.svelte'
  import { formatDate } from './accountSecurityFormat'
  import { handleAddKeyServer, handleDeletePGPKey, handleDeletePGPSenderKey, handleKeyLookup, handlePGPEncryptPolicyChange, handlePGPSignPolicyChange, handlePickAndImportPGP, handlePickPGPRecipientKey, handleRemoveKeyServer, handleSetDefaultPGP } from './accountSecurityPgp'

  let { s }: { s: AccountSecurityState } = $props()
</script>

<div class="space-y-4">
  <button
    class="w-full flex items-center gap-2 text-sm font-semibold text-foreground hover:text-primary transition-colors text-left"
    onclick={() => s.pgpCollapsed = !s.pgpCollapsed}
  >
    <Icon icon={s.pgpCollapsed ? 'mdi:chevron-right' : 'mdi:chevron-down'} class="w-4 h-4 shrink-0" />
    <Icon icon="mdi:key-outline" class="w-4 h-4" />
    {$_('security.pgp')}
    {#if s.pgpKeys.length > 0}
      <span class="text-2xs px-1.5 py-0.5 rounded bg-muted text-muted-foreground font-medium">{$_('security.keysCount', { values: { count: s.pgpKeys.length } })}</span>
    {/if}
  </button>

  {#if !s.pgpCollapsed}
  <!-- Your PGP Keys -->
  <div class="space-y-3">
    <div class="flex items-center justify-between">
      <h4 class="text-xs font-medium text-muted-foreground uppercase tracking-wider">{$_('security.yourKeys')}</h4>
      <Button variant="outline" size="sm" onclick={() => handlePickAndImportPGP(s)}>
        <Icon icon="mdi:key-plus" class="w-4 h-4 mr-1" />
        {$_('security.importSecretKey')}
      </Button>
    </div>

    {#if s.pgpKeys.length === 0}
      <p class="text-sm text-muted-foreground py-2">{$_('security.noPGPKeysHelp')}</p>
    {:else}
      <div class="space-y-2">
        {#each s.pgpKeys as key (key.id)}
          <div class="flex items-start gap-3 p-3 rounded-md border border-border bg-card">
            <div class="shrink-0 mt-0.5">
              {#if key.isExpired}
                <Icon icon="mdi:key-remove" class="w-5 h-5 text-destructive" />
              {:else}
                <Icon icon="mdi:key" class="w-5 h-5 text-green-600 dark:text-green-400" />
              {/if}
            </div>
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2">
                <span class="text-sm font-medium truncate">{key.email}</span>
                {#if key.isDefault}
                  <span class="text-2xs px-1.5 py-0.5 rounded bg-primary/10 text-primary font-medium">{$_('security.defaultBadge')}</span>
                {/if}
                {#if key.isExpired}
                  <span class="text-2xs px-1.5 py-0.5 rounded bg-destructive/10 text-destructive font-medium">{$_('security.expiredBadge')}</span>
                {/if}
              </div>
              <p class="text-xs text-muted-foreground truncate mt-0.5">{key.userId}</p>
              <p class="text-xs text-muted-foreground mt-0.5">
                {key.algorithm}{key.keySize ? ` ${key.keySize}-bit` : ''} &middot; {key.fingerprint?.slice(-16)}
              </p>
              <p class="text-xs text-muted-foreground">
                {$_('security.created')} {formatDate(key.createdAtKey)}{key.expiresAtKey ? ` · ${$_('security.expires')} ${formatDate(key.expiresAtKey)}` : ''}
              </p>
            </div>
            <div class="flex items-center gap-1 shrink-0">
              {#if !key.isDefault}
                <Button variant="ghost" size="sm" onclick={() => handleSetDefaultPGP(s, key.id)} title={$_('security.setAsDefault')}>
                  <Icon icon="mdi:star-outline" class="w-4 h-4" />
                </Button>
              {/if}
              <Button variant="ghost" size="sm" onclick={() => handleDeletePGPKey(s, key.id)} title={$_('security.removeKey')}>
                <Icon icon="mdi:delete-outline" class="w-4 h-4 text-destructive" />
              </Button>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>

  <!-- PGP Signing Policy -->
  {#if s.pgpKeys.length > 0}
    <div class="space-y-2">
      <h4 class="text-xs font-medium text-muted-foreground uppercase tracking-wider">{$_('security.signingPolicy')}</h4>
      <div class="flex items-center gap-4">
        <label class="flex items-center gap-2 text-sm cursor-pointer">
          <input
            type="radio"
            name="pgpSignPolicy"
            value="never"
            checked={s.pgpSignPolicy === 'never'}
            onchange={() => handlePGPSignPolicyChange(s, 'never')}
            class="accent-primary"
          />
          {$_('security.neverSignByDefault')}
        </label>
        <label class="flex items-center gap-2 text-sm cursor-pointer">
          <input
            type="radio"
            name="pgpSignPolicy"
            value="always"
            checked={s.pgpSignPolicy === 'always'}
            onchange={() => handlePGPSignPolicyChange(s, 'always')}
            class="accent-primary"
          />
          {$_('security.alwaysSignByDefault')}
        </label>
      </div>
      <p class="text-xs text-muted-foreground">{$_('security.policyOverrideHint')}</p>
    </div>

    <!-- PGP Encryption Policy -->
    <div class="space-y-2">
      <h4 class="text-xs font-medium text-muted-foreground uppercase tracking-wider">{$_('security.encryptionPolicy')}</h4>
      <div class="flex items-center gap-4">
        <label class="flex items-center gap-2 text-sm cursor-pointer">
          <input
            type="radio"
            name="pgpEncryptPolicy"
            value="never"
            checked={s.pgpEncryptPolicy === 'never'}
            onchange={() => handlePGPEncryptPolicyChange(s, 'never')}
            class="accent-primary"
          />
          {$_('security.neverEncryptByDefault')}
        </label>
        <label class="flex items-center gap-2 text-sm cursor-pointer">
          <input
            type="radio"
            name="pgpEncryptPolicy"
            value="always"
            checked={s.pgpEncryptPolicy === 'always'}
            onchange={() => handlePGPEncryptPolicyChange(s, 'always')}
            class="accent-primary"
          />
          {$_('security.alwaysEncryptByDefault')}
        </label>
      </div>
      <p class="text-xs text-muted-foreground">{$_('security.encryptRequiresRecipientKeys')}</p>
    </div>
  {/if}

  <!-- Key Servers -->
  <div class="space-y-3">
    <button
      class="w-full flex items-center gap-2 text-xs font-medium text-muted-foreground uppercase tracking-wider hover:text-foreground transition-colors text-left"
      onclick={() => s.keyServersCollapsed = !s.keyServersCollapsed}
    >
      <Icon icon={s.keyServersCollapsed ? 'mdi:chevron-right' : 'mdi:chevron-down'} class="w-3.5 h-3.5 shrink-0" />
      {$_('security.keyServersLabel')}
      <span class="text-2xs px-1.5 py-0.5 rounded bg-muted text-muted-foreground font-medium">{s.keyServers.length}</span>
    </button>

    {#if !s.keyServersCollapsed}
      {#if s.keyServers.length > 0}
        <div class="space-y-1">
          {#each s.keyServers as server (server.id)}
            <div class="flex items-center gap-3 p-2 rounded-md border border-border">
              <Icon icon="mdi:web" class="w-4 h-4 text-muted-foreground shrink-0" />
              <span class="text-sm flex-1 truncate">{server.url.replace('https://', '')}</span>
              <Button variant="ghost" size="sm" onclick={() => handleRemoveKeyServer(s, server.id)} title={$_('security.removeServer')}>
                <Icon icon="mdi:close" class="w-3.5 h-3.5" />
              </Button>
            </div>
          {/each}
        </div>
      {/if}

      <div class="flex items-center gap-2">
        <input
          type="url"
          bind:value={s.newKeyServerURL}
          placeholder="https://"
          class="flex-1 px-3 py-1.5 rounded-md border border-border bg-background text-sm focus:outline-hidden focus:ring-2 focus:ring-primary"
          onkeydown={(e) => { if (e.key === 'Enter') handleAddKeyServer(s) }}
        />
        <Button variant="outline" size="sm" onclick={() => handleAddKeyServer(s)} disabled={s.addingKeyServer || !s.newKeyServerURL.trim()}>
          {#if s.addingKeyServer}
            <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
          {:else}
            {$_('security.addButton')}
          {/if}
        </Button>
      </div>
      <p class="text-xs text-muted-foreground">{$_('security.keyServersHelp')}</p>
    {/if}
  </div>

  <!-- PGP Recipient Keys -->
  <div class="space-y-3">
    <div class="flex items-center justify-between">
      <h4 class="text-xs font-medium text-muted-foreground uppercase tracking-wider">{$_('security.recipientKeys')}</h4>
      <Button variant="outline" size="sm" onclick={() => handlePickPGPRecipientKey(s)}>
        <Icon icon="mdi:key-plus" class="w-4 h-4 mr-1" />
        {$_('security.importButton')}
      </Button>
    </div>

    <!-- Key Lookup (WKD + HKP) -->
    <div class="flex items-center gap-2">
      <input
        type="email"
        bind:value={s.keyLookupEmail}
        placeholder={$_('security.searchByEmail')}
        class="flex-1 px-3 py-1.5 rounded-md border border-border bg-background text-sm focus:outline-hidden focus:ring-2 focus:ring-primary"
        onkeydown={(e) => { if (e.key === 'Enter') handleKeyLookup(s) }}
      />
      <Button variant="outline" size="sm" onclick={() => handleKeyLookup(s)} disabled={s.keyLookupLoading || !s.keyLookupEmail.trim()}>
        {#if s.keyLookupLoading}
          <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
        {:else}
          <Icon icon="mdi:magnify" class="w-4 h-4" />
        {/if}
      </Button>
    </div>

    {#if s.pgpSenderKeys.length === 0}
      <p class="text-sm text-muted-foreground py-2">{$_('security.noRecipientPGPKeysHelp')}</p>
    {:else}
      <div class="space-y-2">
        {#each s.pgpSenderKeys as key (key.id)}
          <div class="flex items-center gap-3 p-2 rounded-md border border-border">
            <Icon icon="mdi:key-variant" class="w-4 h-4 text-muted-foreground shrink-0" />
            <div class="flex-1 min-w-0">
              <span class="text-sm truncate block">{key.email}</span>
              <span class="text-xs text-muted-foreground truncate block">{key.fingerprint?.slice(-16)} &middot; {key.algorithm}</span>
            </div>
            <span class="text-2xs px-1.5 py-0.5 rounded bg-muted text-muted-foreground font-medium shrink-0">
              {key.source}
            </span>
            <span class="text-xs text-muted-foreground shrink-0">
              {formatDate(key.lastSeenAt)}
            </span>
            <Button variant="ghost" size="sm" onclick={() => handleDeletePGPSenderKey(s, key.id)} title={$_('security.removeButton')}>
              <Icon icon="mdi:close" class="w-3.5 h-3.5" />
            </Button>
          </div>
        {/each}
      </div>
    {/if}
  </div>
  {/if}
</div>
