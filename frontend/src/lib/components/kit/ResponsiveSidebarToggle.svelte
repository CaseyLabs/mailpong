<script lang="ts">
  // ResponsiveSidebarToggle — drop-in left-sidebar toggle for every view's
  // list/toolbar header (mail's ChatList, kit ListHeader, Calendar's
  // ViewSwitcher). Fires toggleActiveSidebar(): in narrow mode that opens or
  // closes the slide-in overlay; in full/medium mode it collapses or expands
  // the active view's sidebar (persisted per view).
  //
  // Consumers just compose:
  //
  //   <ResponsiveSidebarToggle />
  //
  // at the leading edge of their toolbar. No props, no plumbing.

  import Icon from '@iconify/svelte'
  import { _ } from 'svelte-i18n'
  import { getLayoutMode, isSidebarHidden, toggleActiveSidebar } from '$lib/stores/layout.svelte'

  const narrow = $derived(getLayoutMode() === 'narrow')
  const hidden = $derived(isSidebarHidden())
  const label = $derived($_(narrow ? 'aria.toggleSidebar' : hidden ? 'aria.showSidebar' : 'aria.hideSidebar'))
</script>

<button
  type="button"
  class="p-1.5 -ml-1 rounded-md hover:bg-muted transition-colors shrink-0"
  title={label}
  aria-label={label}
  aria-expanded={narrow ? undefined : !hidden}
  onclick={toggleActiveSidebar}
>
  <Icon icon="mdi:dock-left" class="w-5 h-5 text-muted-foreground" />
</button>
