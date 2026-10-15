<script lang="ts">
  import Icon from '@iconify/svelte'
  import { Button } from '$lib/components/ui/button'
  import { addToast } from '$lib/stores/toast'
  import { oauthStore } from '$lib/stores/oauth.svelte'
  import { _ } from '$lib/i18n'
  // @ts-ignore - wailsjs path
  import { app } from '../../../../../wailsjs/go/models'
  // @ts-ignore - wailsjs path
  import { CheckGoogleSetup, OpenURL } from '../../../../../wailsjs/go/app/App'

  interface Props {
    accountId: string
  }

  let { accountId }: Props = $props()

  let checking = $state(false)
  let reauthorizing = $state(false)
  let result = $state<app.GoogleSetupCheck | null>(null)

  const labels: Record<string, string> = {
    gmail: 'identity.googleCheckGmail',
    people: 'identity.googleCheckPeople',
    calendar: 'identity.googleCheckCalendar',
  }

  async function check() {
    checking = true
    try {
      result = await CheckGoogleSetup(accountId)
    } catch (err) {
      addToast({ type: 'error', message: $_('identity.googleCheckFailed') + ': ' + (err as Error).message })
    } finally {
      checking = false
    }
  }

  async function enable(url: string) {
    try {
      await OpenURL(url)
    } catch (err) {
      addToast({ type: 'error', message: (err as Error).message })
    }
  }

  async function reauthorize() {
    reauthorizing = true
    try {
      await oauthStore.reauthorize(accountId)
      await check()
    } catch (err) {
      addToast({ type: 'error', message: $_('identity.googleCheckReauthorizeFailed') + ': ' + (err as Error).message })
    } finally {
      reauthorizing = false
    }
  }
</script>

<section class="space-y-3 mt-6 pt-6 border-t border-border" aria-label={$_('identity.googleCheckTitle')}>
  <div class="flex items-center justify-between gap-3">
    <div>
      <h3 class="text-sm font-medium flex items-center gap-2">
        <Icon icon="mdi:google" class="w-4 h-4" />
        {$_('identity.googleCheckTitle')}
      </h3>
      <p class="text-xs text-muted-foreground mt-1">{$_('identity.googleCheckHelp')}</p>
    </div>
    <Button size="sm" variant="outline" onclick={check} disabled={checking}>
      <Icon icon={checking ? 'mdi:loading' : 'mdi:stethoscope'} class="w-4 h-4 mr-1 {checking ? 'animate-spin' : ''}" />
      {checking ? $_('identity.googleCheckRunning') : $_('identity.googleCheckRun')}
    </Button>
  </div>

  {#if result}
    <ul class="space-y-2" aria-live="polite">
      {#each result.results as r (r.api)}
        <li class="flex items-start gap-3 p-3 rounded-lg border border-border bg-card" data-state={r.state}>
          <div class="flex-1 min-w-0">
            <div class="text-sm font-medium">{$_(labels[r.api] ?? 'identity.googleCheckError')}</div>
            <div class="text-xs text-muted-foreground break-words">
              {#if r.state === 'ok'}
                {$_('identity.googleCheckOk')}
              {:else if r.state === 'disabled'}
                {$_('identity.googleCheckDisabled', { values: { project: r.project } })}. {$_('identity.googleCheckDisabledHint')}
              {:else if r.state === 'missing-scope'}
                {$_('identity.googleCheckMissingScope')}: {r.detail}
              {:else if r.state === 'not-connected'}
                {r.detail || $_('identity.googleCheckNotConnected')}
              {:else}
                {$_('identity.googleCheckError')}: {r.detail}
              {/if}
            </div>
          </div>
          {#if r.state === 'disabled'}
            <Button size="sm" onclick={() => enable(r.enableUrl)}>
              <Icon icon="mdi:open-in-new" class="w-4 h-4 mr-1" />
              {$_('identity.googleCheckEnable')}
            </Button>
          {:else if r.state === 'missing-scope'}
            <Button size="sm" onclick={reauthorize} disabled={reauthorizing}>
              <Icon icon={reauthorizing ? 'mdi:loading' : 'mdi:account-key'} class="w-4 h-4 mr-1 {reauthorizing ? 'animate-spin' : ''}" />
              {$_('identity.googleCheckReauthorize')}
            </Button>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
</section>
