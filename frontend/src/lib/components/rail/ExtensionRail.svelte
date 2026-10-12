<script lang="ts">
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import RailButton from './RailButton.svelte'
  import SettingsDialog from '$lib/components/settings/SettingsDialog.svelte'
  import { getRailTabs } from '$lib/stores/extensionRegistry.svelte'
  import { getActiveExtension, setActiveExtension } from '$lib/stores/uiState.svelte'
  import { contactSourcesStore } from '$lib/stores/contactSources.svelte'
  import { setFocusedPane } from '$lib/stores/keyboard.svelte'

  // Mail is always present and always first; extensions follow in their
  // registered Order. App settings sit at the bottom, so the rail always
  // renders, even when no extension is enabled.
  let active = $derived(getActiveExtension())
  let tabs = $derived(getRailTabs())
  let showSettingsDialog = $state(false)

  function select(name: string) {
    setActiveExtension(name)
  }

  function closeSettings() {
    showSettingsDialog = false
    if (active === 'mail') setFocusedPane('messageList')
  }
</script>

<nav
  class="flex flex-col items-stretch w-12 shrink-0 bg-surface-rail border-r border-divider pt-2"
  aria-label="Active extension"
>
  <RailButton
    icon="mdi:email"
    label="Mail"
    active={active === 'mail'}
    onclick={() => select('mail')}
  />
  {#each tabs as tab (tab.extensionId)}
    <RailButton
      icon={tab.icon || 'mdi:puzzle'}
      label={tab.label}
      active={active === tab.extensionId}
      onclick={() => select(tab.extensionId)}
    />
  {/each}

  <button
    class="relative mt-auto flex items-center justify-center w-12 h-11 text-muted-foreground hover:text-foreground hover:bg-accent/30 transition-colors duration-150 cursor-pointer focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-primary focus-visible:-outline-offset-2"
    type="button"
    title={$_('sidebar.settings')}
    aria-label={$_('sidebar.settings')}
    onclick={() => showSettingsDialog = true}
  >
    <Icon icon="mdi:cog" width="22" height="22" />
    {#if contactSourcesStore.hasErrors}
      <span class="absolute top-2.5 right-2.5 w-2.5 h-2.5 bg-destructive rounded-full border border-background"></span>
    {/if}
  </button>
</nav>

<SettingsDialog bind:open={showSettingsDialog} onClose={closeSettings} />
