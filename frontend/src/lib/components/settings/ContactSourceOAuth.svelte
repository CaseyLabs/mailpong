<script lang="ts">
  import Icon from '@iconify/svelte'
  import { Label } from '$lib/components/ui/label'
  import { Input } from '$lib/components/ui/input'
  import { Button } from '$lib/components/ui/button'
  import { _ } from '$lib/i18n'
  import type { ContactSourceState } from './contactSourceState.svelte'
  import ContactSourceSyncInterval from './ContactSourceSyncInterval.svelte'
  import { contactSourcesStore } from '$lib/stores/contactSources.svelte'
  import { handleStartOAuth } from './contactSourceActions'

  let { s }: { s: ContactSourceState } = $props()
</script>

  <!-- Google or Microsoft OAuth Form -->
  {#if s.editSource && s.isEditingOAuthSource}
    <!-- Editing existing OAuth source -->
    <div class="space-y-2">
      <Label for="name">{$_('contactSource.name')}</Label>
      <Input
        id="name"
        bind:value={s.name}
        placeholder={$_('contactSource.sourceName')}
      />
    </div>

    <ContactSourceSyncInterval {s} />

    <!-- Writable toggle lives in the Contacts extension settings now
         (Settings → Extensions → Contacts → Write Access). Same UI
         handles both enable (CardDAV: flag flip; Google/MS: OAuth
         consent) and disable for all source types. -->

  {:else}
    <!-- New OAuth source -->
    {#if s.loadingAccounts}
      <div class="flex items-center justify-center py-8">
        <Icon icon="mdi:loading" class="w-6 h-6 animate-spin text-muted-foreground" />
      </div>
    {:else}
      <!-- Link existing account section -->
      {#if s.availableAccounts.length > 0}
        <div class="space-y-3">
          <Label>{$_('contactSource.linkToExisting', { values: { provider: s.sourceType === 'google' ? 'Google' : 'Microsoft' } })}</Label>
          <div class="border border-border rounded-md divide-y divide-border">
            {#each s.availableAccounts as account (account.accountId)}
              <button
                type="button"
                class="w-full flex items-center gap-3 p-3 text-left hover:bg-muted/50 transition-colors"
                onclick={() => {
                  s.selectedAccountId = account.accountId
                  s.oauthEmail = ''
                  if (!s.name) s.name = $_('contactSource.autoName', { values: { name: account.name || account.email } })
                }}
              >
                <div class="w-4 h-4 border border-border rounded flex items-center justify-center {s.selectedAccountId === account.accountId ? 'bg-primary border-primary' : ''}">
                  {#if s.selectedAccountId === account.accountId}
                    <Icon icon="mdi:check" class="w-3 h-3 text-primary-foreground" />
                  {/if}
                </div>
                <div class="flex-1 min-w-0">
                  <div class="font-medium text-sm truncate">{account.name || account.email}</div>
                  <div class="text-xs text-muted-foreground truncate">{account.email}</div>
                </div>
                {#if !account.hasContactScope}
                  <span class="text-xs text-amber-500 flex items-center gap-1">
                    <Icon icon="mdi:alert" class="w-3 h-3" />
                    {$_('contactSource.reauthNeeded')}
                  </span>
                {/if}
              </button>
            {/each}
          </div>
        </div>

        <div class="relative">
          <div class="absolute inset-0 flex items-center">
            <span class="w-full border-t border-border"></span>
          </div>
          <div class="relative flex justify-center text-xs uppercase">
            <span class="bg-background px-2 text-muted-foreground">{$_('contactSource.or')}</span>
          </div>
        </div>
      {/if}

      <!-- Sign in with OAuth -->
      <div class="space-y-3">
        <Label>
          {s.availableAccounts.length > 0 ? $_('contactSource.signInDifferent') : $_('contactSource.signInProvider', { values: { provider: s.sourceType === 'google' ? 'Google' : 'Microsoft' } })}
        </Label>

        {#if s.oauthInProgress}
          <div class="p-4 border border-border rounded-lg text-center space-y-2">
            <Icon icon="mdi:loading" class="w-8 h-8 animate-spin text-primary mx-auto" />
            <p class="text-sm text-muted-foreground">
              {$_('contactSource.waitingForSignIn')}
            </p>
            {#if s.oauthAuthURL}
              <button
                type="button"
                class="text-xs text-muted-foreground hover:text-foreground inline-flex items-center gap-1.5 transition-colors mx-auto"
                onclick={s.handleCopyOAuthLink}
              >
                {s.oauthLinkCopied ? $_('account.linkCopied') : $_('viewer.copyLink')}
                <Icon icon={s.oauthLinkCopied ? 'mdi:check' : 'mdi:content-copy'} class="w-3.5 h-3.5" />
              </button>
            {/if}
            <Button variant="ghost" size="sm" onclick={() => {
              contactSourcesStore.cancelOAuthFlow()
              s.oauthInProgress = false
              s.oauthAuthURL = null
            }}>
              {$_('common.cancel')}
            </Button>
          </div>
        {:else if s.oauthEmail}
          <div class="p-3 border border-green-500/30 bg-green-500/10 rounded-lg flex items-center gap-3">
            <Icon icon="mdi:check-circle" class="w-5 h-5 text-green-500" />
            <div class="flex-1">
              <div class="text-sm font-medium">{$_('contactSource.signedInAs')}</div>
              <div class="text-sm text-muted-foreground">{s.oauthEmail}</div>
            </div>
            <Button variant="ghost" size="sm" onclick={() => {
              s.oauthEmail = ''
              s.name = ''
            }}>
              {$_('contactSource.change')}
            </Button>
          </div>
        {:else}
          <Button
            variant="outline"
            class="w-full"
            onclick={() => handleStartOAuth(s)}
          >
            <Icon icon={s.sourceType === 'google' ? 'mdi:google' : 'mdi:microsoft'} class="w-4 h-4 mr-2" />
            {$_('contactSource.signInProvider', { values: { provider: s.sourceType === 'google' ? 'Google' : 'Microsoft' } })}
          </Button>
        {/if}
      </div>

      <!-- Name and sync interval (shown when account is selected or OAuth completed) -->
      {#if s.selectedAccountId || s.oauthEmail}
        <div class="space-y-4 pt-2">
          <div class="space-y-2">
            <Label for="oauth-name">{$_('contactSource.name')}</Label>
            <Input
              id="oauth-name"
              bind:value={s.name}
              placeholder={$_('contactSource.sourceName')}
            />
          </div>

          <ContactSourceSyncInterval {s} />
        </div>
      {/if}
    {/if}
  {/if}
