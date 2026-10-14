<script lang="ts">
  import { onDestroy } from 'svelte'
  import { _ } from '$lib/i18n'
  import { Button } from '$lib/components/ui/button'
  import { RunSetupDiagnostic, CancelSetupDiagnostic, ExportSetupDiagnostic } from '../../../../../wailsjs/go/app/App'
  import { account, app } from '../../../../../wailsjs/go/models'
  let { accountId, identities }: { accountId: string, identities: account.Identity[] } = $props()
  let from = $state('')
  let domain = $state('')
  let owned = $state(false)
  let selectors = $state('')
  let busy = $state(false)
  let cancelling = $state(false)
  let runId = $state('')
  let report = $state<app.SetupDiagnosticReport | null>(null)
  let error = $state('')
  let exported = $state(false)
  const selectedFrom = $derived(from || identities[0]?.email || '')
  async function start() {
    if (busy || !selectedFrom) return
    busy = true; cancelling = false; error = ''; report = null; exported = false
    runId = crypto.randomUUID()
    try {
      report = await RunSetupDiagnostic(new app.SetupDiagnosticRequest({
        runId, accountId, from: selectedFrom, route: 'imap-smtp', domain: owned ? domain.trim() : '',
        ownedDomain: owned, dkimSelectors: owned ? selectors.split(/[ ,]+/).filter(Boolean) : [],
      }))
    } catch { error = $_('setupDiagnostic.failed') }
    finally { busy = false; cancelling = false }
  }
  async function cancel() {
    cancelling = true
    try { await CancelSetupDiagnostic(runId) }
    catch { error = $_('setupDiagnostic.cancelFailed'); cancelling = false }
  }
  async function exportReport() {
    error = ''; exported = false
    try {
      exported = await ExportSetupDiagnostic(runId)
    } catch { error = $_('setupDiagnostic.exportFailed') }
  }
  onDestroy(() => { if (busy) void CancelSetupDiagnostic(runId).catch(() => {}) })
</script>
<section class="space-y-3 border-t border-border pt-4" aria-label={$_('setupDiagnostic.title')}>
  <h3 class="font-medium">{$_('setupDiagnostic.title')}</h3>
  <p class="text-sm text-muted-foreground">{$_('setupDiagnostic.guidance')}</p>
  <label class="block space-y-1 text-sm">
    <span>{$_('setupDiagnostic.from')}</span>
    <select class="block w-full rounded border border-border bg-background p-2" aria-label={$_('setupDiagnostic.from')} bind:value={from} disabled={busy}>
      <option value="">{identities[0]?.email || $_('setupDiagnostic.chooseIdentity')}</option>
      {#each identities as identity (identity.id)}<option value={identity.email}>{identity.email}</option>{/each}
    </select>
  </label>
  <label class="block space-y-1 text-sm">
    <span>{$_('setupDiagnostic.route')}</span>
    <select aria-label={$_('setupDiagnostic.route')} class="block w-full rounded border border-border bg-background p-2" disabled={busy}><option value="imap-smtp">{$_('setupDiagnostic.existingRoute')}</option></select>
  </label>
  <label class="flex items-center gap-2 text-sm"><input type="checkbox" bind:checked={owned} disabled={busy} />{$_('setupDiagnostic.owned')}</label>
  {#if owned}
    <label class="block space-y-1 text-sm"><span>{$_('setupDiagnostic.domain')}</span><input class="block w-full rounded border border-border bg-background p-2" bind:value={domain} disabled={busy} maxlength="253" /></label>
    <label class="block space-y-1 text-sm"><span>{$_('setupDiagnostic.selectors')}</span><input class="block w-full rounded border border-border bg-background p-2" bind:value={selectors} disabled={busy} maxlength="640" /></label>
  {/if}
  <div class="flex flex-wrap gap-2">
    <Button size="sm" onclick={start} disabled={busy || !selectedFrom || (owned && !domain.trim())}>{busy ? $_('setupDiagnostic.running') : $_('setupDiagnostic.start')}</Button>
    {#if busy}<Button size="sm" variant="outline" onclick={cancel} disabled={cancelling}>{cancelling ? $_('setupDiagnostic.cancelling') : $_('common.cancel')}</Button>{/if}
    {#if report && !busy}<Button size="sm" variant="outline" onclick={exportReport}>{$_('setupDiagnostic.export')}</Button>{/if}
  </div>
  {#if error}<p role="alert" class="text-sm text-destructive">{error}</p>{/if}
  {#if exported}<p role="status" class="text-sm">{$_('setupDiagnostic.exportSaved')}</p>{/if}
  <div aria-live="polite" class="space-y-3 text-sm">
    {#if report}
      <p class="break-all">{$_('setupDiagnostic.checkedSelection', { values: { mailbox: report.mailbox, from: report.from } })}</p>
      <p>{report.startedAt} – {report.finishedAt}</p>
      {#each report.endpoints || [] as endpoint (endpoint)}<p class="break-all">{endpoint}</p>{/each}
      <p class="break-all">{$_('setupDiagnostic.oauth')}: {report.oauthResource || $_('setupDiagnostic.notApplicable')} · {report.oauthExpiresAt || $_('setupDiagnostic.unknown')}</p>
      <p class="break-all">{$_('setupDiagnostic.scopes')}: {report.oauthScopesKnown ? (report.oauthScopes || []).join(', ') || $_('setupDiagnostic.none') : $_('setupDiagnostic.unknown')}</p>
      {#each report.checks || [] as check, i (i)}
        <div class="rounded border border-border p-2">
          <p class="font-medium">{check.name}: {check.status}</p>
          <p>{check.detail}</p><p class="text-xs text-muted-foreground">{check.at} · {$_('setupDiagnostic.age')}: {check.evidenceAgeSeconds < 0 ? $_('setupDiagnostic.unknown') : `${check.evidenceAgeSeconds}s`} {check.code}</p>
        </div>
      {/each}
      {#each report.dns?.checks || [] as check, i (i)}
        <div class="rounded border border-border p-2 break-all"><p class="font-medium">{check.query}: {check.status}</p><p>{check.evidence}</p><p class="text-xs text-muted-foreground">{check.resolver} · TTL: {check.ttlSeconds ?? $_('setupDiagnostic.unknown')} · {check.observedAt}</p></div>
      {/each}
      <p class="text-muted-foreground">{report.limitations}</p>
    {/if}
  </div>
</section>
