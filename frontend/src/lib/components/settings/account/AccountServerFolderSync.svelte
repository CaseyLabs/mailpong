<script lang="ts">
  import Icon from '@iconify/svelte'
  import { Label } from '$lib/components/ui/label'
  import Switch from '$lib/components/ui/switch/Switch.svelte'
  // @ts-ignore - wailsjs path
  import { account } from '../../../../../wailsjs/go/models'
  // @ts-ignore - wailsjs path
  import { GetFolders, SubscribeFolder, UnsubscribeFolder, SubscribeAllFolders } from '../../../../../wailsjs/go/app/App'
  // @ts-ignore - wailsjs path
  import { folder } from '../../../../../wailsjs/go/models'
  import { _ } from '$lib/i18n'

  interface Props {
    editAccount: account.Account
    syncAllFolders: boolean
    onSyncAllFoldersChange: (value: boolean) => void
    syncFoldersEnabled: boolean
    onSyncFoldersEnabledChange: (value: boolean) => void
  }

  let {
    editAccount,
    syncAllFolders = $bindable(),
    onSyncAllFoldersChange,
    syncFoldersEnabled = $bindable(),
    onSyncFoldersEnabledChange,
  }: Props = $props()

  // Folder sync subscription state
  let showFolderSync = $state(false)
  let loadingSyncFolders = $state(false)
  let syncFolders = $state<folder.Folder[]>([])

  const coreFolderTypes = ['inbox', 'drafts', 'sent']

  async function loadSyncFolders() {
    if (syncFolders.length > 0) return
    loadingSyncFolders = true
    try {
      syncFolders = await GetFolders(editAccount.id)
    } catch (err) {
      console.error('Failed to load folders for sync:', err)
    } finally {
      loadingSyncFolders = false
    }
  }

  function handleFolderSyncToggle() {
    showFolderSync = !showFolderSync
    if (showFolderSync) {
      loadSyncFolders()
    }
  }

  async function handleSyncAllToggle(checked: boolean) {
    syncAllFolders = checked
    onSyncAllFoldersChange(checked)
    if (!checked) return
    try {
      await SubscribeAllFolders(editAccount.id)
      // Refresh folder list to show updated subscription state
      syncFolders = await GetFolders(editAccount.id)
    } catch (err) {
      console.error('Failed to subscribe to all folders:', err)
    }
  }

  async function handleFolderSubscriptionToggle(f: folder.Folder, subscribed: boolean) {
    const action = subscribed ? SubscribeFolder : UnsubscribeFolder
    try {
      await action(editAccount.id, f.id)
      // Update local state
      syncFolders = syncFolders.map(sf =>
        sf.id === f.id ? { ...sf, subscribed } as folder.Folder : sf
      )
    } catch (err) {
      console.error('Failed to update folder subscription:', err)
    }
  }
</script>

  <!-- Folder Sync Subscriptions -->
  <div class="space-y-2">
    <button
      type="button"
      class="flex items-center gap-2 text-sm font-medium hover:text-primary transition-colors"
      onclick={handleFolderSyncToggle}
    >
      <Icon
        icon={showFolderSync ? 'mdi:chevron-down' : 'mdi:chevron-right'}
        class="w-4 h-4"
      />
      <Icon icon="mdi:folder-sync-outline" class="w-4 h-4" />
      {$_('account.folderSync')}
    </button>

    {#if showFolderSync}
      <div class="space-y-4 pl-6 pt-2 border-l border-border ml-2">
        <!-- Manage folder sync toggle (opt-in) -->
        <div class="space-y-1">
          <div class="flex items-center gap-3">
            <Label>{$_('account.manageFolderSync')}</Label>
            <Switch
              bind:checked={syncFoldersEnabled}
              onCheckedChange={(v) => { syncFoldersEnabled = v; onSyncFoldersEnabledChange(v); if (v) loadSyncFolders() }}
            />
          </div>
          <p class="text-xs text-muted-foreground">
            {$_('account.manageFolderSyncHelp')}
          </p>
        </div>

        {#if syncFoldersEnabled}
          <!-- Sync All Folders toggle -->
          <div class="flex items-center gap-3">
            <Label>{$_('account.syncAllFolders')}</Label>
            <Switch
              bind:checked={syncAllFolders}
              onCheckedChange={handleSyncAllToggle}
            />
          </div>

          <!-- Folder checklist (when sync all is off) -->
          {#if !syncAllFolders}
            {#if loadingSyncFolders}
              <div class="flex items-center gap-2 text-sm text-muted-foreground">
                <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
                {$_('account.loadingFolders')}
              </div>
            {:else}
              <div class="space-y-1 max-h-48 overflow-y-auto">
                {#each syncFolders as f (f.id)}
                  {@const isCore = coreFolderTypes.includes(f.type)}
                  <label class="flex items-center gap-2 cursor-pointer py-0.5 {isCore ? 'opacity-60' : ''}">
                    <input
                      type="checkbox"
                      checked={isCore || f.subscribed}
                      disabled={isCore}
                      onchange={(e) => handleFolderSubscriptionToggle(f, (e.target as HTMLInputElement).checked)}
                      class="rounded border-border"
                    />
                    <span class="text-sm truncate">{f.path}</span>
                    {#if isCore}
                      <span class="text-xs text-muted-foreground">({$_('account.alwaysSynced')})</span>
                    {/if}
                  </label>
                {/each}
              </div>
            {/if}
          {/if}
        {/if}
      </div>
    {/if}
  </div>
