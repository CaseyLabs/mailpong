<script lang="ts">
  import * as Dialog from '$lib/components/ui/dialog'
  import * as Tabs from '$lib/components/ui/tabs'
  import AccountForm, { type OAuthCredentials } from './AccountForm.svelte'
  import AccountGeneralTab from './account/AccountGeneralTab.svelte'
  import AccountIdentityTab from './account/AccountIdentityTab.svelte'
  import AccountServerTab from './account/AccountServerTab.svelte'
  import MailAPISection from './account/MailAPISection.svelte'
  import AccountSecurityTab from './account/AccountSecurityTab.svelte'
  import AccountSetupHooksStep from './AccountSetupHooksStep.svelte'
  import AccountEditTabList from './AccountEditTabList.svelte'
  import AccountEditActions from './AccountEditActions.svelte'
  import { isGenericProviderHost, createAccountFromWizard, reauthorizeAccount } from './accountDialogActions'
  import { loadAccountSetupHooks } from '$lib/stores/extensionRegistry.svelte'
  // @ts-ignore - wailsjs path
  import type { v1 } from '../../../../wailsjs/go/models'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { oauthStore } from '$lib/stores/oauth.svelte'
  import { addToast } from '$lib/stores/toast'
  import { dialogGuardOpen, dialogGuardClose } from '$lib/stores/dialogGuard'
  import { _ } from '$lib/i18n'
  // @ts-ignore - wailsjs path
  import { account, app } from '../../../../wailsjs/go/models'
  // @ts-ignore - wailsjs path
  import { GetIdentities, GetAllAccountIdentities } from '../../../../wailsjs/go/app/App'

  interface Props {
    /** Whether the dialog is open */
    open?: boolean
    /** Account to edit (null for new account) */
    editAccount?: account.Account | null
    /** Callback when dialog should close */
    onClose?: () => void
    /** Callback when account is successfully created/updated */
    onSuccess?: (account: account.Account) => void
  }

  let {
    open = $bindable(false),
    editAccount = null,
    onClose,
    onSuccess,
  }: Props = $props()

  // Tab state (for edit mode)
  let activeTab = $state('general')

  const isGenericProvider = $derived(isGenericProviderHost(editAccount?.imapHost))

  // Form state (for edit mode)
  let name = $state('')
  let displayName = $state('')
  let color = $state('')
  let email = $state('')
  let username = $state('')
  let password = $state('')
  let imapHost = $state('')
  let imapPort = $state(993)
  let imapSecurity = $state('tls')
  let imapAuthMechanism = $state('auto')
  let smtpHost = $state('')
  let smtpPort = $state(587)
  let smtpSecurity = $state('starttls')
  let noOutgoingServer = $state(false)
  let smtpUsername = $state('')
  let smtpPassword = $state('')
  let smtpAuthMechanism = $state('auto')
  let replyForwardIdentityID = $state('')
  let allIdentityGroups = $state<app.AccountIdentityGroup[]>([])
  let syncPeriodDays = $state('30')
  let syncInterval = $state('30')
  let syncAllFolders = $state(false)
  let syncFoldersEnabled = $state(false)
  let readReceiptRequestPolicy = $state('never')
  let authType = $state('password')

  // Folder mappings
  let sentFolderPath = $state('')
  let draftsFolderPath = $state('')
  let trashFolderPath = $state('')
  let spamFolderPath = $state('')
  let archiveFolderPath = $state('')
  let allMailFolderPath = $state('')
  let starredFolderPath = $state('')

  let saving = $state(false)
  let reauthorizing = $state(false)
  let reauthorizeSuccess = $state(false)
  let errors = $state<Record<string, string>>({})
  let initialized = $state(false)

  // Post-account-add hook step state. When an account is successfully created
  // and at least one extension has registered a matching account-setup hook
  // for the provider, the dialog switches from the wizard to a hooks step.
  let hookAccount = $state<account.Account | null>(null)
  let pendingHooks = $state<v1.AccountSetupHookRequest[]>([])
  let resolvedHooks = $state<Set<string>>(new Set())

  // Initialize form when editing
  $effect(() => {
    if (open && editAccount && !initialized) {
      initialized = true
      activeTab = 'general'

      // Load account values
      name = editAccount.name
      email = editAccount.email
      username = editAccount.username
      imapHost = editAccount.imapHost
      imapPort = editAccount.imapPort
      imapSecurity = editAccount.imapSecurity
      imapAuthMechanism = editAccount.imapAuthMechanism || 'auto'
      smtpHost = editAccount.smtpHost
      smtpPort = editAccount.smtpPort
      smtpSecurity = editAccount.smtpSecurity
      noOutgoingServer = editAccount.noOutgoingServer || false
      smtpUsername = editAccount.smtpUsername || ''
      smtpPassword = ''  // never echo a stored password back; blank means "keep existing"
      smtpAuthMechanism = editAccount.smtpAuthMechanism || 'auto'
      replyForwardIdentityID = editAccount.replyForwardIdentityId || ''
      loadAllIdentityGroups()
      syncPeriodDays = String(editAccount.syncPeriodDays)
      syncInterval = String(editAccount.syncInterval ?? 30)
      syncAllFolders = editAccount.syncAllFolders || false
      syncFoldersEnabled = editAccount.syncFoldersEnabled || false
      readReceiptRequestPolicy = editAccount.readReceiptRequestPolicy || 'never'
      authType = editAccount.authType || 'password'
      color = editAccount.color || ''

      // Folder mappings
      sentFolderPath = editAccount.sentFolderPath || ''
      draftsFolderPath = editAccount.draftsFolderPath || ''
      trashFolderPath = editAccount.trashFolderPath || ''
      spamFolderPath = editAccount.spamFolderPath || ''
      archiveFolderPath = editAccount.archiveFolderPath || ''
      allMailFolderPath = editAccount.allMailFolderPath || ''
      starredFolderPath = editAccount.starredFolderPath || ''

      // Load display name from the default identity
      loadDisplayName(editAccount.id)
    }
  })

  // Reset when dialog closes
  $effect(() => {
    if (!open) {
      initialized = false
      errors = {}
      password = ''
      smtpPassword = ''
      allIdentityGroups = []
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

  async function loadDisplayName(accountId: string) {
    try {
      const identities = await GetIdentities(accountId)
      const defaultIdentity = identities?.find((id: any) => id.isDefault) || identities?.[0]
      if (defaultIdentity) {
        displayName = defaultIdentity.name || ''
      }
    } catch (err) {
      console.error('Failed to load display name:', err)
    }
  }

  async function loadAllIdentityGroups() {
    try {
      allIdentityGroups = (await GetAllAccountIdentities()) || []
    } catch (err) {
      console.error('Failed to load identity groups for Reply/Forward-with picker:', err)
      allIdentityGroups = []
    }
  }

  // Sendable identity-group candidates for the Reply/Forward-with picker:
  // exclude the account being edited (its own identities can't reply on
  // its behalf when it's marked no-outgoing) and any other no-outgoing
  // accounts (their identities aren't sendable either).
  const availableIdentityGroups = $derived(
    allIdentityGroups.filter(g => g.account?.id !== editAccount?.id && !g.account?.noOutgoingServer)
  )

  function validate(): boolean {
    errors = {}

    if (!name.trim()) errors.name = $_('account.accountNameRequired')
    if (!displayName.trim()) errors.displayName = $_('account.displayNameRequired')
    if (!imapHost.trim()) errors.imapHost = $_('account.imapHostRequired')
    if (!smtpHost.trim()) errors.smtpHost = $_('account.smtpHostRequired')
    if (imapPort < 1 || imapPort > 65535) errors.imapPort = $_('account.invalidPort')
    if (smtpPort < 1 || smtpPort > 65535) errors.smtpPort = $_('account.invalidPort')

    return Object.keys(errors).length === 0
  }

  async function handleSaveEdit() {
    if (!validate() || !editAccount) return

    saving = true
    try {
      const config = new account.AccountConfig({
        name,
        displayName,
        color,
        email,
        username: username || email,
        password: password, // Empty = keep current
        imapHost,
        imapPort,
        imapSecurity,
        imapAuthMechanism,
        smtpHost,
        smtpPort,
        smtpSecurity,
        noOutgoingServer,
        smtpUsername,
        smtpPassword, // Empty = keep current (when SMTPUsername unchanged) or skip (when toggle is on)
        smtpAuthMechanism,
        replyForwardIdentityId: replyForwardIdentityID,
        authType,
        syncPeriodDays: Number(syncPeriodDays),
        syncInterval: Number(syncInterval),
        syncAllFolders,
        syncFoldersEnabled,
        readReceiptRequestPolicy,
        sentFolderPath,
        draftsFolderPath,
        trashFolderPath,
        spamFolderPath,
        archiveFolderPath,
        allMailFolderPath,
        starredFolderPath,
      })

      const result = await accountStore.updateAccount(editAccount.id, config)

      addToast({
        type: 'success',
        message: $_('toast.accountSaved'),
      })

      onSuccess?.(result)
      open = false
      onClose?.()
    } catch (err) {
      console.error('Failed to save account:', err)
      addToast({
        type: 'error',
        message: String(err).toLowerCase().includes('already exists')
          ? $_('toast.accountEmailExists')
          : $_('toast.failedToSaveAccount'),
      })
    } finally {
      saving = false
    }
  }

  // Handlers for new account wizard (delegated to AccountForm)
  async function handleSubmit(config: account.AccountConfig, oauthCredentials?: OAuthCredentials) {
    const { result, provider } = await createAccountFromWizard(config, oauthCredentials)

    onSuccess?.(result)

    // After successful account creation, check for matching account-setup hooks
    // (e.g., Contacts extension's "Also set up contacts" offer). If any exist
    // and the user has enabled the relevant extensions, switch the dialog to a
    // hooks step. Otherwise close immediately.
    const hooks = await loadAccountSetupHooks(provider)
    if (hooks.length === 0) {
      open = false
      onClose?.()
      return
    }
    hookAccount = result
    pendingHooks = hooks
    resolvedHooks = new Set()
  }

  function resolveHook(extensionId: string) {
    resolvedHooks = new Set([...resolvedHooks, extensionId])
    if (resolvedHooks.size >= pendingHooks.length) {
      // All hooks settled — close.
      hookAccount = null
      pendingHooks = []
      resolvedHooks = new Set()
      open = false
      onClose?.()
    }
  }

  function skipRemainingHooks() {
    hookAccount = null
    pendingHooks = []
    resolvedHooks = new Set()
    open = false
    onClose?.()
  }

  async function handleTestConnection(config: account.AccountConfig) {
    if (config.authType === 'oauth2') {
      return
    }
    await accountStore.testConnection(config)
  }

  function handleCancel() {
    open = false
    onClose?.()
    oauthStore.cancelFlow()
  }

  function handleOpenChange(isOpen: boolean) {
    open = isOpen
    if (!isOpen) {
      onClose?.()
      oauthStore.cancelFlow()
    }
  }

  async function handleReauthorize() {
    if (!editAccount) return

    // Capture account details before async operations (editAccount could become stale)
    await reauthorizeAccount(editAccount.id, editAccount.name, {
      setReauthorizing: (v) => (reauthorizing = v),
      setSuccess: (v) => (reauthorizeSuccess = v),
    })
  }
</script>

<Dialog.Root bind:open onOpenChange={handleOpenChange}>
  <Dialog.Content class="max-w-xl max-h-[90vh] overflow-hidden flex flex-col" preventCloseAutoFocus onInteractOutside={(e) => e.preventDefault()}>
    <Dialog.Header>
      <Dialog.Title>
        {editAccount?.sharedMailboxParentId ? $_('account.editSharedMailboxTitle') : editAccount ? $_('account.editTitle') : $_('account.addTitle')}
      </Dialog.Title>
      <Dialog.Description>
        {editAccount
          ? $_('account.editDescription')
          : $_('account.addDescription')}
      </Dialog.Description>
    </Dialog.Header>

    {#if editAccount}
      <!-- Edit Mode: Tabbed Interface -->
      <Tabs.Root bind:value={activeTab} class="flex-1 flex flex-col overflow-hidden">
        <AccountEditTabList />

        <div class="flex-1 overflow-y-auto mt-4 pl-1 pr-3" style="max-height: calc(90vh - 220px);">
          <Tabs.Content value="general" class="mt-0">
            <AccountGeneralTab
              {editAccount}
              bind:name
              bind:displayName
              bind:color
              bind:email
              bind:username
              bind:password
              bind:syncPeriodDays
              {authType}
              {isGenericProvider}
              {errors}
              {reauthorizing}
              {reauthorizeSuccess}
              onNameChange={(v) => name = v}
              onDisplayNameChange={(v) => displayName = v}
              onColorChange={(v) => color = v}
              onUsernameChange={(v) => username = v}
              onPasswordChange={(v) => password = v}
              onSyncPeriodChange={(v) => syncPeriodDays = v}
              onReauthorize={handleReauthorize}
            />
          </Tabs.Content>

          <Tabs.Content value="identity" class="mt-0">
            <AccountIdentityTab accountId={editAccount.id} {editAccount} />
          </Tabs.Content>

          <Tabs.Content value="security" class="mt-0">
            <AccountSecurityTab accountId={editAccount.id} />
          </Tabs.Content>

          <Tabs.Content value="server" class="mt-0">
            <AccountServerTab
              {editAccount}
              bind:imapHost
              bind:imapPort
              bind:imapSecurity
              bind:imapAuthMechanism
              bind:smtpHost
              bind:smtpPort
              bind:smtpSecurity
              bind:noOutgoingServer
              bind:smtpUsername
              bind:smtpPassword
              bind:smtpAuthMechanism
              bind:replyForwardIdentityID
              {availableIdentityGroups}
              {isGenericProvider}
              bind:syncInterval
              bind:readReceiptRequestPolicy
              bind:sentFolderPath
              bind:draftsFolderPath
              bind:trashFolderPath
              bind:spamFolderPath
              bind:archiveFolderPath
              bind:allMailFolderPath
              bind:starredFolderPath
              {errors}
              onImapHostChange={(v) => imapHost = v}
              onImapPortChange={(v) => imapPort = v}
              onImapSecurityChange={(v) => imapSecurity = v}
              onImapAuthMechanismChange={(v) => imapAuthMechanism = v}
              onSmtpHostChange={(v) => smtpHost = v}
              onSmtpPortChange={(v) => smtpPort = v}
              onSmtpSecurityChange={(v) => smtpSecurity = v}
              onNoOutgoingServerChange={(v) => noOutgoingServer = v}
              onSmtpUsernameChange={(v) => smtpUsername = v}
              onSmtpPasswordChange={(v) => smtpPassword = v}
              onSmtpAuthMechanismChange={(v) => smtpAuthMechanism = v}
              onReplyForwardIdentityIDChange={(v) => replyForwardIdentityID = v}
              onSyncIntervalChange={(v) => syncInterval = v}
              onReadReceiptPolicyChange={(v) => readReceiptRequestPolicy = v}
              bind:syncAllFolders
              onSyncAllFoldersChange={(v) => syncAllFolders = v}
              bind:syncFoldersEnabled
              onSyncFoldersEnabledChange={(v) => syncFoldersEnabled = v}
              onFolderMappingChange={(type, v) => {
                switch (type) {
                  case 'sent': sentFolderPath = v; break
                  case 'drafts': draftsFolderPath = v; break
                  case 'trash': trashFolderPath = v; break
                  case 'spam': spamFolderPath = v; break
                  case 'archive': archiveFolderPath = v; break
                  case 'all': allMailFolderPath = v; break
                  case 'starred': starredFolderPath = v; break
                }
              }}
            />
          {#if editAccount.authType === 'oauth2' || editAccount.sharedMailboxParentId}
              <MailAPISection accountId={editAccount.id} shared={!!editAccount.sharedMailboxParentId} />
            {/if}
          </Tabs.Content>
        </div>

        <AccountEditActions {activeTab} {saving} onCancel={handleCancel} onSave={handleSaveEdit} />
      </Tabs.Root>
    {:else if hookAccount && pendingHooks.length > 0}
      <!-- Post-Add Mode: Account-Setup Hooks -->
      <AccountSetupHooksStep
        {hookAccount}
        {pendingHooks}
        {resolvedHooks}
        onResolve={resolveHook}
        onSkipAll={skipRemainingHooks}
      />
    {:else}
      <!-- New Account Mode: Wizard -->
      <div class="flex-1 overflow-y-auto pl-1 pr-3 pb-4" style="max-height: calc(90vh - 140px);">
        <AccountForm
          {editAccount}
          onSubmit={handleSubmit}
          onTestConnection={handleTestConnection}
          onCancel={handleCancel}
        />
      </div>
    {/if}
  </Dialog.Content>
</Dialog.Root>
