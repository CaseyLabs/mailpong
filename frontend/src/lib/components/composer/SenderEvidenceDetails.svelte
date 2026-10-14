<script lang="ts">
  import { _ } from '$lib/i18n'
  import type { app } from '../../../../wailsjs/go/models'
  let { evidence }: { evidence: app.SenderEvidence } = $props()
  const observed = $derived(evidence.observedAt
    ? $_('composer.senderObservationAge', { values: { minutes: Math.max(0, Math.floor((Date.now() - Date.parse(evidence.observedAt)) / 60000)), time: evidence.observedAt } })
    : $_('composer.senderNotObserved'))
</script>

<div class="space-y-1 text-xs text-muted-foreground" role="status">
  <p>{$_('composer.senderEvidenceSummary', { values: { state: $_(`composer.senderState_${evidence.state}`), source: evidence.source, route: evidence.route, observed } })}</p>
  {#if evidence.observedAt}
    <p>{$_('composer.senderLastObserved', { values: { state: $_(`composer.senderState_${evidence.observedState}`) } })}</p>
  {/if}
  <p>{evidence.guidance}</p>
  {#if evidence.source.includes('Microsoft') || evidence.sharedRead === 'configured'}
    <p>{$_('composer.senderDelegation', { values: { sendAs: $_(`composer.senderState_${evidence.sendAs}`), behalf: $_(`composer.senderState_${evidence.sendOnBehalf}`), read: $_(`composer.senderState_${evidence.sharedRead}`) } })}</p>
  {/if}
</div>
