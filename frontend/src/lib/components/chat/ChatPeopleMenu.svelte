<script lang="ts">
  // ChatPeopleMenu — the open chat's title as a button that lists every
  // participant with their address, like a messenger's chat details.
  // Choosing someone starts a new message to them; "Copy addresses" copies
  // them all.
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import * as DropdownMenu from '$lib/components/ui/dropdown-menu'
  import Avatar from '$lib/components/kit/Avatar.svelte'
  import { contactPhotos } from '$lib/stores/contactPhotos.svelte'
  import { copyText } from './chatClipboard'
  import { displayName, formatAddress, type Person } from './chatFormat'

  interface Props {
    people: Person[]
    title: string
    onComposeTo?: (to: string) => void
  }

  let { people, title, onComposeTo }: Props = $props()

  const addresses = $derived(people.filter((p) => p.email).map(formatAddress))

  const copyAll = () => copyText(addresses.join(', '), $_('chat.addresses'))
</script>

<DropdownMenu.Root>
  <DropdownMenu.Trigger
    class="min-w-0 max-w-full px-1 rounded-md text-body font-semibold text-fg-primary truncate hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-hidden"
    title={$_('chat.showPeople')}
  >
    {title}
  </DropdownMenu.Trigger>
  <DropdownMenu.Content align="center" class="max-h-[60vh] max-w-[min(24rem,90vw)] overflow-y-auto">
    {#each people as p (p.email || p.name)}
      <DropdownMenu.Item disabled={!onComposeTo || !p.email} onSelect={() => onComposeTo?.(p.email)}>
        <Avatar email={p.email} name={p.name} size={24} photoData={contactPhotos.get(p.email)?.data} photoMediaType={contactPhotos.get(p.email)?.mediaType} />
        <span class="ml-2 min-w-0 flex flex-col">
          <span class="truncate">{displayName(p)}</span>
          {#if p.email}<span class="truncate text-xs text-muted-foreground">{p.email}</span>{/if}
        </span>
      </DropdownMenu.Item>
    {/each}
    {#if addresses.length}
      <DropdownMenu.Separator />
      <DropdownMenu.Item onSelect={() => void copyAll()}>
        <Icon icon="mdi:content-copy" class="mr-2 h-4 w-4" />
        {$_('chat.copyAddresses')}
      </DropdownMenu.Item>
    {/if}
  </DropdownMenu.Content>
</DropdownMenu.Root>
