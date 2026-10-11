<script lang="ts">
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import type { ComposerApi } from '$lib/composerApi'
  import type { app } from '../../../../wailsjs/go/models'

  interface Props {
    api: ComposerApi
    /** Account the selected From identity belongs to */
    accountId: string
    /** Selected From identity */
    identityId: string
  }

  let { api, accountId, identityId }: Props = $props()

  let status = $state<app.SendAsStatus | null>(null)
  const sendAsState = $derived(status?.supported ? (status.identities?.[identityId] ?? '') : '')

  // Cached status only, fetched per account; a failed lookup shows no warning.
  $effect(() => {
    const acct = accountId
    status = null
    if (!acct) return
    api.getSendAsStatus(acct).then((st) => {
      if (acct === accountId) status = st
    }).catch(() => {})
  })
</script>

{#if sendAsState === 'not-configured' || sendAsState === 'pending'}
  <div class="flex items-center gap-2 px-4 py-1.5 border-b border-border text-xs text-amber-600 dark:text-amber-400" role="alert">
    <Icon icon="mdi:alert-outline" class="w-4 h-4 shrink-0" />
    {$_('composer.sendAsRewrite', { values: { email: status?.accountEmail } })}
  </div>
{/if}
