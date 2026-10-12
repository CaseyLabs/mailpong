<script lang="ts">
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import { WindowMinimise, WindowToggleMaximise, Quit } from '../../../../wailsjs/runtime/runtime'

  interface Props {
    onClose?: () => void
  }

  let { onClose }: Props = $props()

  let isMaximized = $state(false)
  let isHovering = $state(false)

  async function minimize() {
    await WindowMinimise()
  }

  async function toggleMaximize() {
    await WindowToggleMaximise()
    isMaximized = !isMaximized
  }

  function close() {
    if (onClose) {
      onClose()
    } else {
      // For windows without custom onClose (e.g., composer), just quit directly
      Quit()
    }
  }
</script>

<header class="h-10 flex items-center justify-between bg-muted/50 border-b border-border select-none shrink-0">
  <!-- Drag region - left side with app title -->
  <div class="flex-1 flex items-center gap-2 px-3 h-full" style="--wails-draggable: drag">
    <Icon icon="mdi:email-fast-outline" class="w-5 h-5 text-primary" />
    <span class="text-sm font-medium text-foreground">MailPong</span>
  </div>

  <!-- Mac-style traffic light controls -->
  <div
    class="flex items-center gap-2 px-3 h-full"
    role="group"
    aria-label={$_('aria.windowControls')}
    onmouseenter={() => isHovering = true}
    onmouseleave={() => isHovering = false}
  >
    <!-- Minimize (yellow) -->
    <button
      class="w-3 h-3 rounded-full flex items-center justify-center transition-all bg-[#FEBC2E] hover:brightness-90 active:brightness-75"
      onclick={minimize}
      title={$_('window.minimize')}
      aria-label={$_('aria.minimizeWindow')}
    >
      {#if isHovering}
        <span class="text-2xs font-bold text-black/60 leading-none">−</span>
      {/if}
    </button>

    <!-- Maximize/Restore (green) -->
    <button
      class="w-3 h-3 rounded-full flex items-center justify-center transition-all bg-[#28C840] hover:brightness-90 active:brightness-75"
      onclick={toggleMaximize}
      title={isMaximized ? $_('window.restore') : $_('window.maximize')}
      aria-label={isMaximized ? $_('aria.restoreWindow') : $_('aria.maximizeWindow')}
    >
      {#if isHovering}
        <span class="text-2xs font-bold text-black/60 leading-none">+</span>
      {/if}
    </button>

    <!-- Close (red) -->
    <button
      class="w-3 h-3 rounded-full flex items-center justify-center transition-all bg-[#FF5F57] hover:brightness-90 active:brightness-75"
      onclick={close}
      title={$_('window.close')}
      aria-label={$_('aria.closeWindow')}
    >
      {#if isHovering}
        <span class="text-2xs font-bold text-black/60 leading-none">×</span>
      {/if}
    </button>
  </div>
</header>
