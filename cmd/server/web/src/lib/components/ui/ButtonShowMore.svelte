<script lang="ts">
    import { ChevronDown } from "@lucide/svelte"
    import Spinner from "./Spinner.svelte"

    let { onclick, loading }: { onclick: () => void; loading: boolean } = $props();
</script>

<div class="border-t border-zinc-100 px-6 py-4 dark:border-zinc-800">
    <button
        type="button"
        class="flex w-full items-center justify-center gap-2 text-md text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
        {onclick}
        disabled={loading}
    >
        {#if loading}
            <Spinner size="sm" />
        {:else}
            <ChevronDown class="h-4 w-4" />
            Load more
        {/if}
    </button>
</div>
