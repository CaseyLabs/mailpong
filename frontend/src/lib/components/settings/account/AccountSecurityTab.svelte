<script lang="ts">
  import { onMount } from 'svelte'
  import Icon from '@iconify/svelte'
  import { AccountSecurityState } from './accountSecurityState.svelte'
  import AccountSecurityPgpSection from './AccountSecurityPgpSection.svelte'
  import AccountSecuritySmimeSection from './AccountSecuritySmimeSection.svelte'
  import AccountSecurityDialogs from './AccountSecurityDialogs.svelte'

  interface Props {
    accountId: string
  }

  let { accountId }: Props = $props()

  const s = new AccountSecurityState({
    get accountId() { return accountId },
  })

  onMount(async () => {
    await s.loadData()
  })
</script>

<div class="space-y-6">
  {#if s.loading}
    <div class="flex items-center justify-center py-8">
      <Icon icon="mdi:loading" class="w-6 h-6 animate-spin text-muted-foreground" />
    </div>
  {:else}
    <AccountSecurityPgpSection {s} />
    <AccountSecuritySmimeSection {s} />
  {/if}
</div>

<AccountSecurityDialogs {s} />
