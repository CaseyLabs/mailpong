<script lang="ts">
  import Icon from '@iconify/svelte'
  import { Label } from '$lib/components/ui/label'
  import * as Select from '$lib/components/ui/select'
  // @ts-ignore - wailsjs path
  import { account } from '../../../../../wailsjs/go/models'
  // @ts-ignore - wailsjs path
  import { GetAccountFoldersForMapping, GetAutoDetectedFolders } from '../../../../../wailsjs/go/app/App'
  import { _ } from '$lib/i18n'

  interface Props {
    editAccount: account.Account
    sentFolderPath: string
    draftsFolderPath: string
    trashFolderPath: string
    spamFolderPath: string
    archiveFolderPath: string
    allMailFolderPath: string
    starredFolderPath: string
    onFolderMappingChange: (type: string, value: string) => void
  }

  let {
    editAccount,
    sentFolderPath = $bindable(),
    draftsFolderPath = $bindable(),
    trashFolderPath = $bindable(),
    spamFolderPath = $bindable(),
    archiveFolderPath = $bindable(),
    allMailFolderPath = $bindable(),
    starredFolderPath = $bindable(),
    onFolderMappingChange,
  }: Props = $props()

  // Folder mapping state
  let showFolderMapping = $state(false)
  let loadingFolders = $state(false)
  let availableFolders = $state<any[]>([])
  let autoDetectedFolders = $state<Record<string, string>>({})

  // Load folders for mapping UI
  async function loadFoldersForMapping() {
    if (availableFolders.length > 0) return

    loadingFolders = true
    try {
      availableFolders = await GetAccountFoldersForMapping(editAccount.id)
      autoDetectedFolders = await GetAutoDetectedFolders(editAccount.id)
    } catch (err) {
      console.error('Failed to load folders for mapping:', err)
    } finally {
      loadingFolders = false
    }
  }

  function handleFolderMappingToggle() {
    showFolderMapping = !showFolderMapping
    if (showFolderMapping) {
      loadFoldersForMapping()
    }
  }

  // Folder mapping types configuration
  // get() returns saved mapping or falls back to auto-detected folder
  const folderMappingTypes = [
    { key: 'sent', labelKey: 'account.folderSent', get: () => sentFolderPath || autoDetectedFolders['sent'] || '', set: (v: string) => { sentFolderPath = v; onFolderMappingChange('sent', v) }},
    { key: 'drafts', labelKey: 'account.folderDrafts', get: () => draftsFolderPath || autoDetectedFolders['drafts'] || '', set: (v: string) => { draftsFolderPath = v; onFolderMappingChange('drafts', v) }},
    { key: 'trash', labelKey: 'account.folderTrash', get: () => trashFolderPath || autoDetectedFolders['trash'] || '', set: (v: string) => { trashFolderPath = v; onFolderMappingChange('trash', v) }},
    { key: 'spam', labelKey: 'account.folderSpam', get: () => spamFolderPath || autoDetectedFolders['spam'] || '', set: (v: string) => { spamFolderPath = v; onFolderMappingChange('spam', v) }},
    { key: 'archive', labelKey: 'account.folderArchive', get: () => archiveFolderPath || autoDetectedFolders['archive'] || '', set: (v: string) => { archiveFolderPath = v; onFolderMappingChange('archive', v) }},
    { key: 'all', labelKey: 'account.folderAllMail', get: () => allMailFolderPath || autoDetectedFolders['all'] || '', set: (v: string) => { allMailFolderPath = v; onFolderMappingChange('all', v) }},
    { key: 'starred', labelKey: 'account.folderStarred', get: () => starredFolderPath || autoDetectedFolders['starred'] || '', set: (v: string) => { starredFolderPath = v; onFolderMappingChange('starred', v) }},
  ]
</script>

  <!-- Folder Mapping -->
  <div class="space-y-2">
    <button
      type="button"
      class="flex items-center gap-2 text-sm font-medium hover:text-primary transition-colors"
      onclick={handleFolderMappingToggle}
    >
      <Icon
        icon={showFolderMapping ? 'mdi:chevron-down' : 'mdi:chevron-right'}
        class="w-4 h-4"
      />
      <Icon icon="mdi:folder-cog-outline" class="w-4 h-4" />
      {$_('account.folderMapping')}
    </button>

    {#if showFolderMapping}
      <div class="space-y-3 pl-6 pt-2 border-l border-border ml-2">
        <p class="text-xs text-muted-foreground">
          {$_('account.folderMappingHelp2')}
        </p>

        {#if loadingFolders}
          <div class="flex items-center gap-2 text-sm text-muted-foreground">
            <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
            {$_('account.loadingFolders')}
          </div>
        {:else if availableFolders.length === 0}
          <p class="text-sm text-muted-foreground">{$_('account.noFoldersAvailable')}</p>
        {:else}
          <div class="grid gap-3">
            {#each folderMappingTypes as mapping (mapping.key)}
              <div class="grid grid-cols-[100px_1fr] items-center gap-2">
                <Label class="text-sm">{$_(mapping.labelKey)}:</Label>
                <Select.Root value={mapping.get()} onValueChange={mapping.set}>
                  <Select.Trigger class="h-9">
                    <Select.Value placeholder={$_('account.none')}>
                      {(mapping.get() || $_('account.none')) + (autoDetectedFolders[mapping.key] === mapping.get() ? ' ' + $_('account.detected') : '')}
                    </Select.Value>
                  </Select.Trigger>
                  <Select.Content>
                    <Select.Item value="" label={$_('account.none')} />
                    {#each availableFolders as f (f.path)}
                      <Select.Item
                        value={f.path}
                        label={f.path + (autoDetectedFolders[mapping.key] === f.path ? ' ' + $_('account.detected') : '')}
                      />
                    {/each}
                  </Select.Content>
                </Select.Root>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {/if}
  </div>
