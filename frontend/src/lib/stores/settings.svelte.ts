// Runes-based settings store
// Provides reactive state for application settings

// @ts-ignore - wailsjs path
import { GetMessageListDensity, GetMessageListSortOrder, GetThemeMode, GetShowTitleBar, GetRunBackground, GetStartHidden, GetAutostart, GetLanguage, GetComposerMode, GetMailtoMode, GetComposerFormat, GetNativeTitleBar, GetAlwaysLoadImages, GetDarkMailContent, GetDarkComposerBody, GetAccentBarUnread, GetAccentUnreadStyle, GetShowMessageListCircles, GetShowMessageListProfilePics, GetAlwaysShowMessageCheckbox, GetShowMessagePreview, GetShowViewerCircles, GetSpellcheckEnabled, GetSpellcheckLanguages, GetSpellcheckCustomWords, GetChatSendKey, GetChatAutoAdvance, GetChatShowLowGroup, GetChatReadingWidth, GetChatDisplayStyle } from '../../../wailsjs/go/app/App'
import { setLocale as setI18nLocale, resolveLocale } from '$lib/i18n'
import { filterSupportedDicts } from '$lib/spellcheck/locales'
import { loadDateFnsLocale, getDateFnsLocale } from '$lib/i18n/dateFnsLocale'
import type { Locale } from 'date-fns'

export type ComposerMode = 'inline' | 'detached'
export type ComposerFormat = 'rich' | 'plain'
export type ChatSendKey = 'enter' | 'ctrl-enter'
export type ChatReadingWidth = 'narrow' | 'standard' | 'wide'
export type ChatDisplayStyle = 'bubbles' | 'flat'
export type ChatAutoAdvance = 'next' | 'previous'
export type MessageListDensity = 'micro' | 'compact' | 'standard' | 'large'
export type MessageListSortOrder = 'newest' | 'oldest'
export type ThemeMode = 'system' | 'light' | 'dark'

// Mirrors the default in internal/settings/store.go.
export const DEFAULT_THEME: ThemeMode = 'dark'

// Module-level reactive state
let messageListDensity = $state<MessageListDensity>('standard')
let messageListSortOrder = $state<MessageListSortOrder>('newest')
let themeMode = $state<ThemeMode>(DEFAULT_THEME)
let showTitleBar = $state<boolean>(true)
let runBackground = $state<boolean>(false)
let startHidden = $state<boolean>(false)
let autostart = $state<boolean>(false)
let spellcheckEnabled = $state<boolean>(true)
let chatSendKey = $state<ChatSendKey>('ctrl-enter')
let chatReadingWidth = $state<ChatReadingWidth>('standard')
let chatDisplayStyle = $state<ChatDisplayStyle>('bubbles')
let chatAutoAdvance = $state<ChatAutoAdvance>('next')
let chatShowLowGroup = $state(true)
let spellcheckLanguages = $state<string[]>([])
let spellcheckCustomWords = $state<string[]>([])
let language = $state<string>('')
let composerMode = $state<ComposerMode>('inline')
let mailtoMode = $state<ComposerMode>('inline')
let composerFormat = $state<ComposerFormat>('rich')
let nativeTitleBar = $state<boolean>(true)
let alwaysLoadImages = $state<boolean>(false)
let darkMailContent = $state<boolean>(true)
let darkComposerBody = $state<boolean>(true)
let accentBarUnread = $state<boolean>(false)
let accentUnreadStyle = $state<string>('dot')
let showMessageListCircles = $state<boolean>(true)
let showMessageListProfilePics = $state<boolean>(false)
let alwaysShowMessageCheckbox = $state<boolean>(false)
let showMessagePreview = $state<boolean>(false)
let showViewerCircles = $state<boolean>(true)

// Getter functions to access the state
export function getMessageListDensity(): MessageListDensity {
  return messageListDensity
}

export function getMessageListSortOrder(): MessageListSortOrder {
  return messageListSortOrder
}

export function getThemeMode(): ThemeMode {
  return themeMode
}

export function getShowTitleBar(): boolean {
  return showTitleBar
}

export function getRunBackground(): boolean {
  return runBackground
}

export function getStartHidden(): boolean {
  return startHidden
}

export function getAutostart(): boolean {
  return autostart
}

export function getChatSendKey(): ChatSendKey {
  return chatSendKey
}

export function setChatSendKey(v: ChatSendKey) {
  chatSendKey = v
}

export function getChatDisplayStyle(): ChatDisplayStyle {
  return chatDisplayStyle
}

export function setChatDisplayStyle(v: ChatDisplayStyle) {
  chatDisplayStyle = v
}

export function getChatReadingWidth(): ChatReadingWidth {
  return chatReadingWidth
}

// Stores the width and applies it: app.css maps data-reading-width on <html>
// to the chat column and stream widths.
export function setChatReadingWidth(v: ChatReadingWidth) {
  chatReadingWidth = v
  document.documentElement.dataset.readingWidth = v
}

export function getChatAutoAdvance(): ChatAutoAdvance {
  return chatAutoAdvance
}

export function setChatAutoAdvance(v: ChatAutoAdvance) {
  chatAutoAdvance = v
}

export function getChatShowLowGroup(): boolean {
  return chatShowLowGroup
}

export function setChatShowLowGroup(v: boolean) {
  chatShowLowGroup = v
}

export function getSpellcheckEnabled(): boolean {
  return spellcheckEnabled
}

export function getSpellcheckLanguages(): string[] {
  return spellcheckLanguages
}

export function getSpellcheckCustomWords(): string[] {
  return spellcheckCustomWords
}

export function getLanguage(): string {
  return language
}

export function getComposerMode(): ComposerMode {
  return composerMode
}

export function getMailtoMode(): ComposerMode {
  return mailtoMode
}

export function getComposerFormat(): ComposerFormat {
  return composerFormat
}

export function getNativeTitleBar(): boolean {
  return nativeTitleBar
}

export function getAlwaysLoadImages(): boolean {
  return alwaysLoadImages
}

export function getDarkMailContent(): boolean {
  return darkMailContent
}

export function getDarkComposerBody(): boolean {
  return darkComposerBody
}

export function getAccentBarUnread(): boolean {
  return accentBarUnread
}

export function getAccentUnreadStyle(): string {
  return accentUnreadStyle
}

export function getShowMessageListCircles(): boolean {
  return showMessageListCircles
}

export function getShowMessageListProfilePics(): boolean {
  return showMessageListProfilePics
}

export function getAlwaysShowMessageCheckbox(): boolean {
  return alwaysShowMessageCheckbox
}

export function getShowMessagePreview(): boolean {
  return showMessagePreview
}

export function getShowViewerCircles(): boolean {
  return showViewerCircles
}

export function getCurrentDateFnsLocale(): Locale | undefined {
  return getDateFnsLocale(language || 'en')
}

// Setter functions to update the state
export function setMessageListDensity(density: MessageListDensity) {
  messageListDensity = density
}

export function setMessageListSortOrder(sortOrder: MessageListSortOrder) {
  messageListSortOrder = sortOrder
}

export function setThemeMode(mode: ThemeMode) {
  themeMode = mode
}

export function setShowTitleBar(show: boolean) {
  showTitleBar = show
}

export function setRunBackground(v: boolean) {
  runBackground = v
}

export function setStartHidden(v: boolean) {
  startHidden = v
}

export function setAutostart(v: boolean) {
  autostart = v
}

export function setSpellcheckEnabled(v: boolean) {
  spellcheckEnabled = v
}

export function setSpellcheckLanguages(v: string[]) {
  spellcheckLanguages = v
}

export function setSpellcheckCustomWords(v: string[]) {
  spellcheckCustomWords = v
}

export function setLanguage(lang: string) {
  language = lang ? resolveLocale(lang) : lang
  if (lang) {
    setI18nLocale(lang)
    loadDateFnsLocale(lang)
  }
}

export function setComposerMode(mode: ComposerMode) {
  composerMode = mode
}

export function setMailtoMode(mode: ComposerMode) {
  mailtoMode = mode
}

export function setComposerFormat(format: ComposerFormat) {
  composerFormat = format
}

export function setNativeTitleBar(v: boolean) {
  nativeTitleBar = v
}

export function setAlwaysLoadImages(v: boolean) {
  alwaysLoadImages = v
}

export function setDarkMailContent(v: boolean) {
  darkMailContent = v
}

export function setDarkComposerBody(v: boolean) {
  darkComposerBody = v
}

export function setAccentBarUnread(v: boolean) {
  accentBarUnread = v
}

export function setAccentUnreadStyle(v: string) {
  accentUnreadStyle = v
}

export function setShowMessageListCircles(v: boolean) {
  showMessageListCircles = v
}

export function setShowMessageListProfilePics(v: boolean) {
  showMessageListProfilePics = v
}

export function setAlwaysShowMessageCheckbox(v: boolean) {
  alwaysShowMessageCheckbox = v
}

export function setShowMessagePreview(v: boolean) {
  showMessagePreview = v
}

export function setShowViewerCircles(v: boolean) {
  showViewerCircles = v
}

// Load settings from backend (call on app startup)
export async function loadSettings(): Promise<ThemeMode> {
  try {
    const [density, sortOrder, theme, titleBar, runBg, startHid, autoSt, lang, compMode, mailMode, compFormat, nativeTB, alwaysImages, darkMail, darkComposer, accentBar, accentStyle, listCircles, listProfilePics, alwaysCheckbox, messagePreview, viewerCircles, scEnabled, scLangs, scWords, sendKey, autoAdvance, showLowGroup, readingWidth, displayStyle] = await Promise.all([
      GetMessageListDensity(),
      GetMessageListSortOrder(),
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
      GetSpellcheckCustomWords(),
      GetChatSendKey(),
      GetChatAutoAdvance(),
      GetChatShowLowGroup(),
      GetChatReadingWidth(),
      GetChatDisplayStyle(),
    ])
    messageListDensity = (density as MessageListDensity) || 'standard'
    messageListSortOrder = (sortOrder as MessageListSortOrder) || 'newest'
    themeMode = (theme as ThemeMode) || DEFAULT_THEME
    showTitleBar = titleBar ?? true // Default to true
    runBackground = runBg ?? false
    startHidden = startHid ?? false
    autostart = autoSt ?? false
    composerMode = (compMode as ComposerMode) || 'inline'
    mailtoMode = (mailMode as ComposerMode) || 'inline'
    composerFormat = (compFormat as ComposerFormat) || 'rich'
    nativeTitleBar = nativeTB ?? true
    alwaysLoadImages = alwaysImages ?? false
    darkMailContent = darkMail ?? true
    darkComposerBody = darkComposer ?? true
    accentBarUnread = accentBar ?? false
    accentUnreadStyle = accentStyle || 'dot'
    showMessageListCircles = listCircles ?? true
    showMessageListProfilePics = listProfilePics ?? false
    alwaysShowMessageCheckbox = alwaysCheckbox ?? false
    showMessagePreview = messagePreview ?? false
    showViewerCircles = viewerCircles ?? true
    spellcheckEnabled = scEnabled ?? true
    spellcheckLanguages = filterSupportedDicts(scLangs ?? [])
    spellcheckCustomWords = scWords ?? []
    chatSendKey = sendKey === 'enter' ? 'enter' : 'ctrl-enter'
    chatAutoAdvance = autoAdvance === 'previous' ? 'previous' : 'next'
    chatShowLowGroup = showLowGroup ?? true
    setChatDisplayStyle(displayStyle === 'flat' ? 'flat' : 'bubbles')
    setChatReadingWidth(readingWidth === 'narrow' || readingWidth === 'wide' ? readingWidth : 'standard')
    // Apply saved language (if set, overrides system detection from initI18n)
    if (lang) {
      language = resolveLocale(lang)
      setI18nLocale(language)
      await loadDateFnsLocale(language)
    }
    return themeMode
  } catch (err) {
    console.error('Failed to load settings:', err)
    return DEFAULT_THEME
  }
}
