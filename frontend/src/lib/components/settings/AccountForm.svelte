<script lang="ts">
  import Icon from '@iconify/svelte'
  // @ts-ignore - wailsjs path
  import type { account } from '../../../../wailsjs/go/models'
  import { _ } from '$lib/i18n'
  import CertificateDialog from './CertificateDialog.svelte'
  import { AccountFormState } from './account/accountFormState.svelte'
  import { registerAccountFormEffects } from './account/accountFormEffects.svelte'
  import { goBackToProviders } from './account/accountFormLoad'
  import { handleSubmit, handleCertAcceptOnce, handleCertAcceptPermanently, handleCertDecline } from './account/accountFormSubmit'
  import type { OAuthCredentials as FormOAuthCredentials } from './account/accountFormTypes'
  import AccountFormProviderStep from './account/AccountFormProviderStep.svelte'
  import AccountFormBasicFields from './account/AccountFormBasicFields.svelte'
  import AccountFormAdvanced from './account/AccountFormAdvanced.svelte'
  import AccountFormActions from './account/AccountFormActions.svelte'

  // OAuth credentials to pass to parent
  export type OAuthCredentials = FormOAuthCredentials

  interface Props {
    /** Account to edit (null for new account) */
    editAccount?: account.Account | null
    /** Callback when form is submitted successfully */
    onSubmit?: (config: account.AccountConfig, oauthCredentials?: OAuthCredentials) => Promise<void>
    /** Callback when form is cancelled */
    onCancel?: () => void
    /** Callback for testing connection */
    onTestConnection?: (config: account.AccountConfig) => Promise<void>
  }

  let {
    editAccount = null,
    onSubmit,
    onCancel,
    onTestConnection: _onTestConnection,
  }: Props = $props()

  // The form's state and the props it reads, shared with the accountForm*.ts modules
  const s = new AccountFormState({
    get editAccount() { return editAccount },
    get onSubmit() { return onSubmit },
    get onCancel() { return onCancel },
  })

  registerAccountFormEffects(s)
</script>

<form onsubmit={(e) => handleSubmit(s, e)} class="space-y-6">
  {#if s.step === 'provider' && !s.editAccount}
    <!-- Step 1: Provider Selection -->
    <AccountFormProviderStep {s} />
  {:else}
    <!-- Step 2: Account Details -->
    <div class="space-y-4">
      {#if !s.editAccount}
        <button
          type="button"
          class="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors"
          onclick={() => goBackToProviders(s)}
        >
          <Icon icon="mdi:arrow-left" class="w-4 h-4" />
          {$_('account.changeProvider')}
        </button>
      {/if}

      {#if s.selectedProvider?.notes}
        <div class="flex items-start gap-2 p-3 rounded-lg bg-amber-500/10 border border-amber-500/20">
          <Icon icon="mdi:information-outline" class="w-5 h-5 text-amber-500 shrink-0 mt-0.5" />
          <p class="text-sm text-amber-600 dark:text-amber-400">
            {s.selectedProvider.notesKey ? $_(s.selectedProvider.notesKey) : s.selectedProvider.notes}
          </p>
        </div>
      {/if}

      <!-- Basic Fields -->
      <AccountFormBasicFields {s} />

      <!-- Advanced Settings Toggle -->
      <button
        type="button"
        class="flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
        onclick={() => (s.showAdvanced = !s.showAdvanced)}
      >
        <Icon
          icon={s.showAdvanced ? 'mdi:chevron-down' : 'mdi:chevron-right'}
          class="w-4 h-4"
        />
        {$_('account.advancedSettings')}
      </button>

      <AccountFormAdvanced {s} />

      <AccountFormActions {s} />
    </div>
  {/if}
</form>

<CertificateDialog
  bind:open={s.showCertDialog}
  certificate={s.pendingCertificate}
  onAcceptOnce={() => handleCertAcceptOnce(s)}
  onAcceptPermanently={() => handleCertAcceptPermanently(s)}
  onDecline={() => handleCertDecline(s)}
/>
