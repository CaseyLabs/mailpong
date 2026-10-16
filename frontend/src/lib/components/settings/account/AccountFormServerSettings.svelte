<script lang="ts">
  import { Input } from '$lib/components/ui/input'
  import { Label } from '$lib/components/ui/label'
  import * as Select from '$lib/components/ui/select'
  import Switch from '$lib/components/ui/switch/Switch.svelte'
  import { securityOptions, authMechanismOptions } from '$lib/config/providers'
  import { handleSmtpUseSameAsIncomingChange } from './accountFormLoad'
  import type { AccountFormState } from './accountFormState.svelte'
  import { _ } from '$lib/i18n'

  let { s }: { s: AccountFormState } = $props()

  // Helper functions to get labels
  function getSecurityLabel(value: string): string {
    return securityOptions.find(opt => opt.value === value)?.label || value
  }

  function getAuthMechanismLabel(value: string): string {
    return authMechanismOptions.find(opt => opt.value === value)?.label || value
  }
</script>
<!-- IMAP Settings -->
<div class="space-y-3">
  <h4 class="text-sm font-medium">{$_('account.incomingMail')}</h4>
  <div class="grid grid-cols-2 gap-3">
    <div class="space-y-2">
      <Label for="imapHost">{$_('account.server')}</Label>
      <Input
        id="imapHost"
        type="text"
        placeholder="imap.example.com"
        bind:value={s.imapHost}
        oninput={(e) => {
          const v = (e.target as HTMLInputElement).value
          if (s.isGenericProvider && !s.editAccount && s.smtpHostMirrorsImap) {
            s.smtpHost = v
          }
        }}
        class={s.errors.imapHost ? 'border-destructive' : ''}
      />
      {#if s.errors.imapHost}
        <p class="text-sm text-destructive">{s.errors.imapHost}</p>
      {/if}
    </div>
    <div class="grid grid-cols-2 gap-2">
      <div class="space-y-2">
        <Label for="imapPort">{$_('account.port')}</Label>
        <Input
          id="imapPort"
          type="number"
          bind:value={s.imapPort}
          class={s.errors.imapPort ? 'border-destructive' : ''}
        />
      </div>
      <div class="space-y-2">
        <Label>{$_('account.security')}</Label>
        <Select.Root bind:value={s.imapSecurity}>
          <Select.Trigger class="h-10">
            <Select.Value placeholder="Select">
              {getSecurityLabel(s.imapSecurity)}
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
  {#if s.isGenericProvider}
    <div class="grid grid-cols-2 gap-3">
      <div class="space-y-2">
        <Label>{$_('account.authMechanism')}</Label>
        <Select.Root bind:value={s.imapAuthMechanism}>
          <Select.Trigger class="h-10">
            <Select.Value placeholder="Select">
              {getAuthMechanismLabel(s.imapAuthMechanism)}
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

<!-- "No outgoing server" toggle (above SMTP). When on, SMTP +
     SMTP-auth sections collapse and the composer's From dropdown
     excludes this account. -->
<div class="space-y-2">
  <label class="flex items-center gap-3 text-sm">
    <Switch bind:checked={s.noOutgoingServer} />
    <span class="font-medium">{$_('account.noOutgoingServer')}</span>
  </label>
  <p class="text-xs text-muted-foreground">{$_('account.noOutgoingServerHelp')}</p>

  {#if s.noOutgoingServer}
    <!-- Reply/Forward-with picker. Same shape as the composer's
         From dropdown. Default = empty value, which the composer
         resolves to the user's default sending identity. -->
    <div class="pt-2 space-y-1">
      <Label>{$_('account.replyForwardWith')}</Label>
      <Select.Root bind:value={s.replyForwardIdentityID}>
        <Select.Trigger class="h-10">
          <Select.Value placeholder={$_('account.replyForwardWithDefault')}>
            {#if s.replyForwardIdentityID}
              {@const allIdentities = s.availableIdentityGroups.flatMap(g => (g.identities || []).map(i => ({ identity: i, group: g })))}
              {@const found = allIdentities.find(x => x.identity.id === s.replyForwardIdentityID)}
              {#if found}
                {#if found.group.account?.color}
                  <span class="inline-block w-2 h-2 rounded-full mr-1.5 shrink-0" style="background-color: {found.group.account.color}"></span>
                {/if}
                {found.identity.name} &lt;{found.identity.email}&gt;
              {:else}
                {$_('account.replyForwardWithDefault')}
              {/if}
            {:else}
              {$_('account.replyForwardWithDefault')}
            {/if}
          </Select.Value>
        </Select.Trigger>
        <Select.Content>
          <Select.Item value="" label={$_('account.replyForwardWithDefault')} />
          {#each s.availableIdentityGroups as group (group.account?.id)}
            <Select.Group>
              <Select.GroupHeading class="flex items-center gap-1.5 px-2 py-1 text-xs font-medium text-muted-foreground">
                {#if group.account?.color}
                  <span class="inline-block w-2 h-2 rounded-full shrink-0" style="background-color: {group.account.color}"></span>
                {/if}
                {group.account?.name || group.account?.email}
              </Select.GroupHeading>
              {#each group.identities || [] as identity (identity.id)}
                <Select.Item value={identity.id} label="{identity.name} <{identity.email}>" />
              {/each}
            </Select.Group>
          {/each}
        </Select.Content>
      </Select.Root>
      <p class="text-xs text-muted-foreground">{$_('account.replyForwardWithHelp')}</p>
    </div>
  {/if}
</div>

{#if !s.noOutgoingServer}
<!-- SMTP Settings -->
<div class="space-y-3">
  <h4 class="text-sm font-medium">{$_('account.outgoingMail')}</h4>
  <div class="grid grid-cols-2 gap-3">
    <div class="space-y-2">
      <Label for="smtpHost">{$_('account.server')}</Label>
      <Input
        id="smtpHost"
        type="text"
        placeholder="smtp.example.com"
        bind:value={s.smtpHost}
        oninput={() => { s.smtpHostMirrorsImap = false }}
        class={s.errors.smtpHost ? 'border-destructive' : ''}
      />
      {#if s.errors.smtpHost}
        <p class="text-sm text-destructive">{s.errors.smtpHost}</p>
      {/if}
    </div>
    <div class="grid grid-cols-2 gap-2">
      <div class="space-y-2">
        <Label for="smtpPort">{$_('account.port')}</Label>
        <Input
          id="smtpPort"
          type="number"
          bind:value={s.smtpPort}
          class={s.errors.smtpPort ? 'border-destructive' : ''}
        />
      </div>
      <div class="space-y-2">
        <Label>{$_('account.security')}</Label>
        <Select.Root bind:value={s.smtpSecurity}>
          <Select.Trigger class="h-10">
            <Select.Value placeholder="Select">
              {getSecurityLabel(s.smtpSecurity)}
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

  {#if s.isGenericProvider}
    <!-- SMTP authentication subsection — Generic only. -->
    <div class="space-y-3 pt-3 border-t border-border">
      <h4 class="text-sm font-medium">{$_('account.smtpAuthentication')}</h4>
      <label class="flex items-center gap-3 text-sm">
        <Switch
          checked={s.smtpUseSameAsIncoming}
          onCheckedChange={(v) => handleSmtpUseSameAsIncomingChange(s, v)}
        />
        <span>{$_('account.smtpUseSameAsIncoming')}</span>
      </label>
      {#if !s.smtpUseSameAsIncoming}
        <div class="grid grid-cols-2 gap-3">
          <div class="space-y-2">
            <Label for="wizardSmtpUsername">{$_('account.username')}</Label>
            <Input
              id="wizardSmtpUsername"
              type="text"
              placeholder={$_('account.smtpUsernamePlaceholder')}
              bind:value={s.smtpUsername}
              class={s.errors.smtpUsername ? 'border-destructive' : ''}
            />
            {#if s.errors.smtpUsername}
              <p class="text-sm text-destructive">{s.errors.smtpUsername}</p>
            {/if}
          </div>
          <div class="space-y-2">
            <Label for="wizardSmtpPassword">{$_('account.password')}</Label>
            <Input
              id="wizardSmtpPassword"
              type="password"
              placeholder={$_('account.smtpPasswordPlaceholder')}
              bind:value={s.smtpPassword}
              class={s.errors.smtpPassword ? 'border-destructive' : ''}
            />
            {#if s.errors.smtpPassword}
              <p class="text-sm text-destructive">{s.errors.smtpPassword}</p>
            {/if}
          </div>
          <div class="space-y-2">
            <Label>{$_('account.authMechanism')}</Label>
            <Select.Root bind:value={s.smtpAuthMechanism}>
              <Select.Trigger class="h-10">
                <Select.Value placeholder="Select">
                  {getAuthMechanismLabel(s.smtpAuthMechanism)}
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
{/if}
