<script lang="ts">
  import { _ } from '$lib/i18n'
  import { Button } from '$lib/components/ui/button'
  import { PreviewMailAPISnapshotRestore, RestoreMailAPISnapshot } from '../../../../../wailsjs/go/app/App'
  import type { snapshotrestore } from '../../../../../wailsjs/go/models'

  let { accountId }: { accountId: string } = $props()
  let open = $state(false)
  let checking = $state(false)
  let working = $state(false)
  let agreed = $state(false)
  let closing = $state(false)
  let error = $state('')
  let rep = $state<snapshotrestore.Report | null>(null)

  const date = $derived(rep ? new Date(rep.createdAt * 1000).toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' }) : '')

  async function start() {
    error = ''; open = true; checking = true; agreed = false
    try { rep = await PreviewMailAPISnapshotRestore(accountId) }
    catch (e) { open = false; error = $_('snapshotRestore.failed', { values: { error: String(e) } }) }
    finally { checking = false }
  }
  async function restore() {
    if (!agreed || working) return
    error = ''; working = true
    try { await RestoreMailAPISnapshot(accountId, true); closing = true }
    catch (e) { error = $_('snapshotRestore.failed', { values: { error: String(e) } }) }
    finally { working = false }
  }
  function cancel() { open = false; rep = null; agreed = false }
</script>

<div class="space-y-2">
  {#if !open}
    <Button size="sm" variant="outline" onclick={start}>{$_('snapshotRestore.open')}</Button>
  {:else if checking}
    <p class="text-sm" aria-live="polite">{$_('snapshotRestore.checking')}</p>
  {:else if rep}
    <div class="space-y-2 rounded border border-destructive/50 p-3 text-sm" role="group" aria-label={$_('snapshotRestore.group')}>
      <p>{$_('snapshotRestore.intro', { values: { date } })}</p>
      <p class="font-medium">{$_('snapshotRestore.accountsHeading')}</p>
      <ul class="list-disc pl-5">
        {#each rep.accounts ?? [] as acc (acc.id)}<li>{$_('snapshotRestore.accountCounts', { values: { email: acc.email, messages: acc.messages, drafts: acc.drafts } })}</li>{/each}
      </ul>
      {#if rep.addedSince?.length}<p>{$_('snapshotRestore.addedSince', { values: { emails: rep.addedSince.join(', ') } })}</p>{/if}
      {#if rep.removedSince?.length}<p>{$_('snapshotRestore.removedSince', { values: { emails: rep.removedSince.join(', ') } })}</p>{/if}
      <p>{$_('snapshotRestore.serverNote')}</p>
      <p>{$_('snapshotRestore.heldNote')}</p>
      <p class="break-all">{$_('snapshotRestore.backup', { values: { dir: rep.backupDir } })}</p>
      <label class="flex items-start gap-2">
        <input type="checkbox" bind:checked={agreed} disabled={working || closing} />
        <span>{$_('snapshotRestore.ack', { values: { date } })}</span>
      </label>
      <div class="flex flex-wrap gap-2">
        <Button size="sm" variant="destructive" onclick={restore} disabled={!agreed || working || closing}>{working ? $_('snapshotRestore.working') : $_('snapshotRestore.action', { values: { date } })}</Button>
        <Button size="sm" variant="outline" onclick={cancel} disabled={working || closing}>{$_('common.cancel')}</Button>
      </div>
      {#if closing}<p role="status">{$_('snapshotRestore.closing')}</p>{/if}
    </div>
  {/if}
  {#if error}<p role="alert" class="text-sm text-destructive">{error}</p>{/if}
</div>
