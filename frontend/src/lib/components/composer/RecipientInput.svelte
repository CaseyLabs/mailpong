<script module lang="ts">
  // Module-scope counter — gives each RecipientInput instance a unique ID so
  // drag-and-drop can tell intra-field reorder from cross-field move without
  // any shared store.
  let nextInstanceId = 0
</script>

<script lang="ts">
  import { getContext } from 'svelte'
  import Icon from '@iconify/svelte'
  // @ts-ignore - Wails generated imports
  import { smtp, contact } from '../../../../wailsjs/go/models'
  import { type ComposerApi, COMPOSER_API_KEY, createMainWindowApi } from '$lib/composerApi'

  interface Props {
    recipients: smtp.Address[]
    placeholder?: string
    /** Optional: search contacts function override */
    searchContactsFn?: (query: string, limit: number) => Promise<contact.Contact[]>
  }

  let { recipients = $bindable([]), placeholder = 'Add recipients...', searchContactsFn }: Props = $props()

  nextInstanceId += 1
  const instanceId = nextInstanceId

  // Get API from context or create default
  const contextApi = getContext<ComposerApi | undefined>(COMPOSER_API_KEY)
  const api: ComposerApi = contextApi || createMainWindowApi()

  // Use the prop function or fall back to API (evaluated each call to handle prop changes)
  function doSearchContacts(query: string, limit: number) {
    return searchContactsFn ? searchContactsFn(query, limit) : api.searchContacts(query, limit)
  }

  // State
  let inputValue = $state('')
  let suggestions = $state<contact.Contact[]>([])
  let showSuggestions = $state(false)
  let selectedIndex = $state(-1)
  let inputElement: HTMLInputElement
  let containerElement: HTMLDivElement
  let debounceTimer: ReturnType<typeof setTimeout> | null = null

  // Search contacts as user types
  async function searchContacts(query: string) {
    if (query.length < 2) {
      suggestions = []
      showSuggestions = false
      return
    }

    try {
      const results = await doSearchContacts(query, 10)
      suggestions = results || []
      showSuggestions = suggestions.length > 0
      selectedIndex = -1
    } catch (err) {
      console.error('Failed to search contacts:', err)
      suggestions = []
    }
  }

  function handleInput() {
    // Debounce the search
    if (debounceTimer) {
      clearTimeout(debounceTimer)
    }
    debounceTimer = setTimeout(() => {
      searchContacts(inputValue)
    }, 200)
  }

  function handleKeyDown(e: KeyboardEvent) {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      if (showSuggestions && selectedIndex < suggestions.length - 1) {
        selectedIndex++
      }
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      if (showSuggestions && selectedIndex > 0) {
        selectedIndex--
      }
    } else if (e.key === 'Enter') {
      e.preventDefault()
      if (showSuggestions && selectedIndex >= 0) {
        selectSuggestion(suggestions[selectedIndex])
      } else if (inputValue.trim()) {
        addRecipient(inputValue.trim())
      }
    } else if (e.key === 'Escape') {
      showSuggestions = false
      selectedIndex = -1
    } else if (e.key === 'Backspace' && inputValue === '' && recipients.length > 0) {
      // Remove last recipient
      removeRecipient(recipients.length - 1)
    } else if (e.key === ',' || e.key === ';' || e.key === 'Tab') {
      if (inputValue.trim()) {
        e.preventDefault()
        addRecipient(inputValue.trim())
      }
    }
  }

  function selectSuggestion(contact: contact.Contact) {
    const address = new smtp.Address({
      name: contact.display_name || '',
      address: contact.email,
    })
    recipients = [...recipients, address]
    inputValue = ''
    suggestions = []
    showSuggestions = false
    selectedIndex = -1
    inputElement?.focus()
  }

  function addRecipient(value: string) {
    // Parse email address (handle "Name <email@example.com>" format)
    const emailRegex = /^(?:(.+?)\s*<)?([^\s<>]+@[^\s<>]+)>?$/
    const match = value.match(emailRegex)

    if (match) {
      const name = match[1]?.trim() || ''
      const email = match[2].toLowerCase()

      // Check if already added (handle both 'address' and 'email' field names)
      if (recipients.some(r => (r.address || (r as any).email || '').toLowerCase() === email)) {
        inputValue = ''
        return
      }

      const address = new smtp.Address({
        name: name,
        address: email,
      })
      recipients = [...recipients, address]
      inputValue = ''
      suggestions = []
      showSuggestions = false
    }
  }

  function removeRecipient(index: number) {
    recipients = recipients.filter((_, i) => i !== index)
    inputElement?.focus()
  }

  // ─── Drag-and-drop: reorder within field, move between To/Cc/Bcc fields ───

  const DND_MIME = 'application/x-mailven-recipient'

  let draggingIndex = $state<number | null>(null)
  let dropTargetIndex = $state<number | null>(null)

  function handleChipDragStart(e: DragEvent, index: number) {
    if (!e.dataTransfer) return
    e.dataTransfer.setData(DND_MIME, JSON.stringify({
      sourceId: instanceId,
      recipient: recipients[index],
    }))
    e.dataTransfer.effectAllowed = 'move'
    draggingIndex = index
  }

  function handleChipDragEnd(e: DragEvent) {
    // Source removes its chip only if a cross-field move actually happened.
    // Intra-field reorder clears draggingIndex inside handleDrop so this skips.
    // dropEffect is 'none' for cancelled drops.
    if (e.dataTransfer?.dropEffect === 'move' && draggingIndex !== null) {
      removeRecipient(draggingIndex)
    }
    draggingIndex = null
    dropTargetIndex = null
  }

  function hasDndPayload(e: DragEvent): boolean {
    return !!e.dataTransfer?.types.includes(DND_MIME)
  }

  function handleDragEnter(e: DragEvent, targetIndex: number) {
    if (!hasDndPayload(e)) return
    e.preventDefault()
    dropTargetIndex = targetIndex
  }

  function handleDragOver(e: DragEvent, targetIndex: number) {
    if (!hasDndPayload(e)) return
    e.preventDefault()  // required to allow drop
    if (e.dataTransfer) e.dataTransfer.dropEffect = 'move'
    dropTargetIndex = targetIndex
  }

  function handleDragLeave() {
    dropTargetIndex = null
  }

  function handleDrop(e: DragEvent, targetIndex: number) {
    const raw = e.dataTransfer?.getData(DND_MIME)
    if (!raw) {
      dropTargetIndex = null
      return
    }

    let payload: { sourceId: number; recipient: smtp.Address }
    try {
      payload = JSON.parse(raw)
    } catch {
      dropTargetIndex = null
      return
    }

    if (payload.sourceId === instanceId) {
      // Intra-field reorder: splice from source index to target index.
      e.preventDefault()
      const from = draggingIndex
      if (from === null || from === targetIndex || from + 1 === targetIndex) {
        // No move (dropping on self or immediately after self)
        draggingIndex = null
        dropTargetIndex = null
        return
      }
      const next = [...recipients]
      const [moved] = next.splice(from, 1)
      const adjusted = from < targetIndex ? targetIndex - 1 : targetIndex
      next.splice(adjusted, 0, moved)
      recipients = next
      // Clear draggingIndex so handleChipDragEnd skips removal (move already done).
      draggingIndex = null
      dropTargetIndex = null
      return
    }

    // Cross-field move: append to destination via the existing addRecipient
    // pipeline (parse, dedup, clear). Source's handleChipDragEnd will remove
    // from source on dragend.
    e.preventDefault()
    const r = payload.recipient
    const name = r?.name?.trim()
    const address = (r?.address || '').trim()
    if (!address) {
      dropTargetIndex = null
      return
    }
    addRecipient(name ? `${name} <${address}>` : address)
    dropTargetIndex = null
  }

  function handleBlur() {
    // Delay hiding to allow click on suggestion (mousedown-based selection
    // runs before blur and clears inputValue, so the auto-commit below becomes
    // a no-op for that path).
    setTimeout(() => {
      showSuggestions = false
      // Auto-commit typed text on blur so the user doesn't have to press
      // Tab/Enter to turn a typed address into a chip. Invalid input is left
      // in the field for the user to fix (addRecipient is a no-op on regex
      // miss).
      if (inputValue.trim()) {
        addRecipient(inputValue.trim())
      }
    }, 200)
  }

  // Allow parent to focus the input programmatically
  export function focus() {
    inputElement?.focus()
  }

  function handleFocus() {
    if (inputValue.length >= 2 && suggestions.length > 0) {
      showSuggestions = true
    }
  }

  function handlePaste(e: ClipboardEvent) {
    const text = e.clipboardData?.getData('text')
    if (text) {
      // Handle pasted email addresses (comma or semicolon separated)
      const addresses = text.split(/[,;]/).map(a => a.trim()).filter(Boolean)
      if (addresses.length > 1) {
        e.preventDefault()
        addresses.forEach(addRecipient)
      }
    }
  }
</script>

<div bind:this={containerElement} class="relative">
  <div class="flex flex-wrap items-center gap-1">
    <!-- Recipient chips -->
    {#each recipients as recipient, index (recipient.address + ':' + index)}
      <div
        role="listitem"
        draggable="true"
        ondragstart={(e) => handleChipDragStart(e, index)}
        ondragend={handleChipDragEnd}
        ondragenter={(e) => handleDragEnter(e, index)}
        ondragover={(e) => handleDragOver(e, index)}
        ondragleave={handleDragLeave}
        ondrop={(e) => handleDrop(e, index)}
        class="flex items-center gap-1 px-2 py-0.5 bg-muted rounded-md text-sm transition-opacity cursor-grab {draggingIndex === index ? 'opacity-50' : ''} {dropTargetIndex === index ? 'border-l-2 border-primary -ml-0.5 pl-[7px]' : ''}"
      >
        <span>
          {#if recipient.name}
            {recipient.name}
          {:else}
            {recipient.address || (recipient as any).email || ''}
          {/if}
        </span>
        <button
          onclick={() => removeRecipient(index)}
          class="text-muted-foreground hover:text-foreground"
          type="button"
        >
          <Icon icon="mdi:close" class="w-3.5 h-3.5" />
        </button>
      </div>
    {/each}

    <!-- Input — also a drop target for "insert at end" -->
    <input
      bind:this={inputElement}
      bind:value={inputValue}
      oninput={handleInput}
      onkeydown={handleKeyDown}
      onblur={handleBlur}
      onfocus={handleFocus}
      onpaste={handlePaste}
      ondragenter={(e) => handleDragEnter(e, recipients.length)}
      ondragover={(e) => handleDragOver(e, recipients.length)}
      ondragleave={handleDragLeave}
      ondrop={(e) => handleDrop(e, recipients.length)}
      type="email"
      {placeholder}
      class="flex-1 min-w-[150px] bg-transparent text-sm focus:outline-hidden {dropTargetIndex === recipients.length ? 'border-l-2 border-primary' : ''}"
    />
  </div>

  <!-- Suggestions dropdown -->
  {#if showSuggestions}
    <div class="absolute left-0 right-0 top-full mt-1 bg-popover border border-border rounded-md shadow-lg z-50 max-h-60 overflow-auto">
      {#each suggestions as suggestion, index (suggestion.email + ':' + index)}
        <button
          onmousedown={() => selectSuggestion(suggestion)}
          class="w-full px-3 py-2 text-left hover:bg-muted transition-colors flex items-center gap-3 {index === selectedIndex ? 'bg-muted' : ''}"
          type="button"
        >
          <!-- Avatar placeholder -->
          <div class="w-8 h-8 rounded-full bg-primary/10 flex items-center justify-center text-xs font-medium text-primary">
            {(suggestion.display_name || suggestion.email)[0].toUpperCase()}
          </div>
          <div class="flex-1 min-w-0">
            <div class="text-sm font-medium truncate">
              {suggestion.display_name || suggestion.email}
            </div>
            {#if suggestion.display_name}
              <div class="text-xs text-muted-foreground truncate">
                {suggestion.email}
              </div>
            {/if}
          </div>
          {#if suggestion.send_count > 0}
            <div class="text-xs text-muted-foreground">
              {suggestion.send_count}x
            </div>
          {/if}
        </button>
      {/each}
    </div>
  {/if}
</div>
