<script lang="ts">
  // ChatLowGroupHeader — the Low priority group's collapse toggle, with
  // "Archive all" (undoable) and "Mark all read" for every low chat in scope,
  // loaded or not.
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import { chatList, type ChatItem } from '$lib/stores/chat.svelte'
  import { toasts } from '$lib/stores/toast'
  import { isChatGroupExpanded, toggleChatGroupExpanded } from '$lib/stores/uiState.svelte'
  import { archiveChats, markChatsRead } from './chatTriage'

  interface Props {
    // Runs after Archive all and after its Undo.
    onArchived: () => void
  }

  let { onArchived }: Props = $props()

  let busy = $state(false)
  const expanded = $derived(isChatGroupExpanded('low'))

  async function withLowChats(action: (chats: ChatItem[]) => Promise<unknown>) {
    if (busy) return
    busy = true
    try {
      const chats = await chatList.lowGroupChats()
      if (chats.length > 0) await action(chats)
    } catch (err) {
      console.error('Failed to load low priority chats:', err)
      toasts.error($_('chat.stateChangeFailed'))
    } finally {
      busy = false
    }
  }

  const archiveAll = () => withLowChats(async (chats) => {
    if (await archiveChats(chats, onArchived)) onArchived()
  })
  const markAllRead = () => withLowChats((chats) => markChatsRead(chats))
</script>

{#snippet action(icon: string, label: string, onclick: () => void)}
  <button class="p-1.5 rounded-md hover:bg-muted disabled:opacity-50" title={label} aria-label={label} disabled={busy} {onclick}>
    <Icon {icon} class="w-4 h-4" />
  </button>
{/snippet}

<div class="flex items-center w-[calc(100%-0.75rem)] mx-1.5 mt-2 rounded-lg text-sm text-muted-foreground hover:bg-muted/60">
  <button
    class="flex flex-1 min-w-0 items-center gap-1.5 px-3 py-2 text-left"
    aria-expanded={expanded}
    onclick={() => toggleChatGroupExpanded('low')}
  >
    <Icon icon={expanded ? 'mdi:chevron-down' : 'mdi:chevron-right'} class="w-4 h-4 shrink-0" />
    <Icon icon="mdi:newspaper-variant-outline" class="w-4 h-4 shrink-0" />
    <span class="truncate">{$_('chat.lowPriorityGroup', { values: { count: chatList.lowCount } })}</span>
  </button>
  <div class="flex items-center gap-0.5 pr-1.5 shrink-0">
    {@render action('mdi:email-open-multiple-outline', $_('chat.markAllLowRead'), markAllRead)}
    {@render action('mdi:archive-arrow-down-outline', $_('chat.archiveAllLow'), archiveAll)}
  </div>
</div>
