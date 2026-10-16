<script lang="ts">
  import Icon from '@iconify/svelte'
  import { Button } from '$lib/components/ui/button'
  import { Input } from '$lib/components/ui/input'
  import { Label } from '$lib/components/ui/label'
  import GoogleSetupGuide from '../GoogleSetupGuide.svelte'
  import { oauthStore } from '$lib/stores/oauth.svelte'
  import {
    allowsPasswordFallback,
    supportsCustomOAuth,
    getOAuthProviderType,
    type EmailProvider,
  } from '$lib/config/providers'
  import { canUseOAuth, startOAuthFlow, cancelOAuthFlow, handleCopyOAuthLink, discoverCustomOAuth, getOAuthButtonIcon } from './accountFormOAuth'
  import type { AccountFormState } from './accountFormState.svelte'
  import { _ } from '$lib/i18n'

  let { s }: { s: AccountFormState } = $props()

  // Get OAuth button text based on provider
  function getOAuthButtonText(provider: EmailProvider | null): string {
    if (!provider) return $_('account.signIn')
    const oauthType = getOAuthProviderType(provider)
    if (oauthType === 'google') return $_('account.signInWith', { values: { provider: 'Google' } })
    if (oauthType === 'microsoft') return $_('account.signInWith', { values: { provider: 'Microsoft' } })
    return $_('account.signIn')
  }
</script>
<!-- Authentication Section -->
<div class="space-y-3">
  <!-- Shared OAuth flow status (idle/pending/success/error). Used by both the
       shipped-provider block and the custom ("bring your own app") block.
       signInDisabled gates the initial sign-in button (e.g. until the custom
       endpoint/client fields are filled). -->
  {#snippet oauthFlowStatus(signInDisabled = false)}
    <div class="rounded-lg border border-border p-4 space-y-3">
      {#if oauthStore.flowState === 'idle' || oauthStore.flowState === 'cancelled'}
        <!-- Initial state - show sign in button -->
        <Button
          type="button"
          variant="outline"
          class="w-full h-12"
          disabled={signInDisabled}
          onclick={() => startOAuthFlow(s)}
        >
          <Icon icon={getOAuthButtonIcon(s.selectedProvider)} class="w-5 h-5 mr-3" />
          {getOAuthButtonText(s.selectedProvider)}
        </Button>
        <p class="text-xs text-muted-foreground text-center">
          {$_('account.redirectToSignIn')}
        </p>
      {:else if oauthStore.flowState === 'pending'}
        <!-- Waiting for OAuth callback -->
        <div class="flex flex-col items-center gap-3 py-2">
          <Icon icon="mdi:loading" class="w-8 h-8 animate-spin text-primary" />
          <div class="text-center">
            <p class="text-sm font-medium">{$_('account.waitingForAuth')}</p>
            <p class="text-xs text-muted-foreground mt-1">
              {$_('account.completeSignIn')}
            </p>
          </div>
          {#if oauthStore.authURL}
            <button
              type="button"
              class="text-xs text-muted-foreground hover:text-foreground inline-flex items-center gap-1.5 transition-colors"
              onclick={() => handleCopyOAuthLink(s)}
            >
              {s.oauthLinkCopied ? $_('account.linkCopied') : $_('viewer.copyLink')}
              <Icon icon={s.oauthLinkCopied ? 'mdi:check' : 'mdi:content-copy'} class="w-3.5 h-3.5" />
            </button>
          {/if}
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onclick={() => cancelOAuthFlow()}
          >
            {$_('common.cancel')}
          </Button>
        </div>
      {:else if oauthStore.flowState === 'success'}
        <!-- OAuth completed successfully -->
        <div class="flex items-center gap-3 py-2">
          <div class="shrink-0 w-10 h-10 rounded-full bg-green-500/10 flex items-center justify-center">
            <Icon icon="mdi:check" class="w-5 h-5 text-green-500" />
          </div>
          <div class="flex-1 min-w-0">
            <p class="text-sm font-medium text-green-600 dark:text-green-400">
              {$_('account.connectedSuccessfully')}
            </p>
            <p class="text-xs text-muted-foreground truncate">
              {oauthStore.flowResult?.email}
            </p>
          </div>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onclick={() => {
              oauthStore.reset()
            }}
          >
            <Icon icon="mdi:refresh" class="w-4 h-4" />
          </Button>
        </div>
      {:else if oauthStore.flowState === 'error'}
        <!-- OAuth failed -->
        <div class="space-y-3">
          <div class="flex items-start gap-3">
            <div class="shrink-0 w-10 h-10 rounded-full bg-destructive/10 flex items-center justify-center">
              <Icon icon="mdi:alert" class="w-5 h-5 text-destructive" />
            </div>
            <div class="flex-1">
              <p class="text-sm font-medium text-destructive">
                {$_('account.authFailed')}
              </p>
              <p class="text-xs text-muted-foreground mt-1">
                {oauthStore.flowError || $_('account.authFailed')}
              </p>
            </div>
          </div>
          <Button
            type="button"
            variant="outline"
            size="sm"
            class="w-full"
            disabled={signInDisabled}
            onclick={() => startOAuthFlow(s)}
          >
            {$_('account.tryAgain')}
          </Button>
        </div>
      {/if}
    </div>
    {#if s.errors.oauth}
      <p class="text-sm text-destructive">{s.errors.oauth}</p>
    {/if}
  {/snippet}

  {#if canUseOAuth(s, s.selectedProvider) && !s.editAccount}
    <!-- OAuth Provider - Show Sign In Button -->
    <div class="space-y-3">
      <Label>{$_('account.authentication')}</Label>

      {#if allowsPasswordFallback(s.selectedProvider!)}
        <!-- Provider allows both OAuth and password -->
        <div class="flex gap-2">
          <Button
            type="button"
            variant={s.authMethod === 'oauth2' ? 'default' : 'outline'}
            size="sm"
            onclick={() => s.authMethod = 'oauth2'}
            class="flex-1"
          >
            <Icon icon={getOAuthButtonIcon(s.selectedProvider)} class="w-4 h-4 mr-2" />
            OAuth
          </Button>
          <Button
            type="button"
            variant={s.authMethod === 'password' ? 'default' : 'outline'}
            size="sm"
            onclick={() => s.authMethod = 'password'}
            class="flex-1"
          >
            <Icon icon="mdi:key" class="w-4 h-4 mr-2" />
            {$_('account.appPassword')}
          </Button>
        </div>
      {/if}

      {#if s.authMethod === 'oauth2'}
        {@render oauthFlowStatus(getOAuthProviderType(s.selectedProvider!) === 'microsoft' && !s.microsoftAccountType)}
        {#if getOAuthProviderType(s.selectedProvider!) === 'google'}
          <GoogleSetupGuide />
        {/if}
      {:else}
        <!-- Password field for app password -->
        <div class="space-y-2">
          <Label for="password">{$_('account.appPassword')}</Label>
          <Input
            id="password"
            type="password"
            placeholder={$_('account.enterAppPassword')}
            bind:value={s.password}
            class={s.errors.password ? 'border-destructive' : ''}
          />
          {#if s.errors.password}
            <p class="text-sm text-destructive">{s.errors.password}</p>
          {/if}
        </div>
      {/if}
    </div>
  {:else if supportsCustomOAuth(s.selectedProvider) && !s.editAccount}
    <!-- Generic provider: Password or user-supplied ("bring your own app") OAuth -->
    <div class="space-y-3">
      <Label>{$_('account.authentication')}</Label>

      <div class="flex gap-2">
        <Button
          type="button"
          variant={s.authMethod === 'password' ? 'default' : 'outline'}
          size="sm"
          onclick={() => s.authMethod = 'password'}
          class="flex-1"
        >
          <Icon icon="mdi:key" class="w-4 h-4 mr-2" />
          {$_('account.password')}
        </Button>
        <Button
          type="button"
          variant={s.authMethod === 'oauth2' ? 'default' : 'outline'}
          size="sm"
          onclick={() => s.authMethod = 'oauth2'}
          class="flex-1"
        >
          <Icon icon="mdi:shield-key-outline" class="w-4 h-4 mr-2" />
          OAuth
        </Button>
      </div>

      {#if s.authMethod === 'oauth2'}
        <!-- Issuer + client credentials; OIDC discovery resolves the endpoints -->
        <div class="rounded-lg border border-border p-4 space-y-3">
          <p class="text-xs text-muted-foreground">{$_('account.customOAuthHelp')}</p>

          <div class="space-y-2">
            <Label for="custom-oauth-issuer">{$_('account.customOAuthIssuer')}</Label>
            <div class="flex gap-2">
              <Input
                id="custom-oauth-issuer"
                type="url"
                placeholder="https://mail.example.com"
                bind:value={s.customOAuthIssuer}
                disabled={s.customOAuthInputsLocked}
              />
              <Button
                type="button"
                variant="outline"
                size="sm"
                onclick={() => discoverCustomOAuth(s)}
                disabled={!s.customOAuthIssuer.trim() || s.customOAuthDiscovering || s.customOAuthInputsLocked}
              >
                {#if s.customOAuthDiscovering}
                  <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
                {:else}
                  {$_('account.customOAuthDiscover')}
                {/if}
              </Button>
            </div>
            <p class="text-xs text-muted-foreground">{$_('account.customOAuthIssuerHelp')}</p>
            {#if s.customOAuthDiscoverError}
              <p class="text-xs text-destructive">{s.customOAuthDiscoverError}</p>
            {:else if s.customOAuthDiscovered}
              <p class="text-xs text-green-600 dark:text-green-400 inline-flex items-center gap-1">
                <Icon icon="mdi:check" class="w-3.5 h-3.5" />
                {$_('account.customOAuthDiscovered')}
              </p>
            {/if}
          </div>

          <div class="space-y-2">
            <Label for="custom-oauth-client-id">{$_('account.customOAuthClientId')}</Label>
            <Input
              id="custom-oauth-client-id"
              type="text"
              bind:value={s.customOAuthClientID}
              disabled={s.customOAuthInputsLocked}
            />
          </div>
          <div class="space-y-2">
            <Label for="custom-oauth-client-secret">{$_('account.customOAuthClientSecret')}</Label>
            <Input
              id="custom-oauth-client-secret"
              type="password"
              placeholder={$_('account.customOAuthClientSecretPlaceholder')}
              bind:value={s.customOAuthClientSecret}
              disabled={s.customOAuthInputsLocked}
            />
          </div>
          <div class="space-y-2">
            <Label for="custom-oauth-scopes">{$_('account.customOAuthScopes')}</Label>
            <Input
              id="custom-oauth-scopes"
              type="text"
              placeholder="offline_access"
              bind:value={s.customOAuthScopes}
              disabled={s.customOAuthInputsLocked}
            />
            <p class="text-xs text-muted-foreground">{$_('account.customOAuthScopesHelp')}</p>
          </div>

          <!-- Redirect URI the user registers in their OAuth app -->
          <div class="space-y-1">
            <Label>{$_('account.customOAuthRedirectUri')}</Label>
            <code class="block text-xs bg-muted rounded px-2 py-1.5 break-all">{s.customOAuthRedirectURI}</code>
            <p class="text-xs text-muted-foreground">{$_('account.customOAuthRedirectUriHelp')}</p>
          </div>

          <!-- Advanced: manual endpoint entry (fallback for servers without discovery) -->
          <button
            type="button"
            class="text-xs text-muted-foreground hover:text-foreground inline-flex items-center gap-1 transition-colors"
            onclick={() => s.customOAuthManual = !s.customOAuthManual}
          >
            <Icon icon={s.customOAuthManual ? 'mdi:chevron-down' : 'mdi:chevron-right'} class="w-4 h-4" />
            {$_('account.customOAuthManual')}
          </button>
          {#if s.customOAuthManual}
            <div class="space-y-2">
              <Label for="custom-oauth-auth-url">{$_('account.customOAuthAuthUrl')}</Label>
              <Input
                id="custom-oauth-auth-url"
                type="url"
                placeholder="https://auth.example.com/authorize"
                bind:value={s.customOAuthAuthURL}
                disabled={s.customOAuthInputsLocked}
              />
            </div>
            <div class="space-y-2">
              <Label for="custom-oauth-token-url">{$_('account.customOAuthTokenUrl')}</Label>
              <Input
                id="custom-oauth-token-url"
                type="url"
                placeholder="https://auth.example.com/token"
                bind:value={s.customOAuthTokenURL}
                disabled={s.customOAuthInputsLocked}
              />
            </div>
            <div class="space-y-2">
              <Label for="custom-oauth-userinfo-url">{$_('account.customOAuthUserinfoUrl')}</Label>
              <Input
                id="custom-oauth-userinfo-url"
                type="url"
                placeholder="https://auth.example.com/userinfo"
                bind:value={s.customOAuthUserinfoURL}
                disabled={s.customOAuthInputsLocked}
              />
            </div>
          {/if}
        </div>

        {@render oauthFlowStatus(!s.customOAuthReady)}
      {:else}
        <!-- Password field -->
        <div class="space-y-2">
          <Label for="password">{$_('account.password')}</Label>
          <Input
            id="password"
            type="password"
            placeholder={$_('account.enterPassword')}
            bind:value={s.password}
            class={s.errors.password ? 'border-destructive' : ''}
          />
          {#if s.errors.password}
            <p class="text-sm text-destructive">{s.errors.password}</p>
          {/if}
        </div>
      {/if}
    </div>
  {:else if s.editAccount && s.editAccount.authType === 'oauth2'}
    <!-- Editing an OAuth account -->
    <div class="space-y-2">
      <Label>{$_('account.authentication')}</Label>
      <div class="rounded-lg border border-border p-4">
        <div class="flex items-center gap-3">
          <div class="shrink-0 w-10 h-10 rounded-full bg-primary/10 flex items-center justify-center">
            <Icon icon={getOAuthButtonIcon(s.selectedProvider)} class="w-5 h-5" />
          </div>
          <div class="flex-1">
            <p class="text-sm font-medium">{$_('account.oauthConnected')}</p>
            <p class="text-xs text-muted-foreground">
              {$_('account.signInAgainHelp')}
            </p>
          </div>
        </div>
      </div>
    </div>
  {:else}
    <!-- Standard password field -->
    <div class="space-y-2">
      <Label for="password">
        {s.selectedProvider?.notes?.includes('App Password') ? $_('account.appPassword') : $_('account.password')}
      </Label>
      <Input
        id="password"
        type="password"
        placeholder={s.editAccount ? $_('account.leaveEmptyToKeep') : $_('account.password')}
        bind:value={s.password}
        class={s.errors.password ? 'border-destructive' : ''}
      />
      {#if s.errors.password}
        <p class="text-sm text-destructive">{s.errors.password}</p>
      {/if}
    </div>
  {/if}
</div>
