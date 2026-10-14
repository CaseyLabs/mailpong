<script lang="ts">
  import { onMount } from 'svelte'
  import { _ } from '$lib/i18n'
  import { Button } from '$lib/components/ui/button'
  import {
    GetSnapshotRestoreResult, DismissSnapshotRestoreResult, CleanRestoreOrphanCredentials,
    PreviewUndoSnapshotRestore, UndoSnapshotRestore,
  } from '../../../../wailsjs/go/app/App'
  import type { snapshotrestore } from '../../../../wailsjs/go/models'

  let res = $state<snapshotrestore.Result | null>(null)
  let undoing = $state(false)
  let agreed = $state(false)
  let closing = $state(false)
  let error = $state('')

  const date = $derived(res ? new Date(res.snapshotCreatedAt * 1000).toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' }) : '')
  const ok = $derived(res?.status === 'restored')
  const orphans = $derived((res?.orphanedAccounts ?? []).map(o => o.email).join(', '))

  onMount(async () => {
    try { const r = await GetSnapshotRestoreResult(); res = r?.status ? r : null } catch { res = null }
  })
  async function run(fn: () => Promise<unknown>, after?: () => void) {
    error = ''
    try { await fn(); after?.() } catch (e) { error = String(e) }
  }
  const dismiss = () => run(DismissSnapshotRestoreResult, () => { res = null })
  const cleanup = () => run(async () => { await CleanRestoreOrphanCredentials(); if (res) res.orphanedAccounts = [] })
  const beginUndo = () => run(PreviewUndoSnapshotRestore, () => { undoing = true; agreed = false })
  const undo = () => run(() => UndoSnapshotRestore(true), () => { closing = true })
</script>

{#if res}
  <section class="space-y-1 border-b border-border bg-muted px-4 py-2 text-sm" aria-label={$_('snapshotRestore.group')}>
    {#if ok}
      <p role="status">{res.undo ? $_('snapshotRestore.bannerUndone') : $_('snapshotRestore.bannerRestored', { values: { date } })}</p>
      {#if res.heldActions}<p>{$_('snapshotRestore.held', { values: { count: res.heldActions } })}</p>{/if}
      {#if res.queuedSubmissions}<p>{$_('snapshotRestore.submissions', { values: { count: res.queuedSubmissions } })}</p>{/if}
      {#if res.needsReauth?.length}<p role="alert">{$_('snapshotRestore.reauth', { values: { emails: res.needsReauth.join(', ') } })}</p>{/if}
      <p>{$_('snapshotRestore.routeNote')}</p>
      {#if res.orphanedAccounts?.length}<p>{$_('snapshotRestore.orphans', { values: { emails: orphans } })}</p>{/if}
    {:else if res.status === 'intent-discarded'}
      <p role="alert">{$_('snapshotRestore.bannerDiscarded')}</p>
    {:else}
      <p role="alert">{$_('snapshotRestore.bannerFailed', { values: { error: res.error } })}</p>
    {/if}
    {#if undoing}
      <div class="space-y-1" role="group" aria-label={$_('snapshotRestore.undo')}>
        <p>{$_('snapshotRestore.undoIntro')}</p>
        <label class="flex items-start gap-2"><input type="checkbox" bind:checked={agreed} disabled={closing} /><span>{$_('snapshotRestore.undoAck')}</span></label>
        <Button size="sm" variant="destructive" onclick={undo} disabled={!agreed || closing}>{$_('snapshotRestore.undoAction')}</Button>
        {#if closing}<p role="status">{$_('snapshotRestore.closing')}</p>{/if}
      </div>
    {/if}
    {#if error}<p role="alert" class="text-destructive">{error}</p>{/if}
    <div class="flex flex-wrap gap-2">
      {#if ok && !res.undo && res.backupPath && !undoing}<Button size="sm" variant="outline" onclick={beginUndo}>{$_('snapshotRestore.undo')}</Button>{/if}
      {#if ok && res.orphanedAccounts?.length}<Button size="sm" variant="outline" onclick={cleanup}>{$_('snapshotRestore.cleanup')}</Button>{/if}
      <Button size="sm" variant="outline" onclick={dismiss} disabled={closing}>{$_('snapshotRestore.dismiss')}</Button>
    </div>
  </section>
{/if}
