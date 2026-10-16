<script lang="ts">
  import Icon from '@iconify/svelte'
  import { Label } from '$lib/components/ui/label'
  import { Input } from '$lib/components/ui/input'
  import { Button } from '$lib/components/ui/button'
  import { _ } from '$lib/i18n'
  import type { ContactSourceState } from './contactSourceState.svelte'
  import ContactSourceSyncInterval from './ContactSourceSyncInterval.svelte'
  import { handleDiscover } from './contactSourceActions'

  let { s }: { s: ContactSourceState } = $props()
</script>

  <!-- CardDAV Form -->
  <div class="space-y-2">
    <Label for="name">{$_('contactSource.name')}</Label>
    <Input
      id="name"
      bind:value={s.name}
      placeholder={$_('contactSource.namePlaceholder')}
    />
  </div>

  {#if !s.editSource}
    <!-- Authentication method -->
    <div class="space-y-2">
      <Label>{$_('contactSource.authentication')}</Label>
      <div class="flex gap-2">
        <Button
          type="button"
          variant={!s.cardDavIsOAuth ? 'default' : 'outline'}
          size="sm"
          class="flex-1"
          onclick={() => s.setCardDavAuth('password')}
        >
          <Icon icon="mdi:key" class="w-4 h-4 mr-2" />
          {$_('contactSource.authPassword')}
        </Button>
        <Button
          type="button"
          variant={s.cardDavIsOAuth ? 'default' : 'outline'}
          size="sm"
          class="flex-1"
          onclick={() => s.setCardDavAuth('oauth')}
        >
          <Icon icon="mdi:shield-key-outline" class="w-4 h-4 mr-2" />
          {$_('contactSource.authOAuth')}
        </Button>
      </div>
    </div>
  {/if}

  <div class="space-y-2">
    <Label for="url">{$_('contactSource.serverUrl')}</Label>
    <Input
      id="url"
      bind:value={s.url}
      placeholder="https://cloud.example.com"
      disabled={!!s.editSource}
    />
    <p class="text-xs text-muted-foreground">
      {$_('contactSource.serverUrlHelp')}
    </p>
  </div>

  {#if s.cardDavIsOAuth}
    <!-- OAuth account: reuse a custom-OAuth mail account's token -->
    <div class="space-y-2">
      <Label>{$_('contactSource.oauthAccount')}</Label>
      {#if s.loadingCustomAccounts}
        <div class="flex items-center justify-center py-3">
          <Icon icon="mdi:loading" class="w-5 h-5 animate-spin text-muted-foreground" />
        </div>
      {:else if s.customOAuthAccounts.length === 0}
        <p class="text-xs text-muted-foreground">{$_('contactSource.noCustomAccounts')}</p>
      {:else}
        <div class="border border-border rounded-md divide-y divide-border">
          {#each s.customOAuthAccounts as acc (acc.accountId)}
            <button
              type="button"
              class="w-full flex items-center gap-3 p-3 text-left hover:bg-muted/50 transition-colors disabled:opacity-60"
              disabled={!!s.editSource}
              onclick={() => {
                s.selectedCustomAccountId = acc.accountId
                if (!s.name) s.name = $_('contactSource.autoName', { values: { name: acc.name || acc.email } })
              }}
            >
              <div class="w-4 h-4 border border-border rounded flex items-center justify-center {s.selectedCustomAccountId === acc.accountId ? 'bg-primary border-primary' : ''}">
                {#if s.selectedCustomAccountId === acc.accountId}
                  <Icon icon="mdi:check" class="w-3 h-3 text-primary-foreground" />
                {/if}
              </div>
              <div class="flex-1 min-w-0">
                <div class="font-medium text-sm truncate">{acc.name || acc.email}</div>
                <div class="text-xs text-muted-foreground truncate">{acc.email}</div>
              </div>
            </button>
          {/each}
        </div>
      {/if}
    </div>
  {:else}
    <div class="space-y-2">
      <Label for="username">{$_('contactSource.username')}</Label>
      <Input
        id="username"
        bind:value={s.username}
        placeholder="your@email.com"
        disabled={!!s.editSource}
      />
    </div>

    <div class="space-y-2">
      <Label for="password">{s.editSource ? $_('contactSource.passwordKeepCurrent') : $_('contactSource.password')}</Label>
      <Input
        id="password"
        type="password"
        bind:value={s.password}
        placeholder={s.editSource ? '********' : $_('contactSource.password')}
      />
    </div>
  {/if}

  <Button
    variant="outline"
    class="w-full"
    onclick={() => handleDiscover(s)}
    disabled={s.discovering || !s.url || (s.cardDavIsOAuth ? !s.selectedCustomAccountId : (!s.username || !s.password))}
  >
    {#if s.discovering}
      <Icon icon="mdi:loading" class="w-4 h-4 mr-2 animate-spin" />
      {$_('contactSource.discovering')}
    {:else}
      <Icon icon="mdi:connection" class="w-4 h-4 mr-2" />
      {s.hasDiscovered ? $_('contactSource.reDiscover') : $_('contactSource.connectDiscover')}
    {/if}
  </Button>

  {#if s.discoveryError}
    <div class="p-3 bg-destructive/10 border border-destructive/30 rounded-md text-sm text-destructive">
      {s.discoveryError}
    </div>
  {/if}

  {#if s.hasDiscovered && s.discoveredAddressbooks.length > 0}
    <div class="space-y-2">
      <Label>{$_('contactSource.addressbooksToSync')}</Label>
      <div class="border border-border rounded-md divide-y divide-border max-h-40 overflow-y-auto">
        {#each s.discoveredAddressbooks as ab (ab.path)}
          <button
            type="button"
            class="w-full flex items-center gap-3 p-3 text-left hover:bg-muted/50 transition-colors"
            onclick={() => s.toggleAddressbook(ab.path)}
          >
            <div class="w-4 h-4 border border-border rounded flex items-center justify-center {s.selectedAddressbooks.has(ab.path) ? 'bg-primary border-primary' : ''}">
              {#if s.selectedAddressbooks.has(ab.path)}
                <Icon icon="mdi:check" class="w-3 h-3 text-primary-foreground" />
              {/if}
            </div>
            <div class="flex-1 min-w-0">
              <div class="font-medium text-sm truncate">{ab.name || ab.path}</div>
              {#if ab.description}
                <div class="text-xs text-muted-foreground truncate">{ab.description}</div>
              {/if}
            </div>
          </button>
        {/each}
      </div>
    </div>

    <ContactSourceSyncInterval {s} />

    <!-- Writable toggle moved out of the source-edit dialog into the
         Contacts extension settings (Settings → Extensions → Contacts
         → Write Access) for symmetric Enable/Disable + OAuth-consent
         handling across all source types. -->
  {/if}
