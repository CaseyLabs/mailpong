<script lang="ts">
  // Remote sources section of CalendarSettingsDialog: per-source rename,
  // sync interval, reauthorize, sync now, organizer email and colors.
  import { _ } from 'svelte-i18n'
  import * as Select from '$lib/components/ui/select'
  import { Button } from '$lib/components/ui/button'
  import { Input } from '$lib/components/ui/input'
  import ColorPicker from '$lib/components/kit/ColorPicker.svelte'
  import Icon from '@iconify/svelte'
  import { toasts } from '$lib/stores/toast'
  import { calendarSources } from '$extensions/calendar/frontend/stores/calendarSources.svelte'
  // @ts-ignore - wailsjs bindings
  import { Calendar_SetSyncInterval, Calendar_SetOrganizerIdentity, Calendar_ReprobeCalDAVOrganizerIdentities, Calendar_RenameSource } from '$wailsjs/go/app/App.js'
  import GrantCalendarAccessButton from './GrantCalendarAccessButton.svelte'
  // @ts-ignore - wailsjs bindings
  import type { backend } from '$wailsjs/go/models'

  interface Props {
    expanded: boolean
    ontoggle: () => void
    emailByAccountId: Record<string, string>
    forceTarget: backend.Source | null
    forceSyncing: boolean
    onForce: (src: backend.Source) => void
    onDelete: (src: backend.Source) => void
    onAddCalDAV: () => void
    onAddGoogle: () => void
    onAddMicrosoft: () => void
  }

  let {
    expanded,
    ontoggle,
    emailByAccountId,
    forceTarget,
    forceSyncing,
    onForce,
    onDelete,
    onAddCalDAV,
    onAddGoogle,
    onAddMicrosoft,
  }: Props = $props()

  // Remote sources — every non-local source surfaces in the "Sources"
  // section. Local calendars have their own section above. Per-source
  // controls (sync interval, sync now, delete) apply uniformly; only the
  // CalDAV-specific organizer-email row is gated on type in the template.
  const remoteSources = $derived(
    calendarSources.sources.filter(s => s.type !== 'local')
  )

  // Per-row state — per-source spinner for Sync Now.
  let syncingSourceID = $state<string | null>(null)

  // Per-source organizer-email edits. Keyed by source.id; populated when
  // the user starts typing so we don't track every CalDAV row in memory
  // up front. Saving applies + clears the local entry.
  let organizerEdits = $state<Record<string, string>>({})
  let reprobingSourceID = $state<string | null>(null)

  // Per-source rename state. `renamingId` is the source currently in
  // edit mode (only one at a time); `renameDraft` holds the in-progress
  // text. Esc cancels; Enter / blur commits.
  let renamingId = $state<string | null>(null)
  let renameDraft = $state('')

  function startRename(src: backend.Source) {
    renamingId = src.id
    renameDraft = src.name
  }

  function cancelRename() {
    renamingId = null
    renameDraft = ''
  }

  async function commitRename(src: backend.Source) {
    const next = renameDraft.trim()
    if (next === '' || next === src.name) {
      cancelRename()
      return
    }
    try {
      await Calendar_RenameSource(src.id, next)
      await calendarSources.load()
      toasts.success($_('calendar.settings.renameToastSuccess', { values: { name: next } }))
    } catch (err) {
      const msg = (err as Error)?.message ?? String(err)
      toasts.error(msg)
    } finally {
      cancelRename()
    }
  }

  function organizerDisplay(src: backend.Source): string {
    if (organizerEdits[src.id] !== undefined) return organizerEdits[src.id]
    const list = src.organizerIdentities ?? []
    return list[0] ?? ''
  }

  function organizerLooksValid(value: string): boolean {
    return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value.trim())
  }

  async function handleSaveOrganizer(src: backend.Source) {
    const value = (organizerEdits[src.id] ?? '').trim()
    if (value !== '' && !organizerLooksValid(value)) {
      toasts.error($_('calendar.add.organizerEmailInvalid'))
      return
    }
    try {
      await Calendar_SetOrganizerIdentity(src.id, value)
      toasts.success($_('calendar.settings.organizerEmailSaved'))
      delete organizerEdits[src.id]
      organizerEdits = { ...organizerEdits }
      await calendarSources.load()
    } catch (err) {
      const msg = (err as Error)?.message ?? String(err)
      toasts.error(msg)
    }
  }

  async function handleReprobeOrganizer(src: backend.Source) {
    if (reprobingSourceID !== null) return
    reprobingSourceID = src.id
    try {
      const count = await Calendar_ReprobeCalDAVOrganizerIdentities(src.id)
      if (count > 0) {
        toasts.success(
          $_('calendar.settings.organizerEmailReprobed', { values: { count } }),
        )
        delete organizerEdits[src.id]
        organizerEdits = { ...organizerEdits }
        await calendarSources.load()
        return
      }
      toasts.warning($_('calendar.settings.organizerEmailReprobeEmpty'))
      await calendarSources.load()
    } catch (err) {
      const msg = (err as Error)?.message ?? String(err)
      toasts.error(msg)
    } finally {
      reprobingSourceID = null
    }
  }

  // Sync interval options (mirrors api.go's validSyncIntervals).
  const INTERVAL_VALUES = [5, 15, 30, 60, 120, 240, 720]

  function formatIntervalLabel(n: number): string {
    if (n < 60) return $_('calendar.settings.intervalMinutes', { values: { n } })
    return $_('calendar.settings.intervalHours', { values: { n: n / 60 } })
  }

  // Relative last-sync time. Uses chosen tz only for label fidelity; the
  // relative-time logic is tz-agnostic.
  function lastSyncLabel(src: backend.Source): string {
    if (!src.lastSyncedAt || src.lastSyncedAt === 0) {
      return $_('calendar.settings.lastSyncNever')
    }
    const elapsed = Math.floor(Date.now() / 1000) - src.lastSyncedAt
    let when: string
    if (elapsed < 60) when = 'just now'
    else if (elapsed < 3600) when = `${Math.floor(elapsed / 60)}m`
    else if (elapsed < 86400) when = `${Math.floor(elapsed / 3600)}h`
    else when = `${Math.floor(elapsed / 86400)}d`
    return $_('calendar.settings.lastSync', { values: { time: when } })
  }

  function calendarCountLabel(src: backend.Source): string {
    const n = calendarSources.calendarsBySource[src.id]?.length ?? 0
    if (n === 1) return $_('calendar.settings.calendarCountOne')
    return $_('calendar.settings.calendarCount', { values: { count: n } })
  }

  async function handleIntervalChange(source: backend.Source, value: string) {
    const minutes = Number(value)
    if (!Number.isFinite(minutes) || minutes <= 0) return
    try {
      await Calendar_SetSyncInterval(source.id, minutes)
      toasts.success($_('calendar.settings.intervalToastSuccess', { values: { name: source.name } }))
      void calendarSources.load()
    } catch (err) {
      const msg = (err as Error)?.message ?? String(err)
      toasts.error($_('calendar.settings.intervalToastError', { values: { message: msg } }))
    }
  }

  async function handleSyncNow(source: backend.Source) {
    if (syncingSourceID !== null) return
    syncingSourceID = source.id
    try {
      await calendarSources.syncSource(source.id)
    } finally {
      syncingSourceID = null
    }
  }

  // Reauthorize applies to every OAuth (Google/Microsoft) source: regardless
  // of which client creds the slot uses, the token lives in the calendar slot
  // and is refreshed by re-running this consent. CalDAV sources use passwords.
  function showReauth(src: backend.Source): boolean {
    switch (src.type) {
      case 'google':
      case 'microsoft':
        return true
    }
    return false
  }

  // The currently-bound interval per source is a derived map so each
  // Select.Root has a stable bound value.
  function currentInterval(src: backend.Source): string {
    const m = src.syncIntervalMin ?? 15
    if (!INTERVAL_VALUES.includes(m)) return '15'
    return String(m)
  }
</script>

  <!-- Sources section — CalDAV + Google + Microsoft sources. Local
       calendars live in their own section above. -->
  <section class="border border-border rounded-md">
    <button
      type="button"
      class="w-full flex items-center justify-between p-3 text-left hover:bg-muted/30 rounded-md"
      onclick={() => ontoggle()}
      aria-expanded={expanded}
    >
      <h3 class="text-sm font-semibold text-foreground">
        {$_('calendar.settings.sourcesSection')}
      </h3>
      <Icon icon={expanded ? 'mdi:chevron-up' : 'mdi:chevron-down'} class="w-4 h-4 text-muted-foreground" />
    </button>
    {#if expanded}
    <div class="space-y-2 px-3 pb-3 pt-1">

    {#if remoteSources.length === 0}
      <p class="text-sm text-muted-foreground py-3">
        {$_('calendar.settings.noSources')}
      </p>
    {/if}

    {#each remoteSources as src (src.id)}
      <div class="flex flex-col gap-2 p-3 border border-border rounded-md">
        <!-- Source header row -->
        <div class="flex items-start justify-between gap-3">
          <div class="flex-1 min-w-0">
            {#if renamingId === src.id}
              <div class="flex items-center gap-1">
                <Input
                  type="text"
                  class="h-8 text-sm flex-1 min-w-0"
                  bind:value={renameDraft}
                  autofocus
                  onkeydown={(e) => {
                    if (e.key === 'Enter') { e.preventDefault(); void commitRename(src) }
                    if (e.key === 'Escape') { e.preventDefault(); cancelRename() }
                  }}
                  onblur={() => void commitRename(src)}
                />
              </div>
            {/if}
            {#if renamingId !== src.id}
              <div class="flex items-center gap-1 min-w-0">
                <span class="text-sm font-medium text-foreground truncate">{src.name}</span>
                <Button
                  variant="ghost"
                  size="sm"
                  class="h-6 w-6 p-0 shrink-0"
                  onclick={() => startRename(src)}
                  title={$_('calendar.settings.renameTitle')}
                  aria-label={$_('calendar.settings.renameTitle')}
                >
                  <Icon icon="mdi:pencil-outline" class="w-3.5 h-3.5" />
                </Button>
              </div>
            {/if}
          </div>

          <div class="flex items-center gap-2 shrink-0">
            <Select.Root
              value={currentInterval(src)}
              onValueChange={(v) => { if (v) void handleIntervalChange(src, v) }}
            >
              <Select.Trigger class="h-8 w-32 text-xs">
                {formatIntervalLabel(Number(currentInterval(src)))}
              </Select.Trigger>
              <Select.Content>
                {#each INTERVAL_VALUES as v (v)}
                  <Select.Item value={String(v)} label={formatIntervalLabel(v)} />
                {/each}
              </Select.Content>
            </Select.Root>

            {#if showReauth(src)}
              <GrantCalendarAccessButton
                provider={src.type as 'google' | 'microsoft'}
                accountId={src.accountId ?? ''}
                email={emailByAccountId[src.accountId ?? ''] ?? ''}
                idleLabel={$_('account.reauthorize')}
                busyLabel={$_('calendar.settings.addGoogleGranting')}
                providerIcon
                class="h-8"
                onSuccess={() => { toasts.success($_('account.oauthReauthorized')); void calendarSources.load() }}
                onError={(m) => toasts.error($_('calendar.settings.addGoogleConsentFailed', { values: { message: m } }))}
              />
            {/if}

            <Button
              variant="outline"
              size="sm"
              onclick={() => handleSyncNow(src)}
              disabled={syncingSourceID !== null}
              class="h-8"
            >
              {#if syncingSourceID === src.id}
                <Icon icon="mdi:loading" class="w-3.5 h-3.5 animate-spin" />
              {:else}
                <Icon icon="mdi:sync" class="w-3.5 h-3.5" />
              {/if}
              <span class="ml-1">{$_('calendar.settings.syncNow')}</span>
            </Button>

            <Button
              variant="ghost"
              size="sm"
              onclick={() => { onForce(src) }}
              disabled={syncingSourceID !== null || forceSyncing}
              class="h-8"
              title={$_('calendar.settings.forceResync')}
            >
              {#if forceSyncing && forceTarget?.id === src.id}
                <Icon icon="mdi:loading" class="w-3.5 h-3.5 animate-spin" />
              {:else}
                <Icon icon="mdi:refresh-auto" class="w-3.5 h-3.5" />
              {/if}
            </Button>

            <Button
              variant="ghost"
              size="sm"
              class="h-8 text-destructive hover:text-destructive"
              onclick={() => { onDelete(src) }}
            >
              <Icon icon="mdi:delete-outline" class="w-3.5 h-3.5" />
            </Button>
          </div>
        </div>

        <!-- Source meta: full-width row so longer-language labels
             (e.g. "Kalendáře: 2 · …") don't wrap inside the squeezed
             header column and overlap the interval dropdown. -->
        <div class="text-xs text-muted-foreground">
          {calendarCountLabel(src)} · {lastSyncLabel(src)}
        </div>
        {#if src.lastError}
          <div class="flex items-start gap-1 text-xs text-destructive min-w-0" title={src.lastError}>
            <Icon icon="mdi:alert-circle" class="w-3 h-3 shrink-0 mt-0.5" />
            <span class="truncate">{src.lastError}</span>
          </div>
        {/if}

        <!-- Organizer email row. CalDAV-only: Google + Microsoft
             sources auto-populate organizerIdentities from the bound
             mail account at source-add time and don't need the
             user-edit / re-probe path. CalDAV picks this up from
             PROPFIND <C:calendar-user-address-set>; servers that
             don't publish it land here empty and the user can type
             one. "Re-probe" re-runs the PROPFIND. -->
        {#if src.type === 'caldav'}
        <div class="flex items-center gap-2 pl-2">
          <span class="text-xs text-muted-foreground shrink-0 w-28">
            {$_('calendar.settings.organizerEmailLabel')}
          </span>
          <Input
            type="email"
            placeholder={$_('calendar.settings.organizerEmailPlaceholder')}
            value={organizerDisplay(src)}
            oninput={(e) => {
              organizerEdits[src.id] = (e.currentTarget as HTMLInputElement).value
              organizerEdits = { ...organizerEdits }
            }}
            class="h-8 text-xs flex-1 min-w-0"
          />
          <Button
            variant="outline"
            size="sm"
            class="h-8 shrink-0"
            onclick={() => handleSaveOrganizer(src)}
            disabled={organizerEdits[src.id] === undefined || organizerEdits[src.id] === (src.organizerIdentities?.[0] ?? '')}
          >
            {$_('calendar.common.save')}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            class="h-8 shrink-0"
            onclick={() => handleReprobeOrganizer(src)}
            disabled={reprobingSourceID !== null}
            title={$_('calendar.settings.organizerEmailReprobeTitle')}
          >
            {#if reprobingSourceID === src.id}
              <Icon icon="mdi:loading" class="w-3.5 h-3.5 animate-spin" />
            {/if}
            {#if reprobingSourceID !== src.id}
              <Icon icon="mdi:cloud-search-outline" class="w-3.5 h-3.5" />
            {/if}
          </Button>
        </div>
        {/if}

        <!-- Per-calendar color rows. Color is the calendar's actual
             rendered hue (stored hex if set, else the deterministic
             HSL→hex fallback) — clicking opens the palette + hex input
             popover. -->
        {#each (calendarSources.calendarsBySource[src.id] ?? []) as cal (cal.id)}
          <div class="flex items-center gap-3 pl-2">
            <ColorPicker
              value={calendarSources.colorOfHex(cal.id)}
              onchange={(hex) => { void calendarSources.setColor(cal.id, hex) }}
            />
            <span class="flex-1 min-w-0 truncate text-sm text-foreground">
              {cal.displayName}
            </span>
          </div>
        {/each}
      </div>
    {/each}

    <div class="flex flex-wrap gap-2 mt-2">
      <Button
        variant="outline"
        size="sm"
        onclick={() => { onAddCalDAV() }}
      >
        <Icon icon="mdi:plus" class="w-4 h-4 mr-1" />
        {$_('calendar.sidebar.addSource')}
      </Button>
      <Button
        variant="outline"
        size="sm"
        onclick={() => { onAddGoogle() }}
      >
        <Icon icon="mdi:google" class="w-4 h-4 mr-1" />
        {$_('calendar.settings.addGoogle')}
      </Button>
      <Button
        variant="outline"
        size="sm"
        onclick={() => { onAddMicrosoft() }}
      >
        <Icon icon="mdi:microsoft-outlook" class="w-4 h-4 mr-1" />
        {$_('calendar.settings.addOutlook')}
      </Button>
    </div>
    </div>
    {/if}
  </section>
