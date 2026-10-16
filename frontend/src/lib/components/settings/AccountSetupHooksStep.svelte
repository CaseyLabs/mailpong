<script lang="ts">
  import { Button } from '$lib/components/ui/button'
  import AccountContactsHookPanel from '$extensions/contacts/frontend/hooks/AccountContactsHookPanel.svelte'
  import AccountCalendarHookPanelGoogle from '$extensions/calendar/frontend/hooks/AccountCalendarHookPanelGoogle.svelte'
  import AccountCalendarHookPanelMicrosoft from '$extensions/calendar/frontend/hooks/AccountCalendarHookPanelMicrosoft.svelte'
  // @ts-ignore - wailsjs path
  import type { v1 } from '../../../../wailsjs/go/models'
  // @ts-ignore - wailsjs path
  import type { account } from '../../../../wailsjs/go/models'

  interface Props {
    hookAccount: account.Account
    pendingHooks: v1.AccountSetupHookRequest[]
    resolvedHooks: Set<string>
    onResolve: (extensionId: string) => void
    onSkipAll: () => void
  }

  let { hookAccount, pendingHooks, resolvedHooks, onResolve, onSkipAll }: Props = $props()
</script>

<div class="flex-1 overflow-y-auto pl-1 pr-3 pb-4" style="max-height: calc(90vh - 140px);">
  <p class="text-sm text-muted-foreground mb-3">
    Your account is added. Set up extras for this account, or skip.
  </p>
  {#each pendingHooks as hook (hook.extensionId)}
    {#if !resolvedHooks.has(hook.extensionId)}
      {#if hook.component === 'AccountContactsHookPanel'}
        <AccountContactsHookPanel
          {hook}
          accountId={hookAccount.id}
          accountName={hookAccount.name}
          onResolved={() => onResolve(hook.extensionId)}
        />
      {/if}
      {#if hook.component === 'AccountCalendarHookPanelGoogle'}
        <AccountCalendarHookPanelGoogle
          {hook}
          accountId={hookAccount.id}
          accountName={hookAccount.name}
          accountEmail={hookAccount.email ?? ''}
          onResolved={() => onResolve(hook.extensionId)}
        />
      {/if}
      {#if hook.component === 'AccountCalendarHookPanelMicrosoft'}
        <AccountCalendarHookPanelMicrosoft
          {hook}
          accountId={hookAccount.id}
          accountName={hookAccount.name}
          accountEmail={hookAccount.email ?? ''}
          onResolved={() => onResolve(hook.extensionId)}
        />
      {/if}
    {/if}
  {/each}
  <div class="flex items-center justify-end gap-2 pt-2 border-t border-border mt-4">
    <Button variant="ghost" onclick={onSkipAll}>Skip all</Button>
  </div>
</div>
