<script lang="ts">
  // CalendarSettingsDialog — Calendar extension's settings.
  //
  // Two sections:
  //   1. Sources — per-source row with sync-interval picker, Sync Now,
  //      Delete, last-sync info. Add CalDAV button at the bottom.
  //   2. Alarms — informational copy (per-source toggles deferred).

  import { _ } from 'svelte-i18n'
  import * as Dialog from '$lib/components/ui/dialog'
  import { Button } from '$lib/components/ui/button'
  import ConfirmDialog from '$lib/components/kit/ConfirmDialog.svelte'
  import OAuthCredsSlotEditor from '$lib/components/kit/OAuthCredsSlotEditor.svelte'
  import AddLocalCalendarDialog from './AddLocalCalendarDialog.svelte'
  import AddGoogleCalendarDialog from './AddGoogleCalendarDialog.svelte'
  import AddMicrosoftCalendarDialog from './AddMicrosoftCalendarDialog.svelte'
  import Icon from '@iconify/svelte'
  import { toasts } from '$lib/stores/toast'
  import { dialogGuardOpen, dialogGuardClose } from '$lib/stores/dialogGuard'
  import { calendarSources } from '$extensions/calendar/frontend/stores/calendarSources.svelte'
  import AddCalDAVSourceDialog from './AddCalDAVSourceDialog.svelte'
  // @ts-ignore - wailsjs bindings
  import { Calendar_DeleteCalendar, GetAccounts } from '$wailsjs/go/app/App.js'
  import { logger } from '$extensions/calendar/frontend/lib/logger'
  import TimezonePicker from './TimezonePicker.svelte'
  import CalendarSettingsLocalSection from './CalendarSettingsLocalSection.svelte'
  import CalendarSettingsSourcesSection from './CalendarSettingsSourcesSection.svelte'
  import CalendarSettingsDefaultsSection from './CalendarSettingsDefaultsSection.svelte'
  // @ts-ignore - wailsjs bindings
  import type { backend } from '$wailsjs/go/models'

  interface Props {
    open: boolean
    onClose?: () => void
  }

  let { open = $bindable(false), onClose }: Props = $props()

  // Add-source dialog opens from inside this one.
  let showAddSource = $state(false)
  let showAddLocalCalendar = $state(false)
  let showAddGoogle = $state(false)
  let showAddMicrosoft = $state(false)
  let deleteCalendarTarget = $state<{ id: string; displayName: string } | null>(null)
  let deletingCalendar = $state(false)


  // Per-row state — pending delete.
  let deleteTarget = $state<backend.Source | null>(null)
  let deleting = $state(false)
  // Pending force-resync confirmation + in-flight flag.
  let forceTarget = $state<backend.Source | null>(null)
  let forceSyncing = $state(false)
  let emailByAccountId = $state<Record<string, string>>({})

  // Collapsible-section state. Object keyed by section id; entries
  // default to false (collapsed) so the user can scan all category
  // titles at once and drill into one. Toggle persists only for the
  // dialog's lifetime — re-opening the dialog re-collapses everything,
  // which keeps the discovery / scanning experience intentional.
  let expanded = $state<Record<string, boolean>>({
    local: false,
    sources: false,
    defaults: false,
    timezone: false,
    oauth: false,
  })

  function toggle(key: string) {
    expanded[key] = !expanded[key]
    expanded = { ...expanded }
  }

  async function handleConfirmForceResync() {
    if (!forceTarget) return
    forceSyncing = true
    const name = forceTarget.name
    try {
      await calendarSources.forceSyncSource(forceTarget.id)
      toasts.success($_('calendar.toast.forceResyncDone', { values: { name } }))
      forceTarget = null
    } catch (err) {
      const msg = (err as Error)?.message ?? String(err)
      toasts.error(msg)
    } finally {
      forceSyncing = false
    }
  }

  async function handleConfirmDelete() {
    if (!deleteTarget) return
    deleting = true
    try {
      await calendarSources.deleteSource(deleteTarget.id)
      toasts.success($_('calendar.toast.sourceDeleted', { values: { name: deleteTarget.name } }))
      deleteTarget = null
    } catch (err) {
      const msg = (err as Error)?.message ?? String(err)
      toasts.error(msg)
    } finally {
      deleting = false
    }
  }

  async function handleConfirmDeleteCalendar() {
    if (!deleteCalendarTarget) return
    deletingCalendar = true
    const name = deleteCalendarTarget.displayName
    try {
      await Calendar_DeleteCalendar(deleteCalendarTarget.id)
      await calendarSources.load()
      toasts.success($_('calendar.settings.calendarDeleted', { values: { name } }))
      deleteCalendarTarget = null
    } catch (err) {
      const msg = (err as Error)?.message ?? String(err)
      toasts.error(msg)
    } finally {
      deletingCalendar = false
    }
  }

  function close() {
    open = false
    onClose?.()
  }

  $effect(() => {
    if (open) {
      dialogGuardOpen()
      void calendarSources.load()
      void loadReauthState()
      return () => dialogGuardClose()
    }
  })

  // Loads the account emails the per-source Reauthorize button needs
  // (GrantCalendarAccessButton requires the expected email).
  async function loadReauthState() {
    try {
      const accts = (await GetAccounts()) || []
      const map: Record<string, string> = {}
      for (const a of accts) map[a.id] = a.email
      emailByAccountId = map
    } catch (e) {
      logger.warn(`reauth: load accounts failed: ${e}`)
    }
  }

  // Reading $locale at template time below already binds reactivity for the
  // formatter calls; no local derived needed.
</script>

<Dialog.Root bind:open onOpenChange={(v) => { if (!v) close() }}>
  <Dialog.Content class="max-w-2xl">
    <Dialog.Header>
      <Dialog.Title>{$_('calendar.settings.title')}</Dialog.Title>
    </Dialog.Header>

    <div class="space-y-3 mt-2 max-h-[60vh] overflow-y-auto pr-1">
      <CalendarSettingsLocalSection
        expanded={expanded.local}
        ontoggle={() => toggle('local')}
        onDelete={(cal) => { deleteCalendarTarget = cal }}
        onAdd={() => { showAddLocalCalendar = true }}
      />

      <CalendarSettingsSourcesSection
        expanded={expanded.sources}
        ontoggle={() => toggle('sources')}
        {emailByAccountId}
        {forceTarget}
        {forceSyncing}
        onForce={(src) => { forceTarget = src }}
        onDelete={(src) => { deleteTarget = src }}
        onAddCalDAV={() => { showAddSource = true }}
        onAddGoogle={() => { showAddGoogle = true }}
        onAddMicrosoft={() => { showAddMicrosoft = true }}
      />

      <CalendarSettingsDefaultsSection
        expanded={expanded.defaults}
        ontoggle={() => toggle('defaults')}
      />

      <!-- Display timezone — picker for choosing how times render across
           views. The same TimezonePicker is also mounted in ViewSwitcher's
           top-right; both paths write to calendarSettings.displayTimezone. -->
      <section class="border border-border rounded-md">
        <button
          type="button"
          class="w-full flex items-center justify-between p-3 text-left hover:bg-muted/30 rounded-md"
          onclick={() => toggle('timezone')}
          aria-expanded={expanded.timezone}
        >
          <h3 class="text-sm font-semibold text-foreground">
            {$_('calendar.settings.timezoneSection')}
          </h3>
          <Icon icon={expanded.timezone ? 'mdi:chevron-up' : 'mdi:chevron-down'} class="w-4 h-4 text-muted-foreground" />
        </button>
        {#if expanded.timezone}
        <div class="px-3 pb-3 pt-1 space-y-2">
          <p class="text-xs text-muted-foreground">
            {$_('calendar.settings.timezoneHelp')}
          </p>
          <TimezonePicker />
        </div>
        {/if}
      </section>

      <!-- OAuth Credentials (advanced) — picker matches Contacts'. Google
           shows "Mailven testing" as the default since the mail-app's verified
           client carries no Calendar scopes. Microsoft resolves to mail's
           client (consolidated in core_provider.go); Custom is always an
           escape hatch. -->
      <section class="border border-border rounded-md">
        <button
          type="button"
          class="w-full flex items-center justify-between p-3 text-left hover:bg-muted/30 rounded-md"
          onclick={() => toggle('oauth')}
          aria-expanded={expanded.oauth}
        >
          <h3 class="text-sm font-semibold text-foreground">
            {$_('calendar.settings.oauthSection')}
          </h3>
          <Icon icon={expanded.oauth ? 'mdi:chevron-up' : 'mdi:chevron-down'} class="w-4 h-4 text-muted-foreground" />
        </button>
        {#if expanded.oauth}
        <div class="px-3 pb-3 pt-1 space-y-3">
          <p class="text-xs text-muted-foreground">
            {$_('calendar.settings.oauthHelp')}
          </p>
          <OAuthCredsSlotEditor
            configID="google-calendar"
            extensionID="calendar"
            label={$_('calendar.settings.oauthGoogleLabel')}
            secretRequired={true}
          />
          <OAuthCredsSlotEditor
            configID="microsoft-calendar"
            extensionID="calendar"
            label={$_('calendar.settings.oauthMicrosoftLabel')}
            secretRequired={false}
          />
        </div>
        {/if}
      </section>
    </div>

    <div class="flex items-center justify-end gap-2 pt-4 border-t border-border mt-4">
      <Button variant="ghost" onclick={close}>{$_('calendar.common.close')}</Button>
    </div>
  </Dialog.Content>
</Dialog.Root>

<AddCalDAVSourceDialog bind:open={showAddSource} onClose={() => { void calendarSources.load() }} />

<AddLocalCalendarDialog bind:open={showAddLocalCalendar} onCreated={() => { void calendarSources.load() }} />

<AddGoogleCalendarDialog bind:open={showAddGoogle} onClose={() => { void calendarSources.load() }} />

<AddMicrosoftCalendarDialog bind:open={showAddMicrosoft} onClose={() => { void calendarSources.load() }} />

<ConfirmDialog
  open={deleteTarget !== null}
  title={deleteTarget ? $_('calendar.settings.deleteConfirmTitle', { values: { name: deleteTarget.name } }) : ''}
  description={$_('calendar.settings.deleteConfirmDescription')}
  confirmLabel={$_('calendar.common.delete')}
  cancelLabel={$_('calendar.common.cancel')}
  variant="destructive"
  loading={deleting}
  onConfirm={handleConfirmDelete}
  onCancel={() => { deleteTarget = null }}
/>

<ConfirmDialog
  open={forceTarget !== null}
  title={forceTarget ? $_('calendar.settings.forceResyncConfirmTitle', { values: { name: forceTarget.name } }) : ''}
  description={$_('calendar.settings.forceResyncConfirmDescription')}
  confirmLabel={$_('calendar.settings.forceResync')}
  cancelLabel={$_('calendar.common.cancel')}
  loading={forceSyncing}
  onConfirm={handleConfirmForceResync}
  onCancel={() => { forceTarget = null }}
/>
<ConfirmDialog
  open={deleteCalendarTarget !== null}
  title={deleteCalendarTarget ? $_('calendar.settings.deleteCalendarConfirmTitle', { values: { name: deleteCalendarTarget.displayName } }) : ''}
  description={$_('calendar.settings.deleteCalendarConfirmDescription')}
  confirmLabel={$_('calendar.common.delete')}
  cancelLabel={$_('calendar.common.cancel')}
  variant="destructive"
  loading={deletingCalendar}
  onConfirm={handleConfirmDeleteCalendar}
  onCancel={() => { deleteCalendarTarget = null }}
/>