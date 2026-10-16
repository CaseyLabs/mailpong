<script lang="ts">
  import Icon from '@iconify/svelte'
  import { Button } from '$lib/components/ui/button'
  import { _ } from '$lib/i18n'

  interface Props {
    onAddAccount: () => void
  }

  let { onAddAccount }: Props = $props()
</script>

<div class="flex flex-col items-center justify-center py-8 px-4 text-center">
  <Icon icon="mdi:email-plus-outline" class="w-12 h-12 text-muted-foreground mb-3" />
  <h3 class="text-sm font-medium mb-1">{$_('sidebar.noAccountsYet')}</h3>
  <p class="text-xs text-muted-foreground mb-4">
    {$_('sidebar.addFirstAccount')}
  </p>
  <Button size="sm" onclick={onAddAccount}>
    <Icon icon="mdi:plus" class="w-4 h-4 mr-1" />
    {$_('sidebar.addAccount')}
  </Button>
</div>
