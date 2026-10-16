<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import Icon from '@iconify/svelte'
  import type { Editor } from '@tiptap/core'
  import { _ } from '$lib/i18n'
  import SignatureColorPicker from './SignatureColorPicker.svelte'
  import SignatureFontSizePicker from './SignatureFontSizePicker.svelte'
  import { createSignatureEditor } from './signatureEditorSetup'
  import { fontSizes, presetColors, formatHtml, readFileAsDataUrl } from './signatureEditorHelpers'

  interface Props {
    /** HTML content of the signature */
    value?: string
    /** Placeholder text when empty */
    placeholder?: string
    /** Callback when content changes */
    onchange?: (html: string) => void
  }

  let { value = '', placeholder = 'Enter your signature...', onchange }: Props = $props()

  let editorElement: HTMLElement | undefined = $state()
  let editor: Editor | null = null
  let isUpdatingFromProp = false

  // Track active formatting states - updated via transaction listener for performance
  let activeStates = $state({
    bold: false,
    italic: false,
    underline: false,
    strike: false,
    link: false,
  })

  // Current text color
  let currentColor = $state<string | null>(null)
  let showColorPicker = $state(false)

  // Current text alignment
  let currentAlign = $state<'left' | 'center' | 'right'>('left')

  // Table context
  let isInTable = $state(false)

  // Raw HTML mode
  let rawHtmlMode = $state(false)
  let rawHtmlContent = $state('')

  // Current font size
  let currentFontSize = $state<string>('')
  let showFontSizePicker = $state(false)

  // Update active states from editor
  function updateActiveStates() {
    if (!editor) return
    activeStates = {
      bold: editor.isActive('bold'),
      italic: editor.isActive('italic'),
      underline: editor.isActive('underline'),
      strike: editor.isActive('strike'),
      link: editor.isActive('link'),
    }
    // Get current text color
    const colorAttr = editor.getAttributes('textStyle').color
    currentColor = colorAttr || null
    // Get current text alignment
    if (editor.isActive({ textAlign: 'center' })) {
      currentAlign = 'center'
    } else if (editor.isActive({ textAlign: 'right' })) {
      currentAlign = 'right'
    } else {
      currentAlign = 'left'
    }
    // Get current font size
    const fontSizeAttr = editor.getAttributes('textStyle').fontSize
    currentFontSize = fontSizeAttr || ''
    // Check if cursor is inside a table
    isInTable = editor.isActive('table')
  }

  onMount(() => {
    editor = createSignatureEditor({
      element: editorElement,
      placeholder,
      content: value,
      onImageFile: handleImageFile,
      onUpdate: () => {
        if (!isUpdatingFromProp) {
          onchange?.(editor?.getHTML() || '')
        }
      },
      onTransaction: () => {
        // Update toolbar button states on selection/content changes
        updateActiveStates()
      },
    })

    // Initial state update
    updateActiveStates()
  })

  onDestroy(() => {
    editor?.destroy()
  })

  // Update editor when value prop changes externally
  $effect(() => {
    if (editor && !rawHtmlMode && value !== editor.getHTML()) {
      isUpdatingFromProp = true
      editor.commands.setContent(value, { emitUpdate: false })
      isUpdatingFromProp = false
    }
  })

  // Handle image file (paste or drop)
  async function handleImageFile(file: File) {
    try {
      const dataUrl = await readFileAsDataUrl(file)
      editor?.chain().focus().setImage({ src: dataUrl }).run()
    } catch (err) {
      console.error('Failed to insert image:', err)
    }
  }

  // Toolbar actions
  function toggleBold() {
    editor?.chain().focus().toggleBold().run()
  }

  function toggleItalic() {
    editor?.chain().focus().toggleItalic().run()
  }

  function toggleUnderline() {
    editor?.chain().focus().toggleUnderline().run()
  }

  function toggleStrike() {
    editor?.chain().focus().toggleStrike().run()
  }

  function insertLink() {
    const url = prompt('Enter URL:')
    if (url) {
      editor?.chain().focus().setLink({ href: url }).run()
    }
  }

  function removeLink() {
    editor?.chain().focus().unsetLink().run()
  }

  function insertImageUrl() {
    const url = prompt('Enter image URL:')
    if (url) {
      editor?.chain().focus().setImage({ src: url }).run()
    }
  }

  function insertImageFile() {
    const input = document.createElement('input')
    input.type = 'file'
    input.accept = 'image/*'
    input.onchange = async (e) => {
      const file = (e.target as HTMLInputElement).files?.[0]
      if (file) {
        await handleImageFile(file)
      }
    }
    input.click()
  }

  // Color functions
  function setColor(color: string) {
    editor?.chain().focus().setColor(color).run()
    showColorPicker = false
  }

  function removeColor() {
    editor?.chain().focus().unsetColor().run()
    showColorPicker = false
  }

  function handleCustomColor(event: Event) {
    const input = event.target as HTMLInputElement
    setColor(input.value)
  }

  function toggleColorPicker() {
    showColorPicker = !showColorPicker
    showFontSizePicker = false  // Close font size picker if open
  }

  // Close pickers when clicking outside
  function handleClickOutside(event: MouseEvent) {
    const target = event.target as HTMLElement
    if (!target.closest('.color-picker-container')) {
      showColorPicker = false
    }
    if (!target.closest('.font-size-picker-container')) {
      showFontSizePicker = false
    }
  }

  // Font size functions
  function setFontSize(size: string) {
    editor?.chain().focus().setFontSize(size).run()
    showFontSizePicker = false
  }

  function toggleFontSizePicker() {
    showFontSizePicker = !showFontSizePicker
    showColorPicker = false  // Close color picker if open
  }

  // Alignment functions
  function setAlign(align: 'left' | 'center' | 'right') {
    editor?.chain().focus().setTextAlign(align).run()
  }

  // Table functions
  function insertTable() {
    editor?.chain().focus().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run()
  }

  function deleteTable() {
    editor?.chain().focus().deleteTable().run()
  }

  // Raw HTML toggle
  function toggleRawHtml() {
    if (!rawHtmlMode) {
      // Switching to raw HTML mode - use value prop (preserves custom HTML attributes
      // that TipTap strips) rather than editor.getHTML() which normalizes away unknowns
      rawHtmlContent = formatHtml(value || '')
      rawHtmlMode = true
      return
    }
    // Switching back to WYSIWYG mode
    isUpdatingFromProp = true
    editor?.commands.setContent(rawHtmlContent, { emitUpdate: false })
    isUpdatingFromProp = false
    rawHtmlMode = false
    onchange?.(editor?.getHTML() || '')
  }

  function handleRawHtmlInput(event: Event) {
    rawHtmlContent = (event.target as HTMLTextAreaElement).value
    onchange?.(rawHtmlContent)
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<div class="border border-input rounded-md overflow-hidden bg-background" role="toolbar" aria-label={$_('aria.signatureEditor')} tabindex="-1" onclick={handleClickOutside}>
  <!-- Toolbar -->
  <div class="flex flex-wrap items-center gap-0.5 px-2 py-1.5 border-b border-border bg-muted/30">
    {#if !rawHtmlMode}
      <button
        type="button"
        onclick={toggleBold}
        class="p-1.5 rounded hover:bg-muted transition-colors"
        class:bg-muted={activeStates.bold}
        title={$_('editor.bold')}
      >
        <Icon icon="mdi:format-bold" class="w-4 h-4" />
      </button>
      <button
        type="button"
        onclick={toggleItalic}
        class="p-1.5 rounded hover:bg-muted transition-colors"
        class:bg-muted={activeStates.italic}
        title={$_('editor.italic')}
      >
        <Icon icon="mdi:format-italic" class="w-4 h-4" />
      </button>
      <button
        type="button"
        onclick={toggleUnderline}
        class="p-1.5 rounded hover:bg-muted transition-colors"
        class:bg-muted={activeStates.underline}
        title={$_('editor.underline')}
      >
        <Icon icon="mdi:format-underline" class="w-4 h-4" />
      </button>
      <button
        type="button"
        onclick={toggleStrike}
        class="p-1.5 rounded hover:bg-muted transition-colors"
        class:bg-muted={activeStates.strike}
        title={$_('editor.strikethrough')}
      >
        <Icon icon="mdi:format-strikethrough" class="w-4 h-4" />
      </button>

      <SignatureColorPicker
        open={showColorPicker}
        {currentColor}
        {presetColors}
        onToggle={toggleColorPicker}
        onSetColor={setColor}
        onCustomColor={handleCustomColor}
        onRemoveColor={removeColor}
      />

      <SignatureFontSizePicker
        open={showFontSizePicker}
        {currentFontSize}
        {fontSizes}
        onToggle={toggleFontSizePicker}
        onSetSize={setFontSize}
      />

      <div class="w-px h-4 bg-border mx-1"></div>

      <!-- Alignment buttons -->
      <button
        type="button"
        onclick={() => setAlign('left')}
        class="p-1.5 rounded hover:bg-muted transition-colors"
        class:bg-muted={currentAlign === 'left'}
        title={$_('editor.alignLeft')}
      >
        <Icon icon="mdi:format-align-left" class="w-4 h-4" />
      </button>
      <button
        type="button"
        onclick={() => setAlign('center')}
        class="p-1.5 rounded hover:bg-muted transition-colors"
        class:bg-muted={currentAlign === 'center'}
        title={$_('editor.alignCenter')}
      >
        <Icon icon="mdi:format-align-center" class="w-4 h-4" />
      </button>
      <button
        type="button"
        onclick={() => setAlign('right')}
        class="p-1.5 rounded hover:bg-muted transition-colors"
        class:bg-muted={currentAlign === 'right'}
        title={$_('editor.alignRight')}
      >
        <Icon icon="mdi:format-align-right" class="w-4 h-4" />
      </button>

      <!-- Table -->
      <div class="w-px h-4 bg-border mx-1"></div>

      <button
        type="button"
        onclick={insertTable}
        class="p-1.5 rounded hover:bg-muted transition-colors"
        title={$_('editor.insertTable')}
      >
        <Icon icon="mdi:table" class="w-4 h-4" />
      </button>
      {#if isInTable}
        <button
          type="button"
          onclick={deleteTable}
          class="p-1.5 rounded hover:bg-muted transition-colors text-destructive"
          title={$_('editor.deleteTable')}
        >
          <Icon icon="mdi:table-remove" class="w-4 h-4" />
        </button>
      {/if}

      <div class="w-px h-4 bg-border mx-1"></div>

      <button
        type="button"
        onclick={insertLink}
        class="p-1.5 rounded hover:bg-muted transition-colors"
        class:bg-muted={activeStates.link}
        title={$_('editor.insertLink')}
      >
        <Icon icon="mdi:link" class="w-4 h-4" />
      </button>
      {#if activeStates.link}
        <button
          type="button"
          onclick={removeLink}
          class="p-1.5 rounded hover:bg-muted transition-colors text-destructive"
          title={$_('editor.removeLink')}
        >
          <Icon icon="mdi:link-off" class="w-4 h-4" />
        </button>
      {/if}

      <div class="w-px h-4 bg-border mx-1"></div>

      <button
        type="button"
        onclick={insertImageUrl}
        class="p-1.5 rounded hover:bg-muted transition-colors"
        title={$_('editor.insertImageUrl')}
      >
        <Icon icon="mdi:image-outline" class="w-4 h-4" />
      </button>
      <button
        type="button"
        onclick={insertImageFile}
        class="p-1.5 rounded hover:bg-muted transition-colors"
        title={$_('editor.insertImageFile')}
      >
        <Icon icon="mdi:image-plus" class="w-4 h-4" />
      </button>
    {/if}

    <!-- HTML toggle (always visible, pushed to far right) -->
    <div class="flex-1"></div>
    <button
      type="button"
      onclick={toggleRawHtml}
      class="p-1.5 rounded hover:bg-muted transition-colors"
      class:bg-muted={rawHtmlMode}
      title={rawHtmlMode ? $_('editor.visualMode') : $_('editor.htmlMode')}
    >
      <Icon icon={rawHtmlMode ? 'mdi:eye' : 'mdi:code-tags'} class="w-4 h-4" />
    </button>
  </div>

  <!-- Editor -->
  {#if rawHtmlMode}
    <textarea
      class="w-full min-h-[100px] p-3 font-mono text-sm bg-background text-foreground border-none outline-hidden resize-y"
      value={rawHtmlContent}
      oninput={handleRawHtmlInput}
      spellcheck="false"
    ></textarea>
  {:else}
    <div bind:this={editorElement} class="min-h-[100px]"></div>
  {/if}
</div>

<style>
  :global(.ProseMirror p.is-editor-empty:first-child::before) {
    color: #adb5bd;
    content: attr(data-placeholder);
    float: left;
    height: 0;
    pointer-events: none;
  }

  /* Table styling */
  :global(.ProseMirror table) {
    border-collapse: collapse;
    margin: 0;
    overflow: hidden;
    table-layout: fixed;
  }

  :global(.ProseMirror td),
  :global(.ProseMirror th) {
    border: 1px solid hsl(var(--border));
    box-sizing: border-box;
    min-width: 1em;
    padding: 6px 8px;
    position: relative;
    vertical-align: top;
  }

  :global(.ProseMirror th) {
    background-color: hsl(var(--muted));
    font-weight: 600;
  }
</style>
