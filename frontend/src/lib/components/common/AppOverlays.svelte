<script lang="ts">
  // Window-level overlays driven by backend events: the shutdown cover, the
  // certificate prompt for background sync and calendar errors, and the
  // Flatpak filesystem permission notice.
  import { onMount } from 'svelte'
  import CertificateDialog from '$lib/components/settings/CertificateDialog.svelte'
  import * as AlertDialog from '$lib/components/ui/alert-dialog'
  import { flushChatComposers } from '$lib/components/chat/chatComposerState.svelte'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { shutdown } from '$lib/stores/shutdown.svelte'
  import { _ } from '$lib/i18n'
  // @ts-ignore - wailsjs path
  import { AcceptCertificate, GetServerCertificateFailure, ChatDraftsFlushed } from '../../../../wailsjs/go/app/App.js'
  // @ts-ignore - wailsjs path
  import type { certificate } from '../../../../wailsjs/go/models'
  // @ts-ignore - wailsjs runtime
  import { EventsOn } from '../../../../wailsjs/runtime/runtime'

  // Certificate TOFU state (for background sync cert errors)
  let showCertDialog = $state(false)
  let pendingCertificate = $state<certificate.CertificateInfo | null>(null)
  let pendingCertAccountId = $state<string | null>(null)
  let pendingCertHost = $state('')

  // Flatpak filesystem permission dialog state
  let showFlatpakFsDialog = $state(false)
  let flatpakFsDialogContext = $state<'open' | 'saveAll'>('open')

  async function handleBgCertAccept(permanent: boolean) {
    if (!pendingCertificate) return
    try {
      const acc = accountStore.accounts.find(a => a.account.id === pendingCertAccountId)
      const host = pendingCertHost || acc?.account.imapHost || ''
      await AcceptCertificate(host, pendingCertificate, permanent)
    } catch (err) {
      console.error('Failed to accept certificate:', err)
    }
    handleBgCertDecline()
  }

  function handleBgCertDecline() {
    showCertDialog = false
    pendingCertificate = null
    pendingCertAccountId = null
    pendingCertHost = ''
  }

  onMount(() => {
    // Shutdown event from the backend (triggered by the OS close signal)
    EventsOn('app:shutting-down', () => {
      shutdown.active = true
      // Save chat composer text before the backend quits; it waits for this.
      void flushChatComposers().finally(() => ChatDraftsFlushed())
    })

    // Untrusted certificate events from background sync
    EventsOn('certificate:untrusted', (data: { accountId?: string; host?: string; certificate: certificate.CertificateInfo }) => {
      // Only show if not already showing a cert dialog
      if (!showCertDialog) {
        pendingCertificate = data.certificate
        pendingCertAccountId = data.accountId ?? null
        pendingCertHost = data.host ?? ''
        showCertDialog = true
      }
    })

    // Calendar owns its scheduler and reports DAV failures through its event bus.
    EventsOn('calendar:source-error', async (data: { message: string; certificateFailure?: string }) => {
      if (showCertDialog) return
      try {
        const failure = await GetServerCertificateFailure(data.certificateFailure ?? data.message)
        if (!showCertDialog && failure.certificateRequired && failure.host && failure.certificate) {
          pendingCertificate = failure.certificate
          pendingCertAccountId = null
          pendingCertHost = failure.host
          showCertDialog = true
        }
      } catch (err) {
        console.error('Failed to resolve calendar certificate error:', err)
      }
    })

    // Flatpak filesystem permission dialog. Optional payload picks the
    // wording: 'saveAll' (bulk attachment save, #384) vs the original
    // toast-open case (no payload).
    EventsOn('flatpak:filesystem-dialog', (context?: string) => {
      flatpakFsDialogContext = context === 'saveAll' ? 'saveAll' : 'open'
      showFlatpakFsDialog = true
    })
  })
</script>

<!-- Shutdown Overlay -->
{#if shutdown.active}
  <div class="fixed inset-0 z-100 flex items-center justify-center bg-black/80">
    <p class="text-white/90 text-sm font-medium">{$_('window.shuttingDown')}</p>
  </div>
{/if}

<!-- Certificate TOFU Dialog (for background sync cert errors) -->
<CertificateDialog
  bind:open={showCertDialog}
  certificate={pendingCertificate}
  onAcceptOnce={() => handleBgCertAccept(false)}
  onAcceptPermanently={() => handleBgCertAccept(true)}
  onDecline={handleBgCertDecline}
/>

<!-- Flatpak Filesystem Permission Dialog -->
<AlertDialog.Root bind:open={showFlatpakFsDialog}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>{$_('attachment.flatpakOpenTitle')}</AlertDialog.Title>
      <AlertDialog.Description>
        <p class="mb-3">{flatpakFsDialogContext === 'saveAll' ? $_('attachment.flatpakSaveAllDescription') : $_('attachment.flatpakOpenDescription')}</p>
        <pre class="mb-3 rounded bg-muted p-2 text-sm overflow-x-auto"><code>flatpak override --user --filesystem=home io.github.caseylabs.Mailven</code></pre>
        <p class="mb-3 text-sm text-destructive">{$_('attachment.flatpakOpenSecurityWarning')}</p>
        <p class="text-sm text-muted-foreground">{flatpakFsDialogContext === 'saveAll' ? $_('attachment.flatpakSaveAllAlternative') : $_('attachment.flatpakOpenAlternative')}</p>
      </AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Action onclick={() => showFlatpakFsDialog = false}>{$_('common.ok')}</AlertDialog.Action>
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
