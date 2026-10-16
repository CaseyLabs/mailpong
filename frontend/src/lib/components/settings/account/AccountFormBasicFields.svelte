<script lang="ts">
  import { Input } from '$lib/components/ui/input'
  import { Label } from '$lib/components/ui/label'
  import { ColorPicker } from '$lib/components/ui/color-picker'
  import { getOAuthProviderType } from '$lib/config/providers'
  import { chooseMicrosoftAccountType } from './accountFormOAuth'
  import AccountFormAuth from './AccountFormAuth.svelte'
  import type { AccountFormState } from './accountFormState.svelte'
  import { _ } from '$lib/i18n'

  let { s }: { s: AccountFormState } = $props()
</script>
      <!-- Basic Fields -->
      <div class="grid gap-4">
        <div class="space-y-2">
          <Label for="name">{$_('account.accountName')}</Label>
          <div class="flex items-center gap-3">
            <ColorPicker value={s.color} onchange={(c) => s.color = c} />
            <Input
              id="name"
              type="text"
              placeholder={$_('account.accountNamePlaceholder')}
              bind:value={s.name}
              class={s.errors.name ? 'border-destructive' : ''}
            />
          </div>
          <p class="text-xs text-muted-foreground">
            {$_('account.colorHelp')}
          </p>
          {#if s.errors.name}
            <p class="text-sm text-destructive">{s.errors.name}</p>
          {/if}
        </div>

        <div class="space-y-2">
          <Label for="displayName">{$_('account.displayName')}</Label>
          <Input
            id="displayName"
            type="text"
            placeholder={$_('account.displayNamePlaceholder')}
            bind:value={s.displayName}
            class={s.errors.displayName ? 'border-destructive' : ''}
          />
          <p class="text-xs text-muted-foreground">
            {$_('account.displayNameHelp')}
          </p>
          {#if s.errors.displayName}
            <p class="text-sm text-destructive">{s.errors.displayName}</p>
          {/if}
        </div>

        <div class="space-y-2">
          <Label for="email">{$_('account.emailAddress')}</Label>
          <Input
            id="email"
            type="email"
            placeholder="you@example.com"
            bind:value={s.email}
            class={s.errors.email ? 'border-destructive' : ''}
          />
          {#if s.errors.email}
            <p class="text-sm text-destructive">{s.errors.email}</p>
          {/if}
        </div>

        <div class="space-y-2">
          <Label for="username">{$_('account.username')}</Label>
          <Input
            id="username"
            type="text"
            placeholder={$_('account.usernamePlaceholder')}
            bind:value={s.username}
          />
          <p class="text-xs text-muted-foreground">
            {$_('account.usernameHelp')}
          </p>
        </div>

        {#if !s.editAccount && s.selectedProvider && getOAuthProviderType(s.selectedProvider) === 'microsoft'}
          <div class="space-y-2">
            <Label for="microsoft-account-type">{$_('account.microsoftAccountType')}</Label>
            <select id="microsoft-account-type" class="w-full rounded-md border border-input bg-background p-2 text-sm" value={s.microsoftAccountType} onchange={(event) => chooseMicrosoftAccountType(s, event.currentTarget.value)}>
              <option value="">{$_('account.microsoftChooseType')}</option>
              <option value="personal">{$_('account.microsoftPersonal')}</option>
              <option value="business">{$_('account.microsoftBusiness')}</option>
            </select>
            <p class="text-xs text-muted-foreground">{$_('account.microsoftSetupHelp')}</p>
          </div>
        {/if}

<AccountFormAuth {s} />
      </div>
