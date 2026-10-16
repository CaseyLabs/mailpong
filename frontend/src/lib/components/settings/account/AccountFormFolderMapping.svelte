<script lang="ts">
  import Icon from '@iconify/svelte'
  import { Label } from '$lib/components/ui/label'
  import * as Select from '$lib/components/ui/select'
  import { loadFoldersForMapping } from './accountFormLoad'
  import type { AccountFormState } from './accountFormState.svelte'
  import { _ } from '$lib/i18n'

  let { s }: { s: AccountFormState } = $props()
</script>
<!-- Folder Mapping -->
<div class="space-y-2">
  <button
    type="button"
    class="flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
    onclick={() => {
      s.showFolderMapping = !s.showFolderMapping
      if (s.showFolderMapping) loadFoldersForMapping(s)
    }}
    disabled={!s.editAccount}
  >
    <Icon
      icon={s.showFolderMapping ? 'mdi:chevron-down' : 'mdi:chevron-right'}
      class="w-4 h-4"
    />
    {$_('account.folderMapping')}
    {#if !s.editAccount}
      <span class="text-xs text-muted-foreground">{$_('account.saveAccountFirst')}</span>
    {/if}
  </button>

  {#if s.showFolderMapping}
    <div class="space-y-3 pl-6 pt-2 border-l border-border ml-2">
      <p class="text-xs text-muted-foreground">
        {$_('account.folderMappingHelp2')}
      </p>

      {#if s.loadingFolders}
        <div class="flex items-center gap-2 text-sm text-muted-foreground">
          <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
          {$_('account.loadingFolders')}
        </div>
      {:else if s.availableFolders.length === 0}
        <p class="text-sm text-muted-foreground">{$_('account.noFoldersAvailable')}</p>
      {:else}
        <div class="grid gap-3">
          <!-- Sent -->
          <div class="grid grid-cols-[100px_1fr] items-center gap-2">
            <Label class="text-sm">{$_('account.folderSent')}:</Label>
            <Select.Root bind:value={s.sentFolderPath}>
              <Select.Trigger class="h-9">
                <Select.Value placeholder={$_('account.none')}>
                  {s.sentFolderPath || $_('account.none')}
                </Select.Value>
              </Select.Trigger>
              <Select.Content>
                <Select.Item value="" label={$_('account.none')} />
                {#each s.availableFolders as f (f.path)}
                  <Select.Item value={f.path} label={f.path + (s.autoDetectedFolders.sent === f.path ? ' ' + $_('account.detected') : '')} />
                {/each}
              </Select.Content>
            </Select.Root>
          </div>

          <!-- Drafts -->
          <div class="grid grid-cols-[100px_1fr] items-center gap-2">
            <Label class="text-sm">{$_('account.folderDrafts')}:</Label>
            <Select.Root bind:value={s.draftsFolderPath}>
              <Select.Trigger class="h-9">
                <Select.Value placeholder={$_('account.none')}>
                  {s.draftsFolderPath || $_('account.none')}
                </Select.Value>
              </Select.Trigger>
              <Select.Content>
                <Select.Item value="" label={$_('account.none')} />
                {#each s.availableFolders as f (f.path)}
                  <Select.Item value={f.path} label={f.path + (s.autoDetectedFolders.drafts === f.path ? ' ' + $_('account.detected') : '')} />
                {/each}
              </Select.Content>
            </Select.Root>
          </div>

          <!-- Trash -->
          <div class="grid grid-cols-[100px_1fr] items-center gap-2">
            <Label class="text-sm">{$_('account.folderTrash')}:</Label>
            <Select.Root bind:value={s.trashFolderPath}>
              <Select.Trigger class="h-9">
                <Select.Value placeholder={$_('account.none')}>
                  {s.trashFolderPath || $_('account.none')}
                </Select.Value>
              </Select.Trigger>
              <Select.Content>
                <Select.Item value="" label={$_('account.none')} />
                {#each s.availableFolders as f (f.path)}
                  <Select.Item value={f.path} label={f.path + (s.autoDetectedFolders.trash === f.path ? ' ' + $_('account.detected') : '')} />
                {/each}
              </Select.Content>
            </Select.Root>
          </div>

          <!-- Spam/Junk -->
          <div class="grid grid-cols-[100px_1fr] items-center gap-2">
            <Label class="text-sm">{$_('account.folderSpam')}:</Label>
            <Select.Root bind:value={s.spamFolderPath}>
              <Select.Trigger class="h-9">
                <Select.Value placeholder={$_('account.none')}>
                  {s.spamFolderPath || $_('account.none')}
                </Select.Value>
              </Select.Trigger>
              <Select.Content>
                <Select.Item value="" label={$_('account.none')} />
                {#each s.availableFolders as f (f.path)}
                  <Select.Item value={f.path} label={f.path + (s.autoDetectedFolders.spam === f.path ? ' ' + $_('account.detected') : '')} />
                {/each}
              </Select.Content>
            </Select.Root>
          </div>

          <!-- Archive -->
          <div class="grid grid-cols-[100px_1fr] items-center gap-2">
            <Label class="text-sm">{$_('account.folderArchive')}:</Label>
            <Select.Root bind:value={s.archiveFolderPath}>
              <Select.Trigger class="h-9">
                <Select.Value placeholder={$_('account.none')}>
                  {s.archiveFolderPath || $_('account.none')}
                </Select.Value>
              </Select.Trigger>
              <Select.Content>
                <Select.Item value="" label={$_('account.none')} />
                {#each s.availableFolders as f (f.path)}
                  <Select.Item value={f.path} label={f.path + (s.autoDetectedFolders.archive === f.path ? ' ' + $_('account.detected') : '')} />
                {/each}
              </Select.Content>
            </Select.Root>
          </div>

          <!-- All Mail -->
          <div class="grid grid-cols-[100px_1fr] items-center gap-2">
            <Label class="text-sm">{$_('account.folderAllMail')}:</Label>
            <Select.Root bind:value={s.allMailFolderPath}>
              <Select.Trigger class="h-9">
                <Select.Value placeholder={$_('account.none')}>
                  {s.allMailFolderPath || $_('account.none')}
                </Select.Value>
              </Select.Trigger>
              <Select.Content>
                <Select.Item value="" label={$_('account.none')} />
                {#each s.availableFolders as f (f.path)}
                  <Select.Item value={f.path} label={f.path + (s.autoDetectedFolders.all === f.path ? ' ' + $_('account.detected') : '')} />
                {/each}
              </Select.Content>
            </Select.Root>
          </div>

          <!-- Starred -->
          <div class="grid grid-cols-[100px_1fr] items-center gap-2">
            <Label class="text-sm">{$_('account.folderStarred')}:</Label>
            <Select.Root bind:value={s.starredFolderPath}>
              <Select.Trigger class="h-9">
                <Select.Value placeholder={$_('account.none')}>
                  {s.starredFolderPath || $_('account.none')}
                </Select.Value>
              </Select.Trigger>
              <Select.Content>
                <Select.Item value="" label={$_('account.none')} />
                {#each s.availableFolders as f (f.path)}
                  <Select.Item value={f.path} label={f.path + (s.autoDetectedFolders.starred === f.path ? ' ' + $_('account.detected') : '')} />
                {/each}
              </Select.Content>
            </Select.Root>
          </div>
        </div>
      {/if}
    </div>
  {/if}
</div>
