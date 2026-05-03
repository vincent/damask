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
  <button
    type="button"
    class="text-md flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-gray-600 transition-colors dark:text-gray-400 {value !==
    key
      ? ''
      : 'border border-gray-200 hover:border-indigo-300 hover:bg-indigo-50 hover:text-indigo-700 dark:border-gray-700 dark:hover:border-indigo-700 dark:hover:bg-indigo-900/20 dark:hover:text-indigo-400'}"
    onclick={() => onSort(key)}
    title={label}
  >
    {#if value === key && asc}
      <SortAsc class="me-2 h-4 w-4 text-gray-600 dark:text-gray-400" />
    {:else if value === key && !asc}
      <SortDesc class="me-2 h-4 w-4 text-gray-600 dark:text-gray-400" />
    {/if}
    {label}
  </button>
{/each}
