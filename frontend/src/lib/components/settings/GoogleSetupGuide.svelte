<script lang="ts">
  // GoogleSetupGuide — checklist for creating your own Google Cloud OAuth
  // client. Console links and the scope list come from the backend
  // (internal/oauth2), so they cannot drift from what Mailven requests. Links
  // open in the system browser; each step also names the menu path to use if a
  // deep link does not land on the right page. Buttons are type="button":
  // the guide sits inside the Add Account form and must not submit it.

  import Icon from '@iconify/svelte'
  import { onMount } from 'svelte'
  import { _ } from 'svelte-i18n'
  import { Button } from '$lib/components/ui/button'
  import { toasts } from '$lib/stores/toast'
  import GoogleClientImport from './GoogleClientImport.svelte'
  // @ts-ignore - wailsjs bindings
  import { GetGoogleSetupGuide, OpenURL } from '$wailsjs/go/app/App'

  interface Props {
    // Show Import JSON… inside the import step; used where no import box sits
    // beside the guide (the Add Account form).
    inlineImport?: boolean
    // Called after an inline import saved a client.
    onImported?: () => void
  }

  let { inlineImport = false, onImported }: Props = $props()

  interface GuideStep { id: string; url: string; fallbackUrl: string; menuPath: string }

  // Steps with a console page take their link from the backend; the last two
  // happen inside Mailven.
  const stepIds = ['project', 'apis', 'branding', 'audience', 'scopes', 'clients', 'import', 'signin']

  let open = $state(false)
  let links = $state<Record<string, GuideStep>>({})
  let scopes = $state<string[]>([])
  let done = $state<Record<string, boolean>>({})
  let loadError = $state('')

  onMount(async () => {
    try {
      const guide = await GetGoogleSetupGuide()
      links = Object.fromEntries((guide.steps ?? []).map((s: GuideStep) => [s.id, s]))
      scopes = guide.scopes ?? []
    } catch (err) {
      loadError = (err as Error)?.message ?? String(err)
    }
  })

  async function openLink(url: string) {
    try {
      await OpenURL(url)
    } catch (err) {
      toasts.error((err as Error)?.message ?? String(err))
    }
  }

  async function copyScopes() {
    try {
      await navigator.clipboard.writeText(scopes.join('\n'))
      toasts.success($_('googleGuide.scopesCopied'))
    } catch {
      toasts.error($_('googleGuide.copyFailed'))
    }
  }
</script>

<div class="border border-border rounded-md p-4 bg-card">
  <div class="flex items-center justify-between gap-3 flex-wrap">
    <p class="text-sm text-muted-foreground flex-1 min-w-0">{$_('googleGuide.help')}</p>
    <Button type="button" size="sm" variant="outline" onclick={() => { open = !open }} aria-expanded={open}>
      <Icon icon="mdi:format-list-checks" class="w-4 h-4 mr-1" />
      {$_('googleGuide.button')}
    </Button>
  </div>
  {#if open}
    {#if loadError}
      <p class="mt-3 text-sm text-destructive" role="alert">{loadError}</p>
    {/if}
    <ol class="mt-3 space-y-3">
      {#each stepIds as id, i (id)}
        {@const link = links[id]}
        <li class="flex gap-3">
          <input
            type="checkbox"
            class="mt-1 self-start"
            aria-label={$_('googleGuide.markDone', { values: { step: $_(`googleGuide.step.${id}.title`) } })}
            bind:checked={done[id]}
          />
          <div class="flex-1 min-w-0 space-y-1">
            <p class="text-sm font-medium" class:line-through={done[id]}>{i + 1}. {$_(`googleGuide.step.${id}.title`)}</p>
            <p class="text-xs text-muted-foreground">{$_(`googleGuide.step.${id}.${id === 'import' && inlineImport ? 'bodyInline' : 'body'}`)}</p>
            {#if id === 'import' && inlineImport}
              <GoogleClientImport {onImported} />
            {/if}
            {#if id === 'scopes' && scopes.length > 0}
              <ul class="text-xs font-mono break-all space-y-0.5" aria-label={$_('googleGuide.scopeList')}>
                {#each scopes as scope (scope)}
                  <li>{scope}</li>
                {/each}
              </ul>
              <Button type="button" size="sm" variant="outline" onclick={copyScopes}>
                <Icon icon="mdi:content-copy" class="w-4 h-4 mr-1" />
                {$_('googleGuide.copyScopes')}
              </Button>
            {/if}
            {#if link}
              <div class="flex items-center gap-2 flex-wrap">
                <Button type="button" size="sm" variant="outline" onclick={() => openLink(link.url)}>
                  <Icon icon="mdi:open-in-new" class="w-4 h-4 mr-1" />
                  {$_(`googleGuide.step.${id}.open`)}
                </Button>
                <span class="text-xs text-muted-foreground">
                  {$_('googleGuide.notRight', { values: { path: link.menuPath } })}
                  <button type="button" class="underline hover:text-foreground" onclick={() => openLink(link.fallbackUrl)}>
                    {$_('googleGuide.openConsole')}
                  </button>
                </span>
              </div>
            {/if}
          </div>
        </li>
      {/each}
    </ol>
  {/if}
</div>
