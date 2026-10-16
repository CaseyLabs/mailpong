<script lang="ts">
  import Icon from '@iconify/svelte'
  import { Input } from '$lib/components/ui/input'
  import { Label } from '$lib/components/ui/label'
  import * as Select from '$lib/components/ui/select'
  import Switch from '$lib/components/ui/switch/Switch.svelte'
  import {
    securityOptions,
    authMechanismOptions,
    syncIntervalOptions,
  } from '$lib/config/providers'
  // @ts-ignore - wailsjs path
  import { account, app } from '../../../../../wailsjs/go/models'
  import { _ } from '$lib/i18n'
  import { getSecurityLabel, getAuthMechanismLabel } from './accountServerLabels'
  import AccountServerSmtp from './AccountServerSmtp.svelte'
  import AccountServerFolderMapping from './AccountServerFolderMapping.svelte'
  import AccountServerFolderSync from './AccountServerFolderSync.svelte'
  import AccountServerTrustedCerts from './AccountServerTrustedCerts.svelte'

  interface Props {
    /** The account being edited */
    editAccount: account.Account
    /** Bound form values */
    imapHost: string
    imapPort: number
    imapSecurity: string
    /** Incoming password auth mechanism: 'auto' | 'plain' | 'login'.
     *  Generic provider only. */
    imapAuthMechanism: string
    smtpHost: string
    smtpPort: number
    smtpSecurity: string
    /** When true, SMTP fields are ignored and the composer hides this
     *  account from the From dropdown. */
    noOutgoingServer: boolean
    /** SMTP-specific username (Generic only). Empty = use IMAP username. */
    smtpUsername: string
    /** SMTP-specific password (write-only; blank on edit means "keep
     *  existing keyring entry"). */
    smtpPassword: string
    /** Outgoing password auth mechanism: 'auto' | 'plain' | 'login'. Only
     *  meaningful with separate SMTP credentials — with "same as incoming"
     *  the backend follows imapAuthMechanism. Generic provider only. */
    smtpAuthMechanism: string
    /** True when the editing account uses the generic provider, so the
     *  separate-SMTP-credentials UI is allowed to render. */
    isGenericProvider: boolean
    /** Identity to pre-select in the composer when replying/forwarding on
     *  a message received via this account. Only consulted when
     *  noOutgoingServer is true. Empty = fall back to default sending
     *  account. */
    replyForwardIdentityID: string
    /** Sendable account+identity groups available as Reply/Forward-with
     *  candidates. Parent excludes the account being edited and any
     *  no-outgoing-server accounts. */
    availableIdentityGroups: app.AccountIdentityGroup[]
    syncInterval: string
    readReceiptRequestPolicy: string
    /** Folder mappings */
    sentFolderPath: string
    draftsFolderPath: string
    trashFolderPath: string
    spamFolderPath: string
    archiveFolderPath: string
    allMailFolderPath: string
    starredFolderPath: string
    /** Validation errors */
    errors: Record<string, string>
    /** Callbacks */
    onImapHostChange: (value: string) => void
    onImapPortChange: (value: number) => void
    onImapSecurityChange: (value: string) => void
    onImapAuthMechanismChange: (value: string) => void
    onSmtpHostChange: (value: string) => void
    onSmtpPortChange: (value: number) => void
    onSmtpSecurityChange: (value: string) => void
    onNoOutgoingServerChange: (value: boolean) => void
    onSmtpUsernameChange: (value: string) => void
    onSmtpPasswordChange: (value: string) => void
    onSmtpAuthMechanismChange: (value: string) => void
    onReplyForwardIdentityIDChange: (value: string) => void
    onSyncIntervalChange: (value: string) => void
    onReadReceiptPolicyChange: (value: string) => void
    onFolderMappingChange: (type: string, value: string) => void
    syncAllFolders: boolean
    onSyncAllFoldersChange: (value: boolean) => void
    syncFoldersEnabled: boolean
    onSyncFoldersEnabledChange: (value: boolean) => void
  }

  let {
    editAccount,
    imapHost = $bindable(),
    imapPort = $bindable(),
    imapSecurity = $bindable(),
    imapAuthMechanism = $bindable('auto'),
    smtpHost = $bindable(),
    smtpPort = $bindable(),
    smtpSecurity = $bindable(),
    noOutgoingServer = $bindable(false),
    smtpUsername = $bindable(''),
    smtpPassword = $bindable(''),
    smtpAuthMechanism = $bindable('auto'),
    isGenericProvider,
    replyForwardIdentityID = $bindable(''),
    availableIdentityGroups,
    syncInterval = $bindable(),
    readReceiptRequestPolicy = $bindable(),
    sentFolderPath = $bindable(),
    draftsFolderPath = $bindable(),
    trashFolderPath = $bindable(),
    spamFolderPath = $bindable(),
    archiveFolderPath = $bindable(),
    allMailFolderPath = $bindable(),
    starredFolderPath = $bindable(),
    errors,
    onImapHostChange,
    onImapPortChange,
    onImapSecurityChange,
    onImapAuthMechanismChange,
    onSmtpHostChange,
    onSmtpPortChange,
    onSmtpSecurityChange,
    onNoOutgoingServerChange,
    onSmtpUsernameChange,
    onSmtpPasswordChange,
    onSmtpAuthMechanismChange,
    onReplyForwardIdentityIDChange,
    onSyncIntervalChange,
    onReadReceiptPolicyChange,
    onFolderMappingChange,
    syncAllFolders = $bindable(),
    onSyncAllFoldersChange,
    syncFoldersEnabled = $bindable(),
    onSyncFoldersEnabledChange,
  }: Props = $props()

  // Read receipt request policy options
  const readReceiptRequestOptions = [
    { value: 'never', labelKey: 'account.neverRequest' },
    { value: 'ask', labelKey: 'account.askEachTime' },
    { value: 'always', labelKey: 'account.alwaysRequest' },
  ]

  function getSyncIntervalLabel(value: string): string {
    const numValue = Number(value)
    const option = syncIntervalOptions.find(opt => opt.value === numValue)
    return option ? $_(option.labelKey) : `${value} min`
  }

  function getReadReceiptLabel(value: string): string {
    switch (value) {
      case 'never': return $_('account.neverRequest')
      case 'ask': return $_('account.askEachTime')
      case 'always': return $_('account.alwaysRequest')
      default: return value
    }
  }
</script>

<div class="space-y-6">
  <!-- Incoming Mail (IMAP) -->
  <div class="space-y-4">
    <h3 class="text-sm font-medium flex items-center gap-2">
      <Icon icon="mdi:email-receive-outline" class="w-4 h-4" />
      {$_('account.incomingMail')}
    </h3>

    <div class="grid grid-cols-2 gap-3">
      <div class="space-y-2">
        <Label for="imapHost">{$_('account.server')}</Label>
        <Input
          id="imapHost"
          type="text"
          placeholder="imap.example.com"
          bind:value={imapHost}
          oninput={(e) => onImapHostChange((e.target as HTMLInputElement).value)}
          class={errors.imapHost ? 'border-destructive' : ''}
        />
        {#if errors.imapHost}
          <p class="text-sm text-destructive">{errors.imapHost}</p>
        {/if}
      </div>
      <div class="grid grid-cols-2 gap-2">
        <div class="space-y-2">
          <Label for="imapPort">{$_('account.port')}</Label>
          <Input
            id="imapPort"
            type="number"
            bind:value={imapPort}
            oninput={(e) => onImapPortChange(Number((e.target as HTMLInputElement).value))}
            class={errors.imapPort ? 'border-destructive' : ''}
          />
        </div>
        <div class="space-y-2">
          <Label>{$_('account.security')}</Label>
          <Select.Root
            value={imapSecurity}
            onValueChange={(v) => { imapSecurity = v; onImapSecurityChange(v) }}
          >
            <Select.Trigger class="h-10">
              <Select.Value placeholder="Select">
                {getSecurityLabel(imapSecurity)}
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
      <div class="grid grid-cols-2 gap-3">
        <div class="space-y-2">
          <Label>{$_('account.authMechanism')}</Label>
          <Select.Root
            value={imapAuthMechanism}
            onValueChange={(v) => { imapAuthMechanism = v; onImapAuthMechanismChange(v) }}
          >
            <Select.Trigger class="h-10">
              <Select.Value placeholder="Select">
                {getAuthMechanismLabel(imapAuthMechanism)}
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

  <!-- Divider -->
  <div class="border-t border-border"></div>

  <!-- "No outgoing server" toggle. When on, the SMTP section + auth
       subsection are hidden, the backend skips SMTP wiring, and the
       composer's From dropdown excludes this account. -->
  <div class="space-y-2">
    <label class="flex items-center gap-3 text-sm">
      <Switch
        checked={noOutgoingServer}
        onCheckedChange={(v) => { noOutgoingServer = v; onNoOutgoingServerChange(v) }}
      />
      <span class="font-medium">{$_('account.noOutgoingServer')}</span>
    </label>
    <p class="text-xs text-muted-foreground">{$_('account.noOutgoingServerHelp')}</p>

    {#if noOutgoingServer}
      <!-- Reply/Forward-with identity picker. Mirrors the composer's
           From-dropdown shape (grouped by sendable account, with color
           dots). Default = empty value, which the composer resolves to
           the user's default sending identity at compose time. -->
      <div class="pt-2 space-y-1">
        <Label>{$_('account.replyForwardWith')}</Label>
        <Select.Root
          value={replyForwardIdentityID}
          onValueChange={(v) => { replyForwardIdentityID = v; onReplyForwardIdentityIDChange(v) }}
        >
          <Select.Trigger class="h-10">
            <Select.Value placeholder={$_('account.replyForwardWithDefault')}>
              {#if replyForwardIdentityID}
                {@const allIdentities = (availableIdentityGroups || []).flatMap(g => (g.identities || []).map(i => ({ identity: i, group: g })))}
                {@const found = allIdentities.find(x => x.identity.id === replyForwardIdentityID)}
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
            {#each availableIdentityGroups || [] as group (group.account?.id)}
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

  {#if !noOutgoingServer}
  <AccountServerSmtp
    {editAccount}
    bind:smtpHost
    bind:smtpPort
    bind:smtpSecurity
    bind:smtpUsername
    bind:smtpPassword
    bind:smtpAuthMechanism
    {isGenericProvider}
    {errors}
    {onSmtpHostChange}
    {onSmtpPortChange}
    {onSmtpSecurityChange}
    {onSmtpUsernameChange}
    {onSmtpPasswordChange}
    {onSmtpAuthMechanismChange}
  />
  {/if}

  <!-- Divider -->
  <div class="border-t border-border"></div>

  <!-- Check for New Mail -->
  <div class="space-y-4">
    <h3 class="text-sm font-medium flex items-center gap-2">
      <Icon icon="mdi:refresh" class="w-4 h-4" />
      {$_('account.syncOptions')}
    </h3>

    <div class="space-y-2">
      <Label>{$_('account.checkNewMail')}</Label>
      <Select.Root
        value={syncInterval}
        onValueChange={(v) => { syncInterval = v; onSyncIntervalChange(v) }}
      >
        <Select.Trigger>
          <Select.Value placeholder="Select">
            {getSyncIntervalLabel(syncInterval)}
          </Select.Value>
        </Select.Trigger>
        <Select.Content>
          {#each syncIntervalOptions as opt (opt.value)}
            <Select.Item value={String(opt.value)} label={$_(opt.labelKey)} />
          {/each}
        </Select.Content>
      </Select.Root>
      <p class="text-xs text-muted-foreground">
        {$_('account.checkNewMailHelp')}
      </p>
    </div>

    <div class="space-y-2">
      <Label>{$_('account.requestReadReceipts')}</Label>
      <Select.Root
        value={readReceiptRequestPolicy}
        onValueChange={(v) => { readReceiptRequestPolicy = v; onReadReceiptPolicyChange(v) }}
      >
        <Select.Trigger>
          <Select.Value placeholder="Select">
            {getReadReceiptLabel(readReceiptRequestPolicy)}
          </Select.Value>
        </Select.Trigger>
        <Select.Content>
          {#each readReceiptRequestOptions as opt (opt.value)}
            <Select.Item value={opt.value} label={$_(opt.labelKey)} />
          {/each}
        </Select.Content>
      </Select.Root>
      <p class="text-xs text-muted-foreground">
        {$_('account.requestReadReceiptsHelp')}
      </p>
    </div>
  </div>

  <!-- Divider -->
  <div class="border-t border-border"></div>

  <AccountServerFolderMapping
    {editAccount}
    bind:sentFolderPath
    bind:draftsFolderPath
    bind:trashFolderPath
    bind:spamFolderPath
    bind:archiveFolderPath
    bind:allMailFolderPath
    bind:starredFolderPath
    {onFolderMappingChange}
  />

  <AccountServerFolderSync
    {editAccount}
    bind:syncAllFolders
    {onSyncAllFoldersChange}
    bind:syncFoldersEnabled
    {onSyncFoldersEnabledChange}
  />

  <AccountServerTrustedCerts />
</div>
