<script lang="ts">
  import Icon from '@iconify/svelte'
  import { Button } from '$lib/components/ui/button'
  import { addToast } from '$lib/stores/toast'
  import { _ } from '$lib/i18n'
  // @ts-ignore - wailsjs path
  import { app } from '../../../../../wailsjs/go/models'
  // @ts-ignore - wailsjs path
  import { RefreshSendAs, ImportSendAs } from '../../../../../wailsjs/go/app/App'

  interface Props {
    accountId: string
    status: app.SendAsStatus
    /** Called with the fresh status after a refresh */
    onStatus: (status: app.SendAsStatus) => void
    /** Called after aliases were added as identities */
    onImported: () => void | Promise<void>
  }

  let { accountId, status, onStatus, onImported }: Props = $props()

  let refreshing = $state(false)
  let addingEmail = $state<string | null>(null)
  let withSignature = $state(false)

  async function refresh() {
    refreshing = true
    try {
      onStatus(await RefreshSendAs(accountId))
      addToast({ type: 'success', message: $_('identity.sendAsRefreshed') })
    } catch (err) {
      addToast({ type: 'error', message: $_('identity.sendAsRefreshFailed') + ': ' + (err as Error).message })
    } finally {
      refreshing = false
    }
  }

  async function add(alias: app.SendAsAlias) {
    addingEmail = alias.email
    try {
      const created = await ImportSendAs(accountId, [alias.email], withSignature)
      if (!created?.length) throw new Error($_('identity.sendAsNotAdded'))
      addToast({ type: 'success', message: $_('identity.sendAsAdded', { values: { email: alias.email } }) })
      await onImported()
    } catch (err) {
      addToast({ type: 'error', message: $_('identity.sendAsAddFailed') + ': ' + (err as Error).message })
    } finally {
      addingEmail = null
    }
  }
</script>

<div class="space-y-3 mt-6 pt-6 border-t border-border">
  <div class="flex items-center justify-between">
    <div>
      <h3 class="text-sm font-medium flex items-center gap-2">
        <Icon icon="mdi:google" class="w-4 h-4" />
        {$_('identity.sendAsTitle')}
      </h3>
      <p class="text-xs text-muted-foreground mt-1">{$_('identity.sendAsHelp')}</p>
    </div>
    <Button size="sm" variant="outline" onclick={refresh} disabled={refreshing}>
      <Icon icon={refreshing ? 'mdi:loading' : 'mdi:refresh'} class="w-4 h-4 mr-1 {refreshing ? 'animate-spin' : ''}" />
      {$_('identity.sendAsRefresh')}
    </Button>
  </div>

  {#if !status.fetched}
    <p class="text-xs text-muted-foreground">{$_('identity.sendAsNever')}</p>
  {:else if status.available.length === 0}
    <p class="text-xs text-muted-foreground">{$_('identity.sendAsNone')}</p>
  {:else}
    <div class="space-y-2">
      {#each status.available as alias (alias.email)}
        <div class="flex items-center gap-3 p-3 rounded-lg border border-border bg-card">
          <div class="flex-1 min-w-0">
            <div class="font-medium text-sm truncate">{alias.email}</div>
            {#if alias.displayName}
              <div class="text-xs text-muted-foreground truncate">{alias.displayName}</div>
            {/if}
          </div>
          <Button size="sm" onclick={() => add(alias)} disabled={addingEmail !== null}>
            <Icon icon={addingEmail === alias.email ? 'mdi:loading' : 'mdi:plus'} class="w-4 h-4 mr-1 {addingEmail === alias.email ? 'animate-spin' : ''}" />
            {$_('identity.sendAsAdd')}
          </Button>
        </div>
      {/each}
    </div>
    <label class="flex items-center gap-2 text-xs text-muted-foreground">
      <input type="checkbox" bind:checked={withSignature} />
      {$_('identity.sendAsImportSignature')}
    </label>
  {/if}
</div>
