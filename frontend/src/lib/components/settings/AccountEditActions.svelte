<script lang="ts">
  import Icon from '@iconify/svelte'
  import { Button } from '$lib/components/ui/button'
  import { _ } from '$lib/i18n'

  interface Props {
    activeTab: string
    saving: boolean
    onCancel: () => void
    onSave: () => void
  }

  let { activeTab, saving, onCancel, onSave }: Props = $props()
</script>

<!-- Actions for General/Server tabs (not Identity - it has its own save) -->
{#if activeTab === 'identity' || activeTab === 'security'}
  <div class="flex items-center justify-end gap-2 pt-4 border-t border-border mt-4">
    <Button variant="ghost" onclick={onCancel}>
      {$_('common.close')}
    </Button>
  </div>
{:else}
  <div class="flex items-center justify-end gap-2 pt-4 border-t border-border mt-4">
    <Button variant="ghost" onclick={onCancel} disabled={saving}>
      {$_('common.cancel')}
    </Button>
    <Button onclick={onSave} disabled={saving}>
      {#if saving}
        <Icon icon="mdi:loading" class="w-4 h-4 mr-2 animate-spin" />
      {/if}
      {$_('common.saveChanges')}
    </Button>
  </div>
{/if}
