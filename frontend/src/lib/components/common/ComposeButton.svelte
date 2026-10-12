<script lang="ts">
  // Primary "Compose" call to action, shared by the chat list header and the
  // conversation header. Tonal: a tinted fill with an accent icon.
  import { _ } from '$lib/i18n'
  import ToolbarButton from './ToolbarButton.svelte'

  interface Props {
    onclick: () => void
    compact?: boolean
  }

  let { onclick, compact = false }: Props = $props()
</script>

<ToolbarButton icon="mdi:pencil" label={$_('sidebar.compose')} tonal {compact} {onclick} />
