<script lang="ts">
  import { onMount } from 'svelte'
  import Icon from '@iconify/svelte'
  import * as Dialog from '$lib/components/ui/dialog'
  import * as Tabs from '$lib/components/ui/tabs'
  import { Button } from '$lib/components/ui/button'
  // @ts-ignore - wailsjs path
  import { GetReadReceiptResponsePolicy, SetReadReceiptResponsePolicy, GetMarkAsReadDelay, SetMarkAsReadDelay, GetMessageListDensity, SetMessageListDensity, GetThemeMode, SetThemeMode, GetShowTitleBar, SetShowTitleBar, GetRunBackground, SetRunBackground, GetStartHidden, SetStartHidden, GetAutostart, SetAutostart, GetLanguage, SetLanguage, GetComposerMode, SetComposerMode, GetMailtoMode, SetMailtoMode, GetComposerFormat, SetComposerFormat, GetNativeTitleBar, SetNativeTitleBar, GetAlwaysLoadImages, SetAlwaysLoadImages, GetDarkMailContent, SetDarkMailContent, GetDarkComposerBody, SetDarkComposerBody, GetAccentBarUnread, SetAccentBarUnread, GetAccentUnreadStyle, SetAccentUnreadStyle, GetShowMessageListCircles, SetShowMessageListCircles, GetShowMessageListProfilePics, SetShowMessageListProfilePics, GetAlwaysShowMessageCheckbox, SetAlwaysShowMessageCheckbox, GetShowMessagePreview, SetShowMessagePreview, GetShowViewerCircles, SetShowViewerCircles, GetSpellcheckEnabled, SetSpellcheckEnabled, GetSpellcheckLanguages, SetSpellcheckLanguages, GetChatSendKey, SetChatSendKey, GetChatIncludeQuote, SetChatIncludeQuote, GetChatAutoAdvance, SetChatAutoAdvance, GetChatNotifyPriorityOnly, SetChatNotifyPriorityOnly, GetChatShowLowGroup, SetChatShowLowGroup, GetChatCombineSenders, SetChatCombineSenders, QuitApp } from '../../../../wailsjs/go/app/App.js'
  import { addToast } from '$lib/stores/toast'
  import { setMessageListDensity as updateDensityStore, setThemeMode as updateThemeStore, setShowTitleBar as updateShowTitleBarStore, setRunBackground as updateRunBackgroundStore, setStartHidden as updateStartHiddenStore, setAutostart as updateAutostartStore, setLanguage as updateLanguageStore, setComposerMode as updateComposerModeStore, setMailtoMode as updateMailtoModeStore, setComposerFormat as updateComposerFormatStore, setNativeTitleBar as updateNativeTitleBarStore, setAlwaysLoadImages as updateAlwaysLoadImagesStore, setDarkMailContent as updateDarkMailContentStore, setDarkComposerBody as updateDarkComposerBodyStore, setAccentBarUnread as updateAccentBarUnreadStore, setAccentUnreadStyle as updateAccentUnreadStyleStore, setShowMessageListCircles as updateShowMessageListCirclesStore, setShowMessageListProfilePics as updateShowMessageListProfilePicsStore, setAlwaysShowMessageCheckbox as updateAlwaysShowMessageCheckboxStore, setShowMessagePreview as updateShowMessagePreviewStore, setShowViewerCircles as updateShowViewerCirclesStore, setSpellcheckEnabled as updateSpellcheckEnabledStore, setSpellcheckLanguages as updateSpellcheckLanguagesStore, setChatSendKey as updateChatSendKeyStore, setChatAutoAdvance as updateChatAutoAdvanceStore, setChatShowLowGroup as updateChatShowLowGroupStore, type ChatSendKey, type ChatAutoAdvance, type MessageListDensity, type ThemeMode, type ComposerMode, type ComposerFormat, DEFAULT_THEME } from '$lib/stores/settings.svelte'
  import { syncSpellcheckLanguagesIfActive, defaultSpellcheckLanguages } from '$lib/spellcheck/settings'
  import { applyThemeFromMode } from '$lib/stores/theme.svelte'
  import { dialogGuardOpen, dialogGuardClose } from '$lib/stores/dialogGuard'
  import { _ } from '$lib/i18n'
  import ConfirmDialog from '$lib/components/ui/confirm-dialog/ConfirmDialog.svelte'
  import GeneralTab from './GeneralTab.svelte'
  import ComposerTab from './ComposerTab.svelte'
  import ImagesTab from './ImagesTab.svelte'
  import ChatTab from './ChatTab.svelte'
  import AccountsTab from './AccountsTab.svelte'
  import ContactsTab from './ContactsTab.svelte'
  import ExtensionsTab from './ExtensionsTab.svelte'
  import AboutTab from './AboutTab.svelte'

  interface Props {
    /** Whether the dialog is open */
    open?: boolean
    /** Callback when dialog should close */
    onClose?: () => void
  }

  let {
    open = $bindable(false),
    onClose,
  }: Props = $props()

  // Settings state
  let readReceiptResponsePolicy = $state<string>('ask')
  let markAsReadDelaySeconds = $state<number>(1) // Display in seconds, store in ms
  let messageListDensity = $state<string>('standard')
  let themeMode = $state<string>(DEFAULT_THEME)
  let showTitleBar = $state<boolean>(true)
  let runBackground = $state<boolean>(false)
  let startHidden = $state<boolean>(false)
  let autostart = $state<boolean>(false)
  let language = $state<string>('')
  let composerMode = $state<string>('inline')
  let mailtoMode = $state<string>('inline')
  let composerFormat = $state<string>('rich')
  let spellcheckEnabled = $state<boolean>(true)
  let spellcheckLanguages = $state<string[]>([])
  let nativeTitleBar = $state<boolean>(false)
  let alwaysLoadImages = $state<boolean>(false)
  let darkMailContent = $state<boolean>(false)
  let darkComposerBody = $state<boolean>(false)
  let accentBarUnread = $state<boolean>(false)
  let accentUnreadStyle = $state<string>('dot')
  let showMessageListCircles = $state<boolean>(true)
  let showMessageListProfilePics = $state<boolean>(false)
  let alwaysShowMessageCheckbox = $state<boolean>(false)
  let showMessagePreview = $state<boolean>(false)
  let showViewerCircles = $state<boolean>(true)
  let chatSendKey = $state<string>('enter')
  let chatIncludeQuote = $state<boolean>(true)
  let chatAutoAdvance = $state<string>('next')
  let chatNotifyPriorityOnly = $state<boolean>(true)
  let chatShowLowGroup = $state<boolean>(true)
  let chatCombineSenders = $state<boolean>(true)
  let originalNativeTitleBar = false
  // Snapshot of the saved theme at dialog open time. Used to revert live preview
  // if the dialog closes without Save (Cancel / ESC / click-outside).
  let originalThemeMode = ''
  let hasSaved = $state(false)
  let showRestartDialog = $state(false)
  let loading = $state(true)
  let saving = $state(false)
  let activeTab = $state('general')
  let tabScrollEl = $state<HTMLDivElement | null>(null)

  // All tabs share one scroll container; start each tab at the top.
  $effect(() => {
    void activeTab
    if (tabScrollEl) tabScrollEl.scrollTop = 0
  })

  // Live theme preview: apply the picker's current value to the document
  // immediately so the user sees what each theme looks like before saving.
  // The revert path is in handleOpenChange when the dialog closes unsaved.
  $effect(() => {
    if (loading || !themeMode) return
    applyThemeFromMode(themeMode as ThemeMode)
  })

  // Load settings on mount
  onMount(async () => {
    await loadSettings()
  })

  // Also load when dialog opens
  $effect(() => {
    if (open) {
      loadSettings()
    }
  })

  // Activate the dialog guard while open: suppresses background refreshes
  // and routes global keyboard shortcuts (e.g. Ctrl+A) to the dialog inputs
  // instead of the message list / viewer behind it.
  $effect(() => {
    if (open) {
      dialogGuardOpen()
      return () => dialogGuardClose()
    }
  })

  async function loadSettings() {
    loading = true
    hasSaved = false
    try {
      const [policy, delayMs, density, theme, titleBar, runBg, startHid, autoSt, lang, comp, mail, compFmt, nativeTB, alwaysImages, darkMail, darkComposer, accentBar, accentStyle, listCircles, listProfilePics, alwaysCheckbox, messagePreview, viewerCircles, scEnabled, scLangs, sendKey, includeQuote, autoAdvance, notifyPriority, showLow, combineSenders] = await Promise.all([
        GetReadReceiptResponsePolicy(),
        GetMarkAsReadDelay(),
        GetMessageListDensity(),
        GetThemeMode(),
        GetShowTitleBar(),
        GetRunBackground(),
        GetStartHidden(),
        GetAutostart(),
        GetLanguage(),
        GetComposerMode(),
        GetMailtoMode(),
        GetComposerFormat(),
        GetNativeTitleBar(),
        GetAlwaysLoadImages(),
        GetDarkMailContent(),
        GetDarkComposerBody(),
        GetAccentBarUnread(),
        GetAccentUnreadStyle(),
        GetShowMessageListCircles(),
        GetShowMessageListProfilePics(),
        GetAlwaysShowMessageCheckbox(),
        GetShowMessagePreview(),
        GetShowViewerCircles(),
        GetSpellcheckEnabled(),
        GetSpellcheckLanguages(),
        GetChatSendKey(),
        GetChatIncludeQuote(),
        GetChatAutoAdvance(),
        GetChatNotifyPriorityOnly(),
        GetChatShowLowGroup(),
        GetChatCombineSenders(),
      ])
      readReceiptResponsePolicy = policy
      // Convert ms to seconds for display
      markAsReadDelaySeconds = delayMs < 0 ? -1 : delayMs / 1000
      messageListDensity = density
      themeMode = theme
      originalThemeMode = theme
      showTitleBar = titleBar
      runBackground = runBg
      startHidden = startHid
      autostart = autoSt
      language = lang
      composerMode = comp || 'inline'
      mailtoMode = mail || 'inline'
      composerFormat = compFmt || 'rich'
      spellcheckEnabled = scEnabled ?? true
      // Empty stored list = "use defaults" — show the same set the engine uses.
      spellcheckLanguages = (scLangs && scLangs.length) ? scLangs : defaultSpellcheckLanguages()
      nativeTitleBar = nativeTB ?? false
      alwaysLoadImages = alwaysImages ?? false
      darkMailContent = darkMail ?? false
      darkComposerBody = darkComposer ?? false
      accentBarUnread = accentBar ?? false
      accentUnreadStyle = accentStyle || 'dot'
      showMessageListCircles = listCircles ?? true
      showMessageListProfilePics = listProfilePics ?? false
      alwaysShowMessageCheckbox = alwaysCheckbox ?? false
      showMessagePreview = messagePreview ?? false
      showViewerCircles = viewerCircles ?? true
      chatSendKey = sendKey || 'enter'
      chatIncludeQuote = includeQuote ?? true
      chatAutoAdvance = autoAdvance || 'next'
      chatNotifyPriorityOnly = notifyPriority ?? true
      chatShowLowGroup = showLow ?? true
      chatCombineSenders = combineSenders ?? true
      originalNativeTitleBar = nativeTitleBar
    } catch (err) {
      console.error('Failed to load settings:', err)
    } finally {
      loading = false
    }
  }

  async function handleSave() {
    saving = true
    try {
      // Convert seconds to ms for storage
      const delayMs = markAsReadDelaySeconds < 0 ? -1 : Math.round(markAsReadDelaySeconds * 1000)

      // Save settings sequentially to avoid SQLite lock conflicts
      await SetReadReceiptResponsePolicy(readReceiptResponsePolicy)
      await SetMarkAsReadDelay(delayMs)
      await SetMessageListDensity(messageListDensity)
      await SetThemeMode(themeMode)
      await SetShowTitleBar(showTitleBar)
      await SetRunBackground(runBackground)
      await SetStartHidden(startHidden)
      await SetAutostart(autostart)
      if (language) {
        await SetLanguage(language)
      }
      await SetComposerMode(composerMode)
      await SetMailtoMode(mailtoMode)
      await SetComposerFormat(composerFormat)
      await SetSpellcheckEnabled(spellcheckEnabled)
      await SetSpellcheckLanguages(spellcheckLanguages)
      await SetNativeTitleBar(nativeTitleBar)
      await SetAlwaysLoadImages(alwaysLoadImages)
      await SetDarkMailContent(darkMailContent)
      await SetDarkComposerBody(darkComposerBody)
      await SetAccentBarUnread(accentBarUnread)
      await SetAccentUnreadStyle(accentUnreadStyle)
      await SetShowMessageListCircles(showMessageListCircles)
      await SetShowMessageListProfilePics(showMessageListProfilePics)
      await SetAlwaysShowMessageCheckbox(alwaysShowMessageCheckbox)
      await SetShowMessagePreview(showMessagePreview)
      await SetShowViewerCircles(showViewerCircles)
      await SetChatSendKey(chatSendKey)
      await SetChatIncludeQuote(chatIncludeQuote)
      await SetChatAutoAdvance(chatAutoAdvance)
      await SetChatNotifyPriorityOnly(chatNotifyPriorityOnly)
      await SetChatShowLowGroup(chatShowLowGroup)
      await SetChatCombineSenders(chatCombineSenders)
      // Update the reactive stores so UI updates immediately
      updateDensityStore(messageListDensity as MessageListDensity)
      updateThemeStore(themeMode as ThemeMode)
      updateShowTitleBarStore(showTitleBar)
      updateRunBackgroundStore(runBackground)
      updateStartHiddenStore(startHidden)
      updateAutostartStore(autostart)
      if (language) {
        updateLanguageStore(language)
      }
      updateComposerModeStore(composerMode as ComposerMode)
      updateMailtoModeStore(mailtoMode as ComposerMode)
      updateComposerFormatStore(composerFormat as ComposerFormat)
      updateSpellcheckEnabledStore(spellcheckEnabled)
      updateSpellcheckLanguagesStore(spellcheckLanguages)
      syncSpellcheckLanguagesIfActive() // live-apply to an open composer; stays lazy otherwise
      updateNativeTitleBarStore(nativeTitleBar)
      updateAlwaysLoadImagesStore(alwaysLoadImages)
      updateDarkMailContentStore(darkMailContent)
      updateDarkComposerBodyStore(darkComposerBody)
      updateAccentBarUnreadStore(accentBarUnread)
      updateAccentUnreadStyleStore(accentUnreadStyle)
      updateShowMessageListCirclesStore(showMessageListCircles)
      updateShowMessageListProfilePicsStore(showMessageListProfilePics)
      updateAlwaysShowMessageCheckboxStore(alwaysShowMessageCheckbox)
      updateShowMessagePreviewStore(showMessagePreview)
      updateShowViewerCirclesStore(showViewerCircles)
      updateChatSendKeyStore(chatSendKey as ChatSendKey)
      updateChatAutoAdvanceStore(chatAutoAdvance as ChatAutoAdvance)
      updateChatShowLowGroupStore(chatShowLowGroup)
      addToast({
        type: 'success',
        message: $_('toast.settingsSaved'),
      })
      hasSaved = true
      originalThemeMode = themeMode
      // Show restart dialog if native title bar setting changed
      if (nativeTitleBar !== originalNativeTitleBar) {
        originalNativeTitleBar = nativeTitleBar
        showRestartDialog = true
        return
      }
      open = false
      onClose?.()
    } catch (err) {
      console.error('Failed to save settings:', err)
      addToast({
        type: 'error',
        message: $_('toast.failedToSaveSettings'),
      })
    } finally {
      saving = false
    }
  }

  function revertLivePreview() {
    if (!hasSaved && originalThemeMode && themeMode !== originalThemeMode) {
      applyThemeFromMode(originalThemeMode as ThemeMode)
    }
  }

  function handleCancel() {
    revertLivePreview()
    open = false
    onClose?.()
  }

  function handleOpenChange(isOpen: boolean) {
    open = isOpen
    if (!isOpen) {
      revertLivePreview()
      onClose?.()
    }
  }
</script>

<Dialog.Root bind:open onOpenChange={handleOpenChange}>
  <Dialog.Content class="max-w-3xl" preventCloseAutoFocus onInteractOutside={(e) => e.preventDefault()}>
    <Dialog.Header>
      <Dialog.Title>{$_('settings.title')}</Dialog.Title>
      <Dialog.Description>
        {$_('settings.description')}
      </Dialog.Description>
    </Dialog.Header>

    {#if loading}
      <div class="flex items-center justify-center py-8">
        <Icon icon="mdi:loading" class="w-6 h-6 animate-spin text-muted-foreground" />
      </div>
    {:else}
      <Tabs.Root bind:value={activeTab} class="w-full">
        <Tabs.List class="grid w-full grid-cols-8">
          <Tabs.Trigger value="general" class="flex items-center gap-2">
            <span class="inline-flex w-4 h-4 items-center justify-center shrink-0"><Icon icon="lucide:settings-2" width="16" height="16" /></span>
            {$_('settings.general')}
          </Tabs.Trigger>
          <Tabs.Trigger value="composer" class="flex items-center gap-2">
            <span class="inline-flex w-4 h-4 items-center justify-center shrink-0"><Icon icon="lucide:square-pen" width="46" height="46" /></span>
            {$_('settings.composer')}
          </Tabs.Trigger>
          <Tabs.Trigger value="chat" class="flex items-center gap-2">
            <span class="inline-flex w-4 h-4 items-center justify-center shrink-0"><Icon icon="lucide:messages-square" width="16" height="16" /></span>
            {$_('settings.chat')}
          </Tabs.Trigger>
          <Tabs.Trigger value="images" class="flex items-center gap-2">
            <span class="inline-flex w-4 h-4 items-center justify-center shrink-0"><Icon icon="lucide:image" width="16" height="16" /></span>
            {$_('settings.images')}
          </Tabs.Trigger>
          <Tabs.Trigger value="accounts" class="flex items-center gap-2">
            <span class="inline-flex w-4 h-4 items-center justify-center shrink-0"><Icon icon="lucide:mails" width="16" height="16" /></span>
            {$_('settings.accounts')}
          </Tabs.Trigger>
          <Tabs.Trigger value="contacts" class="flex items-center gap-2">
            <span class="inline-flex w-4 h-4 items-center justify-center shrink-0"><Icon icon="lucide:contact" width="16" height="16" /></span>
            {$_('settings.contacts')}
          </Tabs.Trigger>
          <Tabs.Trigger value="extensions" class="flex items-center gap-2">
            <span class="inline-flex w-4 h-4 items-center justify-center shrink-0"><Icon icon="lucide:puzzle" width="16" height="16" /></span>
            {$_('settings.extensions')}
          </Tabs.Trigger>
          <Tabs.Trigger value="about" class="flex items-center gap-2">
            <span class="inline-flex w-4 h-4 items-center justify-center shrink-0"><Icon icon="lucide:info" width="16" height="16" /></span>
            {$_('settings.about')}
          </Tabs.Trigger>
        </Tabs.List>

        <div bind:this={tabScrollEl} class="mt-4 h-[350px] overflow-y-auto pl-1 pr-3">
          <Tabs.Content value="general" class="mt-0">
            <GeneralTab
              bind:markAsReadDelaySeconds
              bind:messageListDensity
              bind:themeMode
              bind:nativeTitleBar
              bind:showTitleBar
              bind:runBackground
              bind:startHidden
              bind:autostart
              bind:language
              onDelayChange={(v) => markAsReadDelaySeconds = v}
              onDensityChange={(v) => messageListDensity = v}
              onThemeChange={(v) => themeMode = v}
              onTitleBarChange={(ntb, stb) => { nativeTitleBar = ntb; showTitleBar = stb }}
              onRunBackgroundChange={(v) => { runBackground = v; if (!v) startHidden = false }}
              onStartHiddenChange={(v) => { startHidden = v; if (v) runBackground = true }}
              onAutostartChange={(v) => autostart = v}
              onLanguageChange={(v) => language = v}
              bind:accentBarUnread
              bind:accentUnreadStyle
              bind:showMessageListCircles
              bind:showMessageListProfilePics
              bind:alwaysShowMessageCheckbox
              bind:showMessagePreview
              bind:showViewerCircles
              bind:darkMailContent
              bind:darkComposerBody
            />
          </Tabs.Content>

          <Tabs.Content value="composer" class="mt-0">
            <ComposerTab
              bind:composerMode
              bind:mailtoMode
              bind:composerFormat
              bind:readReceiptResponsePolicy
              bind:spellcheckEnabled
              bind:spellcheckLanguages
              onComposerModeChange={(v) => { composerMode = v; if (v === 'detached') mailtoMode = 'detached' }}
              onMailtoModeChange={(v) => mailtoMode = v}
              onFormatChange={(v) => composerFormat = v}
              onPolicyChange={(v) => readReceiptResponsePolicy = v}
              onSpellcheckEnabledChange={(v) => spellcheckEnabled = v}
              onSpellcheckLanguagesChange={(v) => spellcheckLanguages = v}
            />
          </Tabs.Content>

          <Tabs.Content value="chat" class="mt-0">
            <ChatTab
              bind:sendKey={chatSendKey}
              bind:includeQuote={chatIncludeQuote}
              bind:autoAdvance={chatAutoAdvance}
              bind:notifyPriorityOnly={chatNotifyPriorityOnly}
              bind:showLowGroup={chatShowLowGroup}
              bind:combineSenders={chatCombineSenders}
            />
          </Tabs.Content>

          <Tabs.Content value="images" class="mt-0">
            <ImagesTab
              bind:alwaysLoadImages
              onAlwaysLoadImagesChange={(v) => alwaysLoadImages = v}
            />
          </Tabs.Content>

          <Tabs.Content value="accounts" class="mt-0">
            <AccountsTab />
          </Tabs.Content>

          <Tabs.Content value="contacts" class="mt-0">
            <ContactsTab />
          </Tabs.Content>

          <Tabs.Content value="extensions" class="mt-0">
            <ExtensionsTab />
          </Tabs.Content>

          <Tabs.Content value="about" class="mt-0">
            <AboutTab />
          </Tabs.Content>
        </div>
      </Tabs.Root>

      <!-- Actions - show Save/Cancel on the tabs with dialog-saved settings -->
      {#if ['general', 'composer', 'chat', 'images'].includes(activeTab)}
        <div class="flex items-center justify-end gap-2 pt-4 border-t border-border">
          <Button variant="ghost" onclick={handleCancel} disabled={saving}>
            {$_('common.cancel')}
          </Button>
          <Button onclick={handleSave} disabled={saving}>
            {#if saving}
              <Icon icon="mdi:loading" class="w-4 h-4 mr-2 animate-spin" />
            {/if}
            {$_('common.save')}
          </Button>
        </div>
      {:else}
        <div class="flex items-center justify-end gap-2 pt-4 border-t border-border">
          <Button variant="ghost" onclick={handleCancel}>
            {$_('common.close')}
          </Button>
        </div>
      {/if}
    {/if}
  </Dialog.Content>
</Dialog.Root>

<ConfirmDialog
  bind:open={showRestartDialog}
  title={$_('settingsGeneral.restartRequired')}
  description={$_('settingsGeneral.restartRequiredDescription')}
  confirmLabel={$_('settingsGeneral.quitNow')}
  cancelLabel={$_('settingsGeneral.restartLater')}
  onConfirm={() => QuitApp()}
  onCancel={() => { showRestartDialog = false; open = false; onClose?.() }}
/>
