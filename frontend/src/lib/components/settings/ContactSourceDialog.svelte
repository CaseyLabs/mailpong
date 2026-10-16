<script lang="ts">
  import Icon from '@iconify/svelte'
  import * as Dialog from '$lib/components/ui/dialog'
  import * as Tabs from '$lib/components/ui/tabs'
  import { Button } from '$lib/components/ui/button'
  import { _ } from '$lib/i18n'
  import ServerCertificatePrompt from '$lib/components/kit/ServerCertificatePrompt.svelte'
  // @ts-ignore - wailsjs path
  import type { carddav } from '../../../../wailsjs/go/models'
  import { ContactSourceState } from './contactSourceState.svelte'
  import { registerContactSourceEffects } from './contactSourceEffects.svelte'
  import { handleSave } from './contactSourceActions'
  import ContactSourceCardDav from './ContactSourceCardDav.svelte'
  import ContactSourceOAuth from './ContactSourceOAuth.svelte'

  interface Props {
    open?: boolean
    editSource?: carddav.Source | null
    onClose?: () => void
  }

  let {
    open = $bindable(false),
    editSource = null,
    onClose,
  }: Props = $props()

  const s = new ContactSourceState({
    getOpen: () => open,
    setOpen: (v) => { open = v },
    getEditSource: () => editSource,
    getOnClose: () => onClose,
  })

  // Load existing source data when editing; set up OAuth event listeners
  registerContactSourceEffects(s)
</script>

<Dialog.Root bind:open={s.open} onOpenChange={s.handleOpenChange}>
  <Dialog.Content class="max-w-lg">
    <Dialog.Header>
      <Dialog.Title>{s.editSource ? $_('contactSource.editSource') : $_('contactSource.addSource')}</Dialog.Title>
      <Dialog.Description>
        {$_('contactSource.description')}
      </Dialog.Description>
    </Dialog.Header>

    {#if !s.editSource}
      <!-- Source type tabs (only for new sources) -->
      <Tabs.Root value={s.sourceType} onValueChange={s.handleTabChange} class="mt-4">
        <Tabs.List class="grid w-full grid-cols-3">
          <Tabs.Trigger value="carddav" class="flex items-center gap-1.5">
            <Icon icon="mdi:card-account-details" class="w-4 h-4" />
            CardDAV
          </Tabs.Trigger>
          <Tabs.Trigger value="google" class="flex items-center gap-1.5">
            <Icon icon="mdi:google" class="w-4 h-4" />
            Google
          </Tabs.Trigger>
          <Tabs.Trigger value="microsoft" class="flex items-center gap-1.5">
            <Icon icon="mdi:microsoft" class="w-4 h-4" />
            Microsoft
          </Tabs.Trigger>
        </Tabs.List>
      </Tabs.Root>
    {/if}

    <!-- Body scroll wrapper. Without max-h + overflow, mobile viewports can't
         reach the footer Save/Cancel. Footer stays outside this div so it
         stays pinned at the bottom regardless of body scroll position. -->
    <div class="space-y-4 py-4 max-h-[70vh] overflow-y-auto pr-1">
      {#if s.sourceType === 'carddav'}
        <!-- CardDAV Form -->
        <ContactSourceCardDav {s} />

      {:else}
        <!-- Google or Microsoft OAuth Form -->
        <ContactSourceOAuth {s} />
      {/if}
    </div>

    <!-- Actions -->
    <div class="flex items-center justify-end gap-2 pt-4 border-t border-border">
      <Button variant="ghost" onclick={s.handleCancel} disabled={s.saving}>
        {$_('common.cancel')}
      </Button>
      <Button
        onclick={() => handleSave(s)}
        disabled={s.saving || !s.canSave()}
      >
        {#if s.saving}
          <Icon icon="mdi:loading" class="w-4 h-4 mr-2 animate-spin" />
        {/if}
        {s.editSource ? $_('contactSource.update') : $_('common.add')}
      </Button>
    </div>
  </Dialog.Content>
</Dialog.Root>

<ServerCertificatePrompt bind:this={s.certPrompt} />
