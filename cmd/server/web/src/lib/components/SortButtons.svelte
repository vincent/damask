<script lang="ts">
  import { SortAsc, SortDesc } from '@lucide/svelte'
  type Props = {
    sort?: (key: string, asc: boolean) => void
    keys: Record<string, string>
    value: string
    asc: boolean
  }
  let { value = $bindable(), asc = $bindable(), keys, sort }: Props = $props()

  function onSort(key: string) {
    asc = key === value ? !asc : asc
    value = key
    sort?.(key, asc)
  }
</script>

{#each Object.entries(keys) as [key, label] (key)}
  {@const active = value === key}
  <button
    type="button"
    class="flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm transition-colors {active
      ? 'border border-[var(--border-default)] bg-[var(--bg-elevated)] font-medium text-[var(--text-primary)]'
      : 'border border-transparent text-[var(--text-muted)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-secondary)]'}"
    onclick={() => onSort(key)}
    title={label}
  >
    <span
      class="sort-icon flex h-3.5 w-3.5 shrink-0 items-center justify-center"
    >
      {#if active && asc}
        <SortAsc class="h-3.5 w-3.5 text-[var(--accent)]" />
      {:else if active && !asc}
        <SortDesc class="h-3.5 w-3.5 text-[var(--accent)]" />
      {:else}
        <SortAsc class="ghost-icon h-3.5 w-3.5 text-[var(--text-muted)]" />
      {/if}
    </span>
    {label}
  </button>
{/each}

<style>
  button :global(.ghost-icon) {
    opacity: 0;
    transition: opacity 150ms ease-out;
  }
  button:hover :global(.ghost-icon) {
    opacity: 0.4;
  }
</style>
