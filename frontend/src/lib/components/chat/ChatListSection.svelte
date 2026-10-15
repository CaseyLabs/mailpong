<script lang="ts">
  // ChatListSection — one group of the chat list: its header (see
  // ChatGroupHeader) and a ChatRow per chat. ChatList owns selection and
  // triage; the section only wires each row to them.
  import ChatRow from './ChatRow.svelte'
  import ChatGroupHeader from './ChatGroupHeader.svelte'
  import type { ChatItem } from '$lib/stores/chat.svelte'
  import type { ChatRowActions } from './chatTriage'

  interface Props {
    label: string
    items: ChatItem[]
    open: boolean
    // Absent for a group without a header (it can't collapse).
    onToggle?: () => void
    fewerLabel?: string
    onToggleFewer: () => void
    inPinned?: boolean
    selectedKey: string | null
    folderType: string
    showAccount: boolean
    myEmails: Set<string>
    // Row refs by chat key, for the list's keyboard actions.
    rowRefs: Record<string, ChatRow | null>
    rowId: (c: ChatItem) => string
    rowActions: (c: ChatItem) => ChatRowActions
    onOpen: (c: ChatItem) => void
    onFocusList: () => void
    onActionComplete: (autoSelectNext?: boolean) => void
    onReply?: (mode: 'reply' | 'reply-all' | 'forward', messageId: string) => void
  }

  let {
    label, items, open, onToggle, fewerLabel, onToggleFewer, inPinned = false, selectedKey, folderType, showAccount, myEmails,
    rowRefs = $bindable(), rowId, rowActions, onOpen, onFocusList, onActionComplete, onReply,
  }: Props = $props()
</script>

<ChatGroupHeader {label} {open} {onToggle} {fewerLabel} {onToggleFewer}>
  {#each items as c (c.key)}
    <ChatRow
      bind:this={rowRefs[c.key]}
      chat={c}
      id={rowId(c)}
      selected={selectedKey === c.key}
      {folderType}
      {showAccount}
      {inPinned}
      {myEmails}
      onSelect={() => onOpen(c)}
      actions={rowActions(c)}
      {onFocusList}
      {onActionComplete}
      {onReply}
    />
  {/each}
</ChatGroupHeader>
