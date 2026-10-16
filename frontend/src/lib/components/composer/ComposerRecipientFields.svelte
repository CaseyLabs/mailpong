<script lang="ts">
  import * as Select from '$lib/components/ui/select'
  import RecipientInput from './RecipientInput.svelte'
  import SendAsWarning from './SendAsWarning.svelte'
  import type { ComposerState } from './composerState.svelte'
  import { _ } from '$lib/i18n'
  import { handleIdentityChange } from './composerIdentity'

  let { s }: { s: ComposerState } = $props()

</script>

<!-- From -->
<div class="flex items-center gap-2 px-4 py-2 border-b border-border">
  <span class="text-sm text-muted-foreground w-16">{$_('composer.from')}:</span>
  <div class="flex-1">
    <Select.Root value={s.selectedIdentityId} onValueChange={(v) => handleIdentityChange(s, v)}>
      <Select.Trigger class="h-8 px-0 border-0 bg-transparent shadow-none focus:ring-0">
        <Select.Value placeholder={$_('composer.selectIdentity')}>
          {#if s.selectedIdentityId}
            {@const identity = s.identities.find(i => i.id === s.selectedIdentityId)}
            {#if identity}
              {@const group = s.allGroups.find(g => g.account?.id === identity.accountId)}
              {#if group?.account?.color}
                <span class="inline-block w-2 h-2 rounded-full mr-1.5 shrink-0" style="background-color: {group.account.color}"></span>
              {/if}
              {identity.name} &lt;{identity.email}&gt;
            {/if}
          {/if}
        </Select.Value>
      </Select.Trigger>
      <Select.Content>
        {#if s.allGroups.length > 0}
          <!-- Cross-account: grouped by account -->
          {#each s.allGroups as group (group.account?.id)}
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
        {:else}
          <!-- Single-account fallback (detached window) -->
          {#each s.identities as identity (identity.id)}
            <Select.Item value={identity.id} label="{identity.name} <{identity.email}>" />
          {/each}
        {/if}
      </Select.Content>
    </Select.Root>
  </div>
  {#if !s.showReplyTo}
    <div class="flex items-center gap-1 text-sm text-muted-foreground">
      <button onclick={() => s.showReplyTo = true} class="hover:text-foreground">{$_('composer.replyTo')}</button>
    </div>
  {/if}
</div>

<SendAsWarning
  api={s.api}
  accountId={s.activeAccountId}
  email={s.identities.find(i => i.id === s.selectedIdentityId)?.email || ''}
/>

<!-- Reply-To (single address; revealed via the From-row link) -->
{#if s.showReplyTo}
  <div class="flex items-start gap-2 px-4 py-2 border-b border-border">
    <span class="text-sm text-muted-foreground w-16 pt-1">{$_('composer.replyTo')}:</span>
    <div class="flex-1">
      <RecipientInput
        bind:recipients={s.replyToRecipients}
        placeholder={$_('composer.addRecipients')}
      />
    </div>
  </div>
{/if}

<!-- To -->
<div class="flex items-start gap-2 px-4 py-2 border-b border-border">
  <span class="text-sm text-muted-foreground w-16 pt-1">{$_('composer.to')}:</span>
  <div class="flex-1">
    <RecipientInput
      bind:this={s.toInputRef}
      bind:recipients={s.toRecipients}
      placeholder={$_('composer.addRecipients')}
    />
  </div>
  {#if !s.showCc || !s.showBcc}
    <div class="flex items-center gap-1 text-sm text-muted-foreground">
      {#if !s.showCc}
        <button onclick={() => s.showCc = true} class="hover:text-foreground">{$_('composer.cc')}</button>
      {/if}
      {#if !s.showBcc}
        <button onclick={() => s.showBcc = true} class="hover:text-foreground">{$_('composer.bcc')}</button>
      {/if}
    </div>
  {/if}
</div>

<!-- Cc -->
{#if s.showCc}
  <div class="flex items-start gap-2 px-4 py-2 border-b border-border">
    <span class="text-sm text-muted-foreground w-16 pt-1">{$_('composer.cc')}:</span>
    <div class="flex-1">
      <RecipientInput
        bind:recipients={s.ccRecipients}
        placeholder={$_('composer.addCcRecipients')}
      />
    </div>
  </div>
{/if}

<!-- Bcc -->
{#if s.showBcc}
  <div class="flex items-start gap-2 px-4 py-2 border-b border-border">
    <span class="text-sm text-muted-foreground w-16 pt-1">{$_('composer.bcc')}:</span>
    <div class="flex-1">
      <RecipientInput
        bind:recipients={s.bccRecipients}
        placeholder={$_('composer.addBccRecipients')}
      />
    </div>
  </div>
{/if}

<!-- Subject -->
<div class="flex items-center gap-2 px-4 py-2 border-b border-border">
  <label for="composer-subject" class="text-sm text-muted-foreground w-16">{$_('composer.subject')}:</label>
  <input
    id="composer-subject"
    bind:value={s.subject}
    type="text"
    placeholder={$_('composer.subject')}
    class="flex-1 bg-transparent text-sm focus:outline-hidden"
    onkeydown={(e) => {
      // Tab skips security rows + toolbar and goes directly to body
      if (e.key === 'Tab' && !e.shiftKey) {
        e.preventDefault()
        s.editor?.commands.focus('start')
      }
    }}
  />
</div>
