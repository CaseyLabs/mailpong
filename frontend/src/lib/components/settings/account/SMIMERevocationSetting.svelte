<script lang="ts">
  import { onMount } from 'svelte'
  import { _ } from '$lib/i18n'
  import { Label } from '$lib/components/ui/label'
  import Switch from '$lib/components/ui/switch/Switch.svelte'
  import { toasts } from '$lib/stores/toast'
  import { GetSMIMEOnlineRevocation, SetSMIMEOnlineRevocation } from '$wailsjs/go/app/App'

  let enabled = $state(false)
  let busy = $state(true)

  onMount(async () => {
    try {
      enabled = await GetSMIMEOnlineRevocation()
    } catch {
      toasts.error($_('security.revocationLoadFailed'))
    } finally {
      busy = false
    }
  })

  async function change(value: boolean) {
    busy = true
    try {
      await SetSMIMEOnlineRevocation(value)
      enabled = value
    } catch {
      enabled = !value
      toasts.error($_('security.revocationSaveFailed'))
    } finally {
      busy = false
    }
  }
</script>

<div class="flex items-start justify-between gap-4 rounded-lg border p-3">
  <div class="space-y-1">
    <Label for="smime-online-revocation">{$_('security.onlineRevocation')}</Label>
    <p class="text-xs text-muted-foreground">{$_('security.onlineRevocationHelp')}</p>
  </div>
  <Switch class="shrink-0" ariaLabel={$_('security.onlineRevocation')} id="smime-online-revocation" bind:checked={enabled} disabled={busy} onCheckedChange={change} />
</div>
