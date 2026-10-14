<script lang="ts">
  import { _ } from '$lib/i18n'
  import type { ComposerApi } from '$lib/composerApi'
  import type { app } from '../../../../wailsjs/go/models'
  import SenderEvidenceDetails from './SenderEvidenceDetails.svelte'

  let { api, accountId, email }: { api: ComposerApi; accountId: string; email: string } = $props()
  let evidence = $state<app.SenderEvidence | null>(null)
  let failed = $state(false)
  $effect(() => {
    const acct = accountId
    const from = email
    let current = true
    evidence = null
    failed = false
    if (acct && from) {
      api.getSenderEvidence(acct, from).then(e => {
        if (current) evidence = e
      }).catch(() => { if (current) failed = true })
    }
    return () => { current = false }
  })
</script>

{#if evidence || failed}
  <div class="px-4 py-2 border-b border-border">
    {#if evidence}
      <SenderEvidenceDetails {evidence} />
    {:else}
      <p class="text-xs text-muted-foreground" role="alert">{$_('composer.senderLookupFailed')}</p>
    {/if}
  </div>
{/if}
