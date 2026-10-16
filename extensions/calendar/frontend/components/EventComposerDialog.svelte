<script lang="ts">
  // EventComposerDialog — create + edit form for writable calendars.
  //
  // Two modes: 'create' (build new EventInput) and 'edit' (prefill from
  // an existing event + pass scope for recurring updates). The calendar
  // picker lists every writable calendar (local + CalDAV as of Phase 2
  // Chunk 2; Google + Microsoft in later chunks). Writability is read
  // from Source.Writable, set per provider's CanWrite capability.
  //
  // Date/time inputs render in the user's display timezone via
  // calendarSettings.effectiveTimezone. On save, the local-tz datetime is
  // converted to a UTC unix instant via date-fns-tz's fromZonedTime so
  // the backend always stores absolute time.
  //
  // Form state and save logic: eventComposerState.svelte.ts; pure helpers:
  // eventComposerLogic.ts.

  import { _ } from 'svelte-i18n'
  import * as Dialog from '$lib/components/ui/dialog'
  import * as Select from '$lib/components/ui/select'
  import { Button } from '$lib/components/ui/button'
  import { Input } from '$lib/components/ui/input'
  import { Label } from '$lib/components/ui/label'
  import RichTextEditor from '$lib/components/kit/RichTextEditor.svelte'
  import Icon from '@iconify/svelte'
  import { dialogGuardOpen, dialogGuardClose } from '$lib/stores/dialogGuard'
  import type { SelectedOccurrence } from '$extensions/calendar/frontend/stores/calendarView.svelte'
  // @ts-ignore - wailsjs bindings
  import type { backend } from '$wailsjs/go/models'
  import AttendeesSection from './AttendeesSection.svelte'
  import SendInvitationsDialog from './SendInvitationsDialog.svelte'
  import { EventComposerState, type ComposerMode } from './eventComposerState.svelte'

  interface Props {
    open: boolean
    mode?: ComposerMode
    existing?: backend.Event | null
    // Clicked occurrence of a recurring event; scope this and
    // this-and-future edit it instead of the series start.
    occurrence?: SelectedOccurrence | null
    scope?: 'this' | 'this-and-future' | 'all'
    defaultStart?: Date | null
    defaultCalendarId?: string
    onClose?: () => void
    onSaved?: () => void
  }

  let {
    open = $bindable(false),
    mode = 'create',
    existing = null,
    occurrence = null,
    scope = 'all',
    defaultStart = null,
    defaultCalendarId = '',
    onClose,
    onSaved,
  }: Props = $props()

  const s = new EventComposerState({
    get open() { return open },
    get mode() { return mode },
    get existing() { return existing },
    get occurrence() { return occurrence },
    get scope() { return scope },
    get defaultStart() { return defaultStart },
    get defaultCalendarId() { return defaultCalendarId },
    get onSaved() { return onSaved },
    close,
  })

  // When an error appears, scroll it into view — the error banner lives at
  // the bottom of the scrollable form region and can fall outside the
  // viewport on small dialogs, making a save failure look like a silent
  // refresh. The effect depends on both errorMessage AND errorRef, so it
  // re-fires once the {#if errorMessage} branch mounts and the bind:this
  // assignment populates errorRef.
  $effect(() => {
    if (s.errorMessage && s.errorRef) {
      s.errorRef.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
    }
  })

  // Reset organizer when the user switches calendars mid-compose to a
  // source whose identity list doesn't include the current selection.
  // Without this, a stale cross-source organizer survives the switch
  // and breaks invitations on save — exactly the bug Phase I exists
  // to fix.
  $effect(() => {
    if (!open) return
    const opts = s.identityOptions
    if (opts.length === 0) {
      s.organizer = null
      return
    }
    const current = s.organizer?.email?.toLowerCase()
    if (current && opts.some((o) => o.email.toLowerCase() === current)) return
    s.organizer = { email: opts[0].email.toLowerCase(), cn: opts[0].commonName }
  })

  $effect(() => {
    if (!open) return
    dialogGuardOpen()
    return () => dialogGuardClose()
  })

  $effect(() => {
    if (!open) return
    s.errorMessage = ''
    s.submitting = false
    s.initForm()
  })

  function close() {
    if (s.submitting) return
    open = false
    onClose?.()
  }

  function recurrenceFreqLabel(freq: string): string {
    if (freq === 'DAILY') return $_('calendar.composer.recurrence.daily')
    if (freq === 'WEEKLY') return $_('calendar.composer.recurrence.weekly')
    if (freq === 'MONTHLY') return $_('calendar.composer.recurrence.monthly')
    if (freq === 'YEARLY') return $_('calendar.composer.recurrence.yearly')
    return $_('calendar.composer.recurrence.none')
  }

  function recurrenceEndLabel(v: string): string {
    if (v === 'date') return $_('calendar.composer.recurrence.endOnDate')
    if (v === 'count') return $_('calendar.composer.recurrence.endAfterCount')
    return $_('calendar.composer.recurrence.endNever')
  }

  function reminderLabel(): string {
    if (s.reminderChoice === 'none') return $_('calendar.composer.reminder.none')
    if (s.reminderChoice === 'custom') {
      return $_('calendar.composer.reminder.customLabel', { values: { n: s.reminderCustomMinutes } })
    }
    if (s.reminderChoice === '0') return $_('calendar.composer.reminder.atTime')
    if (s.reminderChoice === '5') return $_('calendar.composer.reminder.fiveMin')
    if (s.reminderChoice === '15') return $_('calendar.composer.reminder.fifteenMin')
    if (s.reminderChoice === '30') return $_('calendar.composer.reminder.thirtyMin')
    if (s.reminderChoice === '60') return $_('calendar.composer.reminder.oneHour')
    if (s.reminderChoice === '1440') return $_('calendar.composer.reminder.oneDay')
    return s.reminderChoice
  }
</script>

<!-- Close on Esc ourselves (same as the mail composer). bits-ui's own Esc
     isn't reliably closing this dialog when the rich-text editor is mounted,
     and DetailOverlay yields Esc to us via its dialogGuard check. Only act on
     our own Esc: not while the invitations dialog is open, and not when a
     Select dropdown was open, since bits-ui closes that on its own. The
     capture phase sees the dropdown before bits-ui's handler removes it. -->
<svelte:window
  onkeydowncapture={(e) => {
    s.escClosesDropdown = e.key === 'Escape' && document.querySelector('[role="listbox"]') !== null
  }}
  onkeydown={(e) => {
    if (!open) return
    if (e.key !== 'Escape') return
    if (s.sendInvitationsOpen || s.escClosesDropdown) return
    e.preventDefault()
    close()
  }}
/>

<Dialog.Root bind:open onOpenChange={(v) => { if (!v) close() }}>
  <Dialog.Content class="max-w-lg">
    <Dialog.Header>
      <Dialog.Title>
        {mode === 'edit' ? $_('calendar.composer.titleEdit') : $_('calendar.composer.titleCreate')}
      </Dialog.Title>
    </Dialog.Header>

    <div class="space-y-3 mt-2 max-h-[60vh] overflow-y-auto pl-1 pr-3">
      <div>
        <Label for="cal-composer-summary">{$_('calendar.composer.summaryLabel')}</Label>
        <Input
          id="cal-composer-summary"
          type="text"
          placeholder={$_('calendar.composer.summaryPlaceholder')}
          bind:value={s.summary}
          disabled={s.submitting}
        />
      </div>

      <div>
        <Label>{$_('calendar.composer.calendarLabel')}</Label>
        {#if s.writableCalendars.length === 0}
          <p class="text-xs text-destructive mt-1">{$_('calendar.composer.noLocalCalendars')}</p>
        {/if}
        {#if s.writableCalendars.length > 0}
          <Select.Root value={s.calendarId} onValueChange={(v) => { if (v) s.calendarId = v }}>
            <Select.Trigger class="h-9">
              {s.writableCalendars.find(c => c.id === s.calendarId)?.name ?? ''}
            </Select.Trigger>
            <Select.Content>
              {#each s.writableCalendars as c (c.id)}
                <Select.Item value={c.id} label={c.name} />
              {/each}
            </Select.Content>
          </Select.Root>
        {/if}
      </div>

      <label class="flex items-center gap-2 text-sm">
        <input type="checkbox" bind:checked={s.isAllDay} disabled={s.submitting} class="accent-primary" />
        <span>{$_('calendar.composer.allDayLabel')}</span>
      </label>

      <div class="grid grid-cols-2 gap-2">
        <div>
          <Label for="cal-composer-startdate">{$_('calendar.composer.startDateLabel')}</Label>
          <Input id="cal-composer-startdate" type="date" bind:value={s.startDate} disabled={s.submitting} />
        </div>
        {#if !s.isAllDay}
          <div>
            <Label for="cal-composer-starttime">{$_('calendar.composer.startTimeLabel')}</Label>
            <Input id="cal-composer-starttime" type="time" bind:value={s.startTime} disabled={s.submitting} />
          </div>
        {/if}
      </div>

      <div class="grid grid-cols-2 gap-2">
        <div>
          <Label for="cal-composer-enddate">{$_('calendar.composer.endDateLabel')}</Label>
          <Input id="cal-composer-enddate" type="date" bind:value={s.endDate} disabled={s.submitting} />
        </div>
        {#if !s.isAllDay}
          <div>
            <Label for="cal-composer-endtime">{$_('calendar.composer.endTimeLabel')}</Label>
            <Input id="cal-composer-endtime" type="time" bind:value={s.endTime} disabled={s.submitting} />
          </div>
        {/if}
      </div>

      <div>
        <Label for="cal-composer-location">{$_('calendar.composer.locationLabel')}</Label>
        <Input id="cal-composer-location" type="text" bind:value={s.location} disabled={s.submitting} />
      </div>

      <div>
        <Label>{$_('calendar.composer.descriptionLabel')}</Label>
        <RichTextEditor
          value={s.initialBody}
          disabled={s.submitting}
          placeholder={$_('calendar.composer.descriptionLabel')}
          onChange={(html, text) => {
            // Save the editor's HTML as the description so the detail view
            // renders it (the same {@html} path used for synced HTML events).
            // text drives the empty check so a blank editor stores "".
            s.description = text.trim() ? html : ''
            s.descriptionHTML = text.trim() ? html : ''
          }}
        />
      </div>

      <AttendeesSection
        bind:attendees={s.attendees}
        bind:organizer={s.organizer}
        selfEmails={s.selfEmails}
        identities={s.identityOptions}
        capability={s.hasAttendeesCapability}
        dayUnixForFreeBusy={s.dayUnixForFreeBusy}
        durationMinutes={s.pickedDurationMinutes}
        onFreeBusyPick={s.applyPickedSlot}
        disabled={s.submitting}
      />

      <div>
        <Label>{$_('calendar.composer.recurrenceLabel')}</Label>
        <Select.Root value={s.recurrenceFreq} onValueChange={(v) => { s.recurrenceFreq = v ?? '' }}>
          <Select.Trigger class="h-9">
            {recurrenceFreqLabel(s.recurrenceFreq)}
          </Select.Trigger>
          <Select.Content>
            <Select.Item value="" label={$_('calendar.composer.recurrence.none')} />
            <Select.Item value="DAILY" label={$_('calendar.composer.recurrence.daily')} />
            <Select.Item value="WEEKLY" label={$_('calendar.composer.recurrence.weekly')} />
            <Select.Item value="MONTHLY" label={$_('calendar.composer.recurrence.monthly')} />
            <Select.Item value="YEARLY" label={$_('calendar.composer.recurrence.yearly')} />
          </Select.Content>
        </Select.Root>

        {#if s.recurrenceFreq}
          <div class="mt-2 grid grid-cols-2 gap-2">
            <Select.Root value={s.recurrenceEnd} onValueChange={(v) => { if (v) s.recurrenceEnd = v }}>
              <Select.Trigger class="h-9">
                {recurrenceEndLabel(s.recurrenceEnd)}
              </Select.Trigger>
              <Select.Content>
                <Select.Item value="never" label={$_('calendar.composer.recurrence.endNever')} />
                <Select.Item value="date" label={$_('calendar.composer.recurrence.endOnDate')} />
                <Select.Item value="count" label={$_('calendar.composer.recurrence.endAfterCount')} />
              </Select.Content>
            </Select.Root>
            {#if s.recurrenceEnd === 'date'}
              <Input type="date" bind:value={s.recurrenceUntilDate} disabled={s.submitting} />
            {/if}
            {#if s.recurrenceEnd === 'count'}
              <Input type="number" min="1" bind:value={s.recurrenceCount} disabled={s.submitting} />
            {/if}
          </div>
        {/if}
      </div>

      <div>
        <Label>{$_('calendar.composer.reminderLabel')}</Label>
        <Select.Root value={s.reminderChoice} onValueChange={(v) => { if (v) s.reminderChoice = v }}>
          <Select.Trigger class="h-9">
            {reminderLabel()}
          </Select.Trigger>
          <Select.Content>
            <Select.Item value="none" label={$_('calendar.composer.reminder.none')} />
            <Select.Item value="0" label={$_('calendar.composer.reminder.atTime')} />
            <Select.Item value="5" label={$_('calendar.composer.reminder.fiveMin')} />
            <Select.Item value="15" label={$_('calendar.composer.reminder.fifteenMin')} />
            <Select.Item value="30" label={$_('calendar.composer.reminder.thirtyMin')} />
            <Select.Item value="60" label={$_('calendar.composer.reminder.oneHour')} />
            <Select.Item value="1440" label={$_('calendar.composer.reminder.oneDay')} />
            <Select.Item value="custom" label={$_('calendar.composer.reminder.custom')} />
          </Select.Content>
        </Select.Root>
        {#if s.reminderChoice === 'custom'}
          <div class="mt-2">
            <Label for="cal-composer-reminder-custom">{$_('calendar.composer.reminder.customMinutesLabel')}</Label>
            <Input
              id="cal-composer-reminder-custom"
              type="number"
              min="0"
              bind:value={s.reminderCustomMinutes}
              disabled={s.submitting}
            />
          </div>
        {/if}
      </div>

      <div>
        <Label>{$_('calendar.composer.availabilityLabel')}</Label>
        <Select.Root value={s.transparency} onValueChange={(v) => { if (v) s.transparency = v }}>
          <Select.Trigger class="h-9">
            {s.transparency === 'free' ? $_('calendar.composer.availability.free') : $_('calendar.composer.availability.busy')}
          </Select.Trigger>
          <Select.Content>
            <Select.Item value="busy" label={$_('calendar.composer.availability.busy')} />
            <Select.Item value="free" label={$_('calendar.composer.availability.free')} />
          </Select.Content>
        </Select.Root>
      </div>

      <div>
        <Label>{$_('calendar.composer.visibilityLabel')}</Label>
        <Select.Root value={s.visibility} onValueChange={(v) => { if (v) s.visibility = v }}>
          <Select.Trigger class="h-9">
            {$_('calendar.composer.visibility.' + s.visibility)}
          </Select.Trigger>
          <Select.Content>
            <Select.Item value="public" label={$_('calendar.composer.visibility.public')} />
            <Select.Item value="private" label={$_('calendar.composer.visibility.private')} />
            <Select.Item value="confidential" label={$_('calendar.composer.visibility.confidential')} />
          </Select.Content>
        </Select.Root>
      </div>

      {#if s.errorMessage}
        <div bind:this={s.errorRef} class="flex items-start gap-2 p-2 bg-destructive/10 rounded text-sm">
          <Icon icon="mdi:alert-circle" class="w-4 h-4 text-destructive shrink-0 mt-0.5" />
          <div class="text-xs text-destructive wrap-break-word">{s.errorMessage}</div>
        </div>
      {/if}
    </div>

    <div class="flex items-center justify-end gap-2 pt-4 border-t border-border mt-4">
      <Button variant="ghost" onclick={close} disabled={s.submitting}>
        {$_('calendar.common.cancel')}
      </Button>
      <Button onclick={() => s.handleSave()} disabled={s.submitting || s.writableCalendars.length === 0}>
        {#if s.submitting}<Icon icon="mdi:loading" class="w-4 h-4 mr-1 animate-spin" />{/if}
        {$_('calendar.common.save')}
      </Button>
    </div>
  </Dialog.Content>
</Dialog.Root>

<SendInvitationsDialog
  bind:open={s.sendInvitationsOpen}
  attendeeCount={s.attendees.length}
  sourceKind={s.sourceKind}
  onConfirm={async (choice) => {
    s.sendUpdates = choice
    await s.performSave()
  }}
  onCancel={() => {
    // Cancel just returns to the composer; user's edits are intact.
  }}
/>
