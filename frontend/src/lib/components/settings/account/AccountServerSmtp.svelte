<script lang="ts">
  import Icon from '@iconify/svelte'
  import { Input } from '$lib/components/ui/input'
  import { Label } from '$lib/components/ui/label'
  import * as Select from '$lib/components/ui/select'
  import Switch from '$lib/components/ui/switch/Switch.svelte'
  import { securityOptions, authMechanismOptions } from '$lib/config/providers'
  import { _ } from '$lib/i18n'
  import { getSecurityLabel, getAuthMechanismLabel } from './accountServerLabels'

  interface Props {
    editAccount: { smtpUsername?: string }
    smtpHost: string
    smtpPort: number
    smtpSecurity: string
    smtpUsername: string
    smtpPassword: string
    smtpAuthMechanism: string
    isGenericProvider: boolean
    errors: Record<string, string>
    onSmtpHostChange: (value: string) => void
    onSmtpPortChange: (value: number) => void
    onSmtpSecurityChange: (value: string) => void
    onSmtpUsernameChange: (value: string) => void
    onSmtpPasswordChange: (value: string) => void
    onSmtpAuthMechanismChange: (value: string) => void
  }

  let {
    editAccount,
    smtpHost = $bindable(),
    smtpPort = $bindable(),
    smtpSecurity = $bindable(),
    smtpUsername = $bindable(''),
    smtpPassword = $bindable(''),
    smtpAuthMechanism = $bindable('auto'),
    isGenericProvider,
    errors,
    onSmtpHostChange,
    onSmtpPortChange,
    onSmtpSecurityChange,
    onSmtpUsernameChange,
    onSmtpPasswordChange,
    onSmtpAuthMechanismChange,
  }: Props = $props()

  // SMTP "Same as incoming server" toggle. Derived from the persisted
  // editAccount.smtpUsername (empty string = on). Tracked as its own
  // state so the user can toggle off, type a username, toggle back on
  // (clearing), and toggle off again without losing UI continuity. On
  // save the parent reads the final smtpUsername — empty means SMTP
  // uses IMAP creds, non-empty means use the separate-creds path.
  //
  // IMPORTANT: capture from editAccount.smtpUsername, NOT from the
  // smtpUsername prop. bits-ui Tabs.Content always-renders its children
  // (only hides via CSS), so this component mounts immediately when the
  // dialog opens — BEFORE the parent dialog's $effect has had a chance
  // to populate the form state `smtpUsername` from editAccount. Reading
  // the form prop here captured the parent's initial empty default,
  // which is why the toggle always appeared as "Same as IMAP" on reopen
  // / restart even when separate creds were persisted. The const indirection
  // is to suppress Svelte's "only captures initial value" warning —
  // initial-only capture is exactly what we want here (editAccount is
  // stable for the life of this mount; the dialog unmounts on close).
  // svelte-ignore state_referenced_locally
  let smtpUseSameAsIncoming = $state(editAccount.smtpUsername === '')

  function handleSmtpUseSameAsIncomingChange(v: boolean) {
    smtpUseSameAsIncoming = v
    if (v) {
      smtpUsername = ''
      smtpPassword = ''
      onSmtpUsernameChange('')
      onSmtpPasswordChange('')
    }
  }
</script>

  <!-- Outgoing Mail (SMTP) -->
  <div class="space-y-4">
    <h3 class="text-sm font-medium flex items-center gap-2">
      <Icon icon="mdi:email-send-outline" class="w-4 h-4" />
      {$_('account.outgoingMail')}
    </h3>

    <div class="grid grid-cols-2 gap-3">
      <div class="space-y-2">
        <Label for="smtpHost">{$_('account.server')}</Label>
        <Input
          id="smtpHost"
          type="text"
          placeholder="smtp.example.com"
          bind:value={smtpHost}
          oninput={(e) => onSmtpHostChange((e.target as HTMLInputElement).value)}
          class={errors.smtpHost ? 'border-destructive' : ''}
        />
        {#if errors.smtpHost}
          <p class="text-sm text-destructive">{errors.smtpHost}</p>
        {/if}
      </div>
      <div class="grid grid-cols-2 gap-2">
        <div class="space-y-2">
          <Label for="smtpPort">{$_('account.port')}</Label>
          <Input
            id="smtpPort"
            type="number"
            bind:value={smtpPort}
            oninput={(e) => onSmtpPortChange(Number((e.target as HTMLInputElement).value))}
            class={errors.smtpPort ? 'border-destructive' : ''}
          />
        </div>
        <div class="space-y-2">
          <Label>{$_('account.security')}</Label>
          <Select.Root
            value={smtpSecurity}
            onValueChange={(v) => { smtpSecurity = v; onSmtpSecurityChange(v) }}
          >
            <Select.Trigger class="h-10">
              <Select.Value placeholder="Select">
                {getSecurityLabel(smtpSecurity)}
              </Select.Value>
            </Select.Trigger>
            <Select.Content>
              {#each securityOptions as opt (opt.value)}
                <Select.Item value={opt.value} label={opt.label} />
              {/each}
            </Select.Content>
          </Select.Root>
        </div>
      </div>
    </div>

    {#if isGenericProvider}
      <!-- SMTP authentication: "Same as incoming server" toggle (on by
           default). When off, the user supplies a separate SMTP
           username + password. Generic provider only. -->
      <div class="space-y-3 pt-3 border-t border-border">
        <h4 class="text-sm font-medium">{$_('account.smtpAuthentication')}</h4>
        <label class="flex items-center gap-3 text-sm">
          <Switch
            checked={smtpUseSameAsIncoming}
            onCheckedChange={handleSmtpUseSameAsIncomingChange}
          />
          <span>{$_('account.smtpUseSameAsIncoming')}</span>
        </label>
        {#if !smtpUseSameAsIncoming}
          <div class="grid grid-cols-2 gap-3">
            <div class="space-y-2">
              <Label for="smtpUsername">{$_('account.username')}</Label>
              <Input
                id="smtpUsername"
                type="text"
                placeholder={$_('account.smtpUsernamePlaceholder')}
                bind:value={smtpUsername}
                oninput={(e) => onSmtpUsernameChange((e.target as HTMLInputElement).value)}
                class={errors.smtpUsername ? 'border-destructive' : ''}
              />
              {#if errors.smtpUsername}
                <p class="text-sm text-destructive">{errors.smtpUsername}</p>
              {/if}
            </div>
            <div class="space-y-2">
              <Label for="smtpPassword">{$_('account.password')}</Label>
              <Input
                id="smtpPassword"
                type="password"
                placeholder={$_('account.leaveEmptyToKeep')}
                bind:value={smtpPassword}
                oninput={(e) => onSmtpPasswordChange((e.target as HTMLInputElement).value)}
                class={errors.smtpPassword ? 'border-destructive' : ''}
              />
              {#if errors.smtpPassword}
                <p class="text-sm text-destructive">{errors.smtpPassword}</p>
              {/if}
            </div>
            <div class="space-y-2">
              <Label>{$_('account.authMechanism')}</Label>
              <Select.Root
                value={smtpAuthMechanism}
                onValueChange={(v) => { smtpAuthMechanism = v; onSmtpAuthMechanismChange(v) }}
              >
                <Select.Trigger class="h-10">
                  <Select.Value placeholder="Select">
                    {getAuthMechanismLabel(smtpAuthMechanism)}
                  </Select.Value>
                </Select.Trigger>
                <Select.Content>
                  {#each authMechanismOptions as opt (opt.value)}
                    <Select.Item value={opt.value} label={opt.label} />
                  {/each}
                </Select.Content>
              </Select.Root>
            </div>
          </div>
        {/if}
      </div>
    {/if}
  </div>
