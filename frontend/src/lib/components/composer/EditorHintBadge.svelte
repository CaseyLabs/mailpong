<script lang="ts">
  interface Props {
    show: boolean
    label: string
  }

  let { show, label }: Props = $props()
</script>

{#if show}
  <span class="absolute -top-1 -left-1 bg-primary text-primary-foreground text-2xs font-bold w-4 h-4 flex items-center justify-center rounded z-20">{label}</span>
{/if}
