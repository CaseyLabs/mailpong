<script lang="ts">
  // ChatSecurityBanners — S/MIME and PGP status plus the read-receipt prompt
  // for one message, with the same states and strings as the classic viewer.
  // Positive states (signed, encrypted) are left to the bubble's meta-line
  // badges.
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import type { message as messageModels } from '../../../../wailsjs/go/models'
  import type { ChatThread } from './chatThread.svelte'
  import { TONES, type SecurityLine } from './chatSecurity'

  interface Props {
    msg: messageModels.Message
    thread: ChatThread
    /** securityStatus(msg, thread, $_), computed once by the bubble. */
    lines: SecurityLine[]
  }

  let { msg, thread, lines }: Props = $props()

  const banners = $derived(lines.filter((l) => !l.badge))

  const showReceipt = $derived(thread.shouldShowReadReceipt(msg))
  const sending = $derived(thread.sendingReadReceipt.has(msg.id))
</script>

{#if showReceipt && thread.readReceiptPolicy === 'ask'}
  <div class="flex flex-wrap items-center justify-between gap-2 px-3 py-2 border rounded-md text-sm {TONES.info}">
    <span class="flex items-center gap-2">
      <Icon icon="mdi:email-check-outline" class="w-4 h-4 shrink-0" />
      {$_('viewer.readReceiptRequested')}
    </span>
    <span class="flex items-center gap-2">
      <button
        class="px-3 py-1 text-xs font-medium text-background bg-info-foreground hover:opacity-90 rounded transition-opacity disabled:opacity-50"
        disabled={sending}
        onclick={() => thread.sendReadReceipt(msg)}
      >
        {#if sending}<Icon icon="mdi:loading" class="w-3 h-3 animate-spin" />{:else}{$_('viewer.sendReceipt')}{/if}
      </button>
      <button class="px-3 py-1 text-xs font-medium rounded hover:bg-info/20 transition-colors" onclick={() => thread.ignoreReadReceipt(msg)}>
        {$_('viewer.ignoreReceipt')}
      </button>
    </span>
  </div>
{:else if showReceipt && thread.readReceiptPolicy === 'always' && sending}
  <div class="flex items-center gap-2 px-3 py-2 border rounded-md text-sm {TONES.ok}">
    <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
    {$_('viewer.sendingReadReceipt')}
  </div>
{/if}

{#each banners as b (b.icon + b.text)}
  <div class="flex items-center gap-2 px-3 py-1.5 border rounded-md text-xs {TONES[b.tone]}">
    <Icon icon={b.icon} class="w-4 h-4 shrink-0 {b.spin ? 'animate-spin' : ''}" />
    <span>{b.text}</span>
  </div>
{/each}
