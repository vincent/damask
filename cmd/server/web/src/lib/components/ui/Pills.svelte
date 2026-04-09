<script lang="ts">

    type Pill = { id: string, label: string }
    type Props = {
        pills: Pill[]
        active: string
        set: (p: Pill) => void
    }
    let { pills, active, set }: Props = $props()
</script>

<div class="flex flex-wrap gap-2 border-b border-gray-100 px-5 py-3 dark:border-gray-800">
    {#each pills as p}
        <button
        type="button"
        class="rounded-full px-3 py-1 text-sm font-medium transition-colors {active === p.id
            ? 'bg-indigo-600 text-white dark:bg-indigo-500'
            : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-800 dark:text-gray-400 dark:hover:bg-gray-700'}"
        onclick={() => set(p)}
        >
        {p.label}
        </button>
    {/each}
</div>
