<script lang="ts">
  // SyncStatusDialog — opened by the chat list and sidebar Sync buttons while a sync
  // is running: per-account progress, any sync errors, and a cancel action.
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import * as Dialog from '$lib/components/ui/dialog'
  import { Button } from '$lib/components/ui/button'
  import { dialogGuardOpen, dialogGuardClose } from '$lib/stores/dialogGuard'
  import { accountStore, type SyncProgress } from '$lib/stores/accounts.svelte'

  interface Props {
    open: boolean
    /** True while any sync (account-wide or the current folder) is running. */
    busy: boolean
    onCancelSync: () => void | Promise<void>
    /** Opens the account editor (where it can be re-authorized or given its own OAuth client). */
    onFixAccount?: (accountId: string) => void
  }

  let { open = $bindable(false), busy, onCancelSync, onFixAccount }: Props = $props()

  // Hold the dialog guard while open so App's shortcuts leave the buttons alone.
  $effect(() => {
    if (!open) return
    dialogGuardOpen()
    return () => dialogGuardClose()
  })

  const rows = $derived(
    accountStore.accounts
      .map((a) => {
        const id = a.account.id
        const progress = accountStore.getSyncProgress(id)
        const error = accountStore.getSyncError(id)
        return {
          id,
          name: a.account.name,
          syncing: a.syncing || !!progress,
          progress,
          progressFolder: folderName(id, progress?.folderId ?? ''),
          error,
          errorFolder: error ? folderName(id, error.folderId) : '',
        }
      })
      .filter((r) => r.syncing || r.error)
  )

  // OAuth failures get a plain-language hint above the raw error.
  function errorHint(message: string): string {
    const missing = /not configured \(missing client ID\)(?:: (\S+))?/.exec(message)
    if (missing) return $_('sidebar.syncErrorOAuthNotConfigured', { values: { provider: missing[1] ?? 'this provider' } })
    if (/unauthorized_client|invalid_grant|token refresh failed/i.test(message)) return $_('sidebar.syncErrorReauthorize')
    return ''
  }

  function phaseLabel(progress: SyncProgress | null): string {
    if (!progress) return $_('sidebar.syncing')
    const values = { percentage: progress.percentage }
    switch (progress.phase) {
      case 'folders': return $_('sidebar.syncingFolders')
      case 'messages': return $_('sidebar.fetchingMessageList')
      case 'headers': return $_('sidebar.fetchingHeaders', { values })
      default: return $_('sidebar.syncingContent', { values })
    }
  }

  function folderName(accountId: string, folderId: string): string {
    return folderId ? accountStore.getFolder(accountId, folderId)?.name ?? '' : ''
  }

  async function cancel() {
    await onCancelSync()
    open = false
  }
</script>

<Dialog.Root bind:open>
  <Dialog.Content class="max-w-md">
    <Dialog.Header>
      <Dialog.Title>{$_('sidebar.syncStatusTitle')}</Dialog.Title>
      <Dialog.Description>
        {busy ? $_('sidebar.syncing') : $_('sidebar.syncFinished')}
      </Dialog.Description>
    </Dialog.Header>

    <ul class="space-y-3 max-h-80 overflow-y-auto" aria-live="polite">
      {#each rows as row (row.id)}
        <li class="space-y-1">
          <div class="flex items-center gap-2 text-sm font-medium">
            {#if row.syncing}
              <Icon icon="mdi:refresh" class="w-4 h-4 shrink-0 animate-spin text-muted-foreground" />
            {:else}
              <Icon icon="mdi:alert-circle" class="w-4 h-4 shrink-0 text-destructive" />
            {/if}
            <span class="truncate">{row.name}</span>
          </div>
          {#if row.syncing}
            <p class="text-xs text-muted-foreground pl-6">
              {phaseLabel(row.progress)}
              {#if row.progressFolder}· {row.progressFolder}{/if}
            </p>
            {#if row.progress && row.progress.phase !== 'folders' && row.progress.phase !== 'messages'}
              <div class="ml-6 h-1.5 rounded bg-muted overflow-hidden" role="progressbar"
                   aria-valuemin={0} aria-valuemax={100} aria-valuenow={row.progress.percentage}>
                <div class="h-full bg-primary transition-all" style="width: {row.progress.percentage}%"></div>
              </div>
            {/if}
          {/if}
          {#if row.error}
            {@const hint = errorHint(row.error.error)}
            {#if hint}
              <p class="text-xs text-destructive pl-6 break-words">{hint}</p>
            {/if}
            <p class="text-xs text-destructive pl-6 break-words select-text">
              {#if row.errorFolder}{row.errorFolder}: {/if}{row.error.error}
            </p>
            {#if onFixAccount}
              <div class="pl-6">
                <Button type="button" size="sm" variant="outline" onclick={() => onFixAccount(row.id)}>{$_('sidebar.syncErrorFixAccount')}</Button>
              </div>
            {/if}
          {/if}
        </li>
      {:else}
        <li class="text-sm text-muted-foreground">{$_('sidebar.syncNoDetails')}</li>
      {/each}
    </ul>

    <Dialog.Footer>
      <Button type="button" variant="outline" onclick={() => (open = false)}>{$_('common.close')}</Button>
      {#if busy}
        <Button type="button" variant="destructive" onclick={cancel}>{$_('sidebar.cancelSync')}</Button>
      {/if}
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>
