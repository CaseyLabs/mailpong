<script lang="ts">
  import { onMount, onDestroy, getContext, setContext } from 'svelte'
  import Icon from '@iconify/svelte'
  // @ts-ignore - Wails generated imports
  import { smtp } from '../../../../wailsjs/go/models'
  // @ts-ignore - Wails runtime for events
  import { EventsOff } from '../../../../wailsjs/runtime/runtime.js'
  import { type ComposerApi, COMPOSER_API_KEY, createMainWindowApi } from '$lib/composerApi'
  import { _ } from '$lib/i18n'
  import { ComposerState } from './composerState.svelte'
  import { registerComposerEffects } from './composerEffects.svelte'
  import { initComposer } from './composerInit'
  import { handleDragOver, handleDragLeave, handleDrop } from './composerAttachments'
  import { handleKeyDown } from './composerKeys'
  import ComposerHeader from './ComposerHeader.svelte'
  import ComposerRecipientFields from './ComposerRecipientFields.svelte'
  import ComposerSecurityBars from './ComposerSecurityBars.svelte'
  import ComposerBody from './ComposerBody.svelte'
  import ComposerDialogs from './ComposerDialogs.svelte'

  // Props
  interface Props {
    accountId: string
    /** Pre-populated message from backend (for reply/forward), or null for new message */
    initialMessage?: smtp.ComposeMessage | null
    /** Existing draft ID if editing a draft */
    draftId?: string | null
    /** Original message ID for reply/forward (needed for pop-out) */
    messageId?: string | null
    onClose?: () => void
    onSent?: () => void
    /** Optional API override - if not provided, uses context or creates main window API */
    api?: ComposerApi
    /** Whether this composer is in a detached window (hides pop-out button) */
    isDetached?: boolean
    /** Signal from parent (detached window) to trigger close flow */
    closeRequested?: boolean
    /** Callback when close request has been handled */
    onCloseHandled?: () => void
    /** Callback when recipient or subject changes (for dynamic window title) */
    onTitleChange?: (to: string, subject: string) => void
    /** Whether remote images were loaded in the viewer before reply/forward */
    imagesLoaded?: boolean
    /** Main window: Undo on a queued send hands the message back (with the draft holding it) */
    onUndoSend?: (accountId: string, message: smtp.ComposeMessage, draftId: string) => void
  }

  let { accountId, initialMessage = null, draftId = null, messageId = null, onClose, onSent, api: propApi, isDetached = false, closeRequested = false, onCloseHandled, onTitleChange, imagesLoaded = false, onUndoSend }: Props = $props()

  // Get API from context, props, or create default main window API
  const contextApi = getContext<ComposerApi | undefined>(COMPOSER_API_KEY)
  const defaultApi = createMainWindowApi()
  // Resolve once at init — the API never changes after mount
  // svelte-ignore state_referenced_locally
  const resolvedApi: ComposerApi = propApi || contextApi || defaultApi
  // Use $derived so propApi changes are detected (even though it typically doesn't change after mount)
  const api: ComposerApi = $derived(propApi || contextApi || defaultApi)

  // Propagate the resolved API to child components (e.g. RecipientInput)
  // so they can access it via getContext instead of falling back to the main window API
  setContext(COMPOSER_API_KEY, resolvedApi)

  // The composer's state and the props it reads, shared with the composer*.ts modules
  const s = new ComposerState({
    get accountId() { return accountId },
    get initialMessage() { return initialMessage },
    get draftId() { return draftId },
    get messageId() { return messageId },
    get onClose() { return onClose },
    get onSent() { return onSent },
    get api() { return api },
    get isDetached() { return isDetached },
    get closeRequested() { return closeRequested },
    get onCloseHandled() { return onCloseHandled },
    get onTitleChange() { return onTitleChange },
    get imagesLoaded() { return imagesLoaded },
    get onUndoSend() { return onUndoSend },
  })

  registerComposerEffects(s)

  // Initialize
  onMount(() => initComposer(s))

  onDestroy(() => {
    // Unsubscribe from draft sync events
    EventsOff('draft:syncStatusChanged')
    // Clear any pending save timeout
    if (s.saveTimeoutId) {
      clearTimeout(s.saveTimeoutId)
    }
    s.editor?.destroy()
  })

  /** Sends a message counting down now; resolves when the send is done. */
  export function flushSend(): Promise<void> {
    s.countdown.sendNow()
    return s.sendDone
  }
</script>

<svelte:window on:keydown={(e) => handleKeyDown(s, e)} />

<div
  class="flex flex-col h-full bg-background relative"
  class:ring-2={s.isDraggingOver}
  class:ring-primary={s.isDraggingOver}
  class:ring-inset={s.isDraggingOver}
  ondragover={(e) => handleDragOver(s, e)}
  ondragleave={(e) => handleDragLeave(s, e)}
  ondrop={(e) => handleDrop(s, e)}
  role="region"
  aria-label={$_('aria.emailComposer')}
>
  <ComposerHeader {s} />

  <!-- Compose form -->
  <div class="flex-1 flex flex-col min-h-0 overflow-hidden">
    <ComposerRecipientFields {s} />
    <ComposerSecurityBars {s} />
    <ComposerBody {s} />
  </div>

  <!-- Drag overlay -->
  {#if s.isDraggingOver}
    <div class="absolute inset-0 bg-primary/10 flex items-center justify-center pointer-events-none z-10">
      <div class="bg-background border-2 border-dashed border-primary rounded-lg px-8 py-6 text-center">
        <Icon icon="mdi:attachment" class="w-12 h-12 text-primary mx-auto mb-2" />
        <p class="text-lg font-medium">{$_('composer.dropToAttach')}</p>
      </div>
    </div>
  {/if}
</div>

<ComposerDialogs {s} />

<style>
  /* Zero-margin paragraphs so Enter looks like a single line break */
  :global(.composer-editor p) {
    margin: 0;
    line-height: 1.25;
  }

  :global(.ProseMirror p.is-editor-empty:first-child::before) {
    color: #adb5bd;
    content: attr(data-placeholder);
    float: left;
    height: 0;
    pointer-events: none;
  }

  /* Table styling for composer */
  :global(.composer-editor table) {
    border-collapse: collapse;
    margin: 0;
    overflow: hidden;
    table-layout: fixed;
    width: 100%;
  }

  :global(.composer-editor td),
  :global(.composer-editor th) {
    border: 1px solid hsl(var(--border));
    box-sizing: border-box;
    min-width: 1em;
    padding: 6px 8px;
    position: relative;
    vertical-align: top;
  }

  :global(.composer-editor th) {
    background-color: hsl(var(--muted));
    font-weight: 600;
  }
</style>
