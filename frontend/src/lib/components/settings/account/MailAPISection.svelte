<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import { _ } from '$lib/i18n'
  import { Button } from '$lib/components/ui/button'
  import { mailApiRoutes, mailApiRouteName } from '$lib/stores/mailApiRoutes.svelte'
  import {
    GetMailAPIRouteStatus, PlanMailAPIMigration, StartMailAPIConsent, CancelMailAPIConsent,
    ClearMailAPIRoute, MigrateMailAPIAccount, RollbackMailAPIMigration,
  } from '../../../../../wailsjs/go/app/App'
  import { EventsOn, EventsOff } from '../../../../../wailsjs/runtime/runtime'
  import { app } from '../../../../../wailsjs/go/models'

  let { accountId }: { accountId: string } = $props()
  let status = $state<app.MailAPIRouteStatus | null>(null)
  let plan = $state<app.MailAPIMigrationPlan | null>(null)
  let waiting = $state(false)
  let reviewing = $state(false)
  let agreed = $state(false)
  let migrating = $state(false)
  let confirmRollback = $state(false)
  let rolling = $state(false)
  let message = $state('')
  let error = $state('')
  let report = $state<app.MailAPIRollbackReport | null>(null)
  let reauthRequired = $state(false)

  const staged = $derived(status?.state === 'staged')
  const active = $derived(status?.state === 'active')
  const busy = $derived(waiting || migrating || rolling)
  const routeName = $derived($_(`mailApi.${mailApiRouteName(active ? status?.route : undefined)}`))
  const resuming = $derived(plan?.migration?.state === 'snapshotted' || plan?.migration?.state === 'activated')
  const blockers = $derived(plan?.blockers ?? [])

  async function load() {
    try {
      status = await GetMailAPIRouteStatus(accountId)
      plan = status.route !== 'protocol' ? await PlanMailAPIMigration(accountId) : null
      void mailApiRoutes.refresh(accountId)
    } catch (e) { error = $_('mailApi.failed', { values: { error: String(e) } }) }
  }
  function failureText(failure: string) {
    if (failure === 'reauthorize') return $_('mailApi.reauthorize')
    if (failure === 'partial-consent') return $_('mailApi.partialConsent')
    return ''
  }
  async function authorize() {
    error = ''; message = ''; reauthRequired = false; waiting = true
    try { await StartMailAPIConsent(accountId, false) }
    catch (e) { waiting = false; error = $_('mailApi.failed', { values: { error: String(e) } }) }
  }
  async function cancelConsent() { await CancelMailAPIConsent() }
  async function removeAuth() {
    error = ''; message = ''; reviewing = false; agreed = false
    try { await ClearMailAPIRoute(accountId) } catch (e) { error = $_('mailApi.failed', { values: { error: String(e) } }) }
    await load()
  }
  async function migrate() {
    if (!agreed || busy || blockers.length) return
    error = ''; message = ''; migrating = true
    try {
      const rec = await MigrateMailAPIAccount(accountId)
      reviewing = false; agreed = false
      message = $_('mailApi.migrated', { values: { correlated: rec.correlated, uncorrelated: rec.uncorrelated } })
    } catch (e) { error = $_('mailApi.migrateFailed', { values: { error: String(e) } }) }
    finally { migrating = false; await load() }
  }
  async function rollback() {
    if (busy) return
    error = ''; message = ''; rolling = true
    try {
      report = await RollbackMailAPIMigration(accountId)
      confirmRollback = false
      message = $_('mailApi.rolledBack', { values: { applied: report.appliedSince } })
    } catch (e) { error = $_('mailApi.failed', { values: { error: String(e) } }) }
    finally { rolling = false; await load() }
  }
  onMount(() => {
    void load()
    EventsOn('mail-api:success', (d: { accountId: string }) => { if (d.accountId === accountId) { waiting = false; reauthRequired = false; void load() } })
    EventsOn('mail-api:cancelled', () => { waiting = false })
    EventsOn('mail-api:error', (d: { accountId: string; failure: string; error: string }) => {
      if (d.accountId !== accountId) return
      waiting = false
      error = $_('mailApi.authFailed', { values: { failure: d.failure, error: d.error } })
    })
    EventsOn('mail-api:reauth-required', (d: { accountId: string; failure: string; error: string }) => {
      if (d.accountId !== accountId) return
      reauthRequired = true
      error = $_('mailApi.authFailed', { values: { failure: d.failure, error: d.error } })
      void load()
    })
  })
  onDestroy(() => {
    EventsOff('mail-api:success'); EventsOff('mail-api:cancelled'); EventsOff('mail-api:error'); EventsOff('mail-api:reauth-required')
    if (waiting) void CancelMailAPIConsent().catch(() => {})
  })
</script>
<section class="space-y-3 border-t border-border mt-6 pt-4" aria-label={$_('mailApi.title')}>
  <h3 class="font-medium">{$_('mailApi.title')}</h3>
  <p class="text-sm">{$_('mailApi.routeLabel')}: <strong>{routeName}</strong></p>
  {#if !active}
    <p class="text-sm text-muted-foreground">{$_('mailApi.intro')}</p>
    <p class="text-sm text-muted-foreground">{$_('mailApi.scopes')}</p>
    <p class="text-sm text-muted-foreground">{$_('mailApi.limits')}</p>
  {/if}

  {#if status && !staged && !active}
    <div class="flex flex-wrap gap-2">
      <Button size="sm" onclick={authorize} disabled={busy}>{$_('mailApi.authorize')}</Button>
      {#if waiting}<Button size="sm" variant="outline" onclick={cancelConsent}>{$_('common.cancel')}</Button>{/if}
    </div>
  {/if}

  {#if staged && status}
    {#if status.failure}<p role="alert" class="text-sm text-destructive">{failureText(status.failure)}</p>{/if}
    {#if plan}
      <div class="space-y-2 text-sm" aria-label={$_('mailApi.planTitle')} role="group">
        <p class="font-medium">{$_('mailApi.planTitle')}</p>
        <p>{$_('mailApi.counts', { values: { folders: plan.counts.folders, imap: plan.counts.imapMessages, api: plan.counts.apiMessages, drafts: plan.counts.drafts, queued: plan.counts.queuedSubmissions, pending: plan.pendingActions } })}</p>
        {#if resuming}<p>{$_('mailApi.resumeNote')}</p>{/if}
        {#if blockers.length}
          <p class="font-medium">{$_('mailApi.blockers')}</p>
          <ul class="list-disc pl-5">{#each blockers as b (b)}<li>{b}</li>{/each}</ul>
        {/if}
      </div>
    {/if}
    {#if !reviewing}
      <div class="flex flex-wrap gap-2">
        <Button size="sm" onclick={() => { reviewing = true; agreed = false }} disabled={busy || blockers.length > 0}>{$_('mailApi.review')}</Button>
        <Button size="sm" variant="outline" onclick={authorize} disabled={busy}>{$_('mailApi.authorize')}</Button>
        {#if waiting}<Button size="sm" variant="outline" onclick={cancelConsent}>{$_('common.cancel')}</Button>{/if}
        <Button size="sm" variant="outline" onclick={removeAuth} disabled={busy}>{$_('mailApi.removeAuth')}</Button>
      </div>
    {:else}
      <div class="space-y-2 rounded border border-border p-3 text-sm" role="group" aria-label={$_('mailApi.review')}>
        <p>{$_('mailApi.snapshotNote')}</p>
        <label class="flex items-start gap-2"><input type="checkbox" class="mt-1" bind:checked={agreed} disabled={migrating} /><span>{$_('mailApi.consent')}</span></label>
        <div class="flex flex-wrap gap-2">
          <Button size="sm" onclick={migrate} disabled={!agreed || busy || blockers.length > 0}>{migrating ? $_('mailApi.migrating') : $_('mailApi.migrate')}</Button>
          <Button size="sm" variant="outline" onclick={() => { reviewing = false; agreed = false }} disabled={migrating}>{$_('common.cancel')}</Button>
        </div>
      </div>
    {/if}
  {/if}

  {#if active}
    <p class="text-sm">{$_('mailApi.active')}</p>
    {#if reauthRequired}<p role="alert" class="text-sm text-destructive">{$_('mailApi.reauthorize')}</p>{/if}
    {#if status?.failure}<p role="alert" class="text-sm text-destructive">{failureText(status.failure)}</p>{/if}
    {#if plan?.migration?.snapshotPath}<p class="break-all text-xs text-muted-foreground">{$_('mailApi.snapshotAt', { values: { path: plan.migration.snapshotPath } })}</p>{/if}
    <div class="flex flex-wrap gap-2">
      <Button size="sm" variant="outline" onclick={authorize} disabled={busy}>{$_('mailApi.reauthorizeAction')}</Button>
      {#if waiting}<Button size="sm" variant="outline" onclick={cancelConsent}>{$_('common.cancel')}</Button>{/if}
    </div>
    {#if !confirmRollback}
      <Button size="sm" variant="outline" onclick={() => confirmRollback = true} disabled={busy}>{$_('mailApi.rollback')}</Button>
    {:else}
      <div class="space-y-2 rounded border border-border p-3 text-sm" role="group" aria-label={$_('mailApi.rollback')}>
        <p>{$_('mailApi.rollbackConfirm')}</p>
        <div class="flex flex-wrap gap-2">
          <Button size="sm" variant="destructive" onclick={rollback} disabled={rolling}>{$_('mailApi.rollbackConfirmAction')}</Button>
          <Button size="sm" variant="outline" onclick={() => confirmRollback = false} disabled={rolling}>{$_('common.cancel')}</Button>
        </div>
      </div>
    {/if}
  {/if}

  {#if report && report.unfinished?.length}
    <div class="text-sm"><p class="font-medium">{$_('mailApi.unfinished')}</p>
      <ul class="list-disc pl-5">{#each report.unfinished as act, i (i)}<li>{act.op}: {act.status}{act.error ? ` – ${act.error}` : ''}</li>{/each}</ul></div>
  {/if}
  {#if plan?.migration?.state === 'failed' && plan.migration.error}<p role="alert" class="text-sm text-destructive">{$_('mailApi.migrateFailed', { values: { error: plan.migration.error } })}</p>{/if}
  {#if error}<p role="alert" class="text-sm text-destructive">{error}</p>{/if}
  <div aria-live="polite" class="text-sm">{#if waiting}{$_('mailApi.waiting')}{:else if migrating}{$_('mailApi.migrating')}{:else if message}{message}{/if}</div>
</section>
