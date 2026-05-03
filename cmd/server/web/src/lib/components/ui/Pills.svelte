<script lang="ts">
  type Pill = { id: string; label: string }
  type Props = {
    pills: Pill[]
    active: string
    set: (p: Pill) => void
  }
  let { pills, active, set }: Props = $props()
</script>

<div
  class="flex flex-wrap gap-1.5 border-b border-gray-100 px-5 py-2 dark:border-gray-800"
>
  {#each pills as p}
    <button
      type="button"
      class="rounded-full px-3 py-1 text-xs font-medium transition-colors {active ===
      p.id
        ? 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-300'
        : 'bg-gray-100 text-[#8ba4c4] hover:bg-gray-200 hover:text-[#4a6b8a] dark:bg-[#2f3a4f] dark:text-[#5f7590] dark:hover:bg-[#343f56] dark:hover:text-[#8ba4c4]'}"
      onclick={() => set(p)}
    >
      {p.label}
    </button>
  {/each}
</div>
