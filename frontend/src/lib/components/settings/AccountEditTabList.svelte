<script lang="ts">
  import Icon from '@iconify/svelte'
  import * as Tabs from '$lib/components/ui/tabs'
  import { _ } from '$lib/i18n'
</script>

<Tabs.List class="grid w-full grid-cols-4">
  <Tabs.Trigger value="general" class="flex items-center gap-2">
    <Icon icon="mdi:cog" class="w-4 h-4" />
    {$_('account.general')}
  </Tabs.Trigger>
  <Tabs.Trigger value="identity" class="flex items-center gap-2">
    <Icon icon="mdi:account-multiple" class="w-4 h-4" />
    {$_('account.identity')}
  </Tabs.Trigger>
  <Tabs.Trigger value="server" class="flex items-center gap-2">
    <Icon icon="mdi:server" class="w-4 h-4" />
    {$_('account.server')}
  </Tabs.Trigger>
  <Tabs.Trigger value="security" class="flex items-center gap-2">
    <Icon icon="mdi:shield-lock-outline" class="w-4 h-4" />
    {$_('account.security')}
  </Tabs.Trigger>
</Tabs.List>
