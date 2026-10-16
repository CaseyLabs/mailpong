<script lang="ts">
  import Icon from '@iconify/svelte'
  import * as DropdownMenu from '$lib/components/ui/dropdown-menu'
  import { _ } from '$lib/i18n'
  import { extractDomain } from './emailBodyHtml'

  interface Props {
    fromEmail: string
    open: boolean
    onLoadImages: () => void
    onAlwaysLoadDomain: () => void
    onAlwaysLoadSender: () => void
  }

  let { fromEmail, open = $bindable(), onLoadImages, onAlwaysLoadDomain, onAlwaysLoadSender }: Props = $props()
</script>

  <!-- Compact split button instead of a banner: load once, or always via the chevron. -->
  <div class="flex justify-end mb-2">
    <div class="inline-flex items-stretch rounded-md border border-border text-xs text-muted-foreground">
      <button
        class="flex items-center gap-1.5 px-2 py-1 rounded-l-md hover:bg-muted hover:text-foreground transition-colors focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring {fromEmail ? '' : 'rounded-r-md'}"
        title={$_('viewer.remoteImagesBlocked')}
        onclick={onLoadImages}
      >
        <Icon icon="mdi:image-off-outline" class="w-3.5 h-3.5 shrink-0" />
        {$_('viewer.loadImages')}
      </button>

      {#if fromEmail}
        <DropdownMenu.Root bind:open>
          <DropdownMenu.Trigger
            class="flex items-center px-1 border-l border-border rounded-r-md hover:bg-muted hover:text-foreground transition-colors focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring"
            title={$_('viewer.alwaysLoad')}
            aria-label={$_('viewer.alwaysLoad')}
          >
            <Icon icon="mdi:chevron-down" class="w-3.5 h-3.5" />
          </DropdownMenu.Trigger>
          <DropdownMenu.Content align="end">
            <div class="px-2 py-1.5 text-xs font-medium text-muted-foreground">{$_('viewer.alwaysLoad')}</div>
            <DropdownMenu.Item onSelect={onAlwaysLoadDomain}>
              <Icon icon="mdi:domain" class="w-4 h-4 mr-2" />
              {$_('viewer.forDomain', { values: { domain: extractDomain(fromEmail) || 'this domain' } })}
            </DropdownMenu.Item>
            <DropdownMenu.Item onSelect={onAlwaysLoadSender}>
              <Icon icon="mdi:account" class="w-4 h-4 mr-2" />
              {$_('viewer.forSender', { values: { email: fromEmail } })}
            </DropdownMenu.Item>
          </DropdownMenu.Content>
        </DropdownMenu.Root>
      {/if}
    </div>
  </div>
