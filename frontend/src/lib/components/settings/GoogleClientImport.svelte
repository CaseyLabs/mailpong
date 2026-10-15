<script lang="ts">
  // GoogleClientImport — "Import JSON…" for the Google custom-credentials
  // slots. Picks Google's downloaded client_secret JSON natively; the backend
  // parses it and saves the client to the chosen slots, so the secret never
  // reaches the UI. Lives beside (not inside) the kit slot editor so the kit
  // stays provider-agnostic.

  import Icon from '@iconify/svelte'
  import { _ } from 'svelte-i18n'
  import { Button } from '$lib/components/ui/button'
  import { toasts } from '$lib/stores/toast'
  // @ts-ignore - wailsjs bindings
  import { PickGoogleClientFile, ImportGoogleClientFile } from '$wailsjs/go/app/App'

  interface Props {
    onImported?: () => void
  }

  const { onImported }: Props = $props()

  const slots = [
    { id: 'google-mail', key: 'oauthImport.slotMail' },
    { id: 'google-contacts', key: 'oauthImport.slotContacts' },
    { id: 'google-calendar', key: 'oauthImport.slotCalendar' },
  ]

  let open = $state(false)
  let selected = $state<string[]>(slots.map(s => s.id))
  let busy = $state(false)
  let error = $state('')

  function toggle(id: string, checked: boolean) {
    selected = checked ? [...selected, id] : selected.filter(s => s !== id)
  }

  async function chooseFile() {
    error = ''
    busy = true
    try {
      const path = await PickGoogleClientFile()
      if (!path) return
      const result = await ImportGoogleClientFile(path, selected)
      toasts.success($_('oauthImport.imported', { values: { count: result.slots.length, id: result.clientIdFingerprint } }))
      open = false
      onImported?.()
    } catch (err) {
      error = (err as Error)?.message ?? String(err)
    } finally {
      busy = false
    }
  }
</script>

<div class="border border-border rounded-md p-4 bg-card">
  <div class="flex items-center justify-between gap-3 flex-wrap">
    <p class="text-sm text-muted-foreground flex-1 min-w-0">{$_('oauthImport.help')}</p>
    <Button size="sm" variant="outline" onclick={() => { open = !open; error = '' }} aria-expanded={open}>
      <Icon icon="mdi:file-import-outline" class="w-4 h-4 mr-1" />
      {$_('oauthImport.button')}
    </Button>
  </div>
  {#if open}
    <fieldset class="mt-3 space-y-1" disabled={busy}>
      <legend class="text-xs text-muted-foreground mb-1">{$_('oauthImport.applyTo')}</legend>
      {#each slots as slot (slot.id)}
        <label class="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={selected.includes(slot.id)} onchange={e => toggle(slot.id, e.currentTarget.checked)} />
          {$_(slot.key)}
        </label>
      {/each}
    </fieldset>
    {#if error}
      <p class="mt-2 text-sm text-destructive" role="alert">{error}</p>
    {/if}
    <div class="mt-3 flex justify-end">
      <Button size="sm" onclick={chooseFile} disabled={busy || selected.length === 0}>
        {#if busy}
          <Icon icon="mdi:loading" class="w-4 h-4 mr-1 animate-spin" />
        {/if}
        {$_('oauthImport.chooseFile')}
      </Button>
    </div>
  {/if}
</div>
