<script lang="ts">
    import { assetsStore } from "$lib/stores/assets.svelte"
    import { selectionStore } from "$lib/stores/selection.svelte"
    import { ZoomIn, ZoomOut } from "@lucide/svelte"

    let { zoom = $bindable<number>(), max = 20 } = $props();

    const selectionCount = $derived(selectionStore.selectedIds.size)
</script>

<div class="absolute z-10 bottom-0 right-0 left-0 flex items-center justify-between border-t border-gray-200 bg-white px-4 py-3 dark:border-gray-800 dark:bg-gray-900 opacity-[92%]">
    <p class="text-md text-gray-500 dark:text-gray-400">
        {#if selectionCount > 0}
            {selectionCount} selected asset{selectionCount === 1 ? '' : 's'}
        {:else}
            {assetsStore.assets.length} asset{assetsStore.assets.length === 1 ? '' : 's'}
        {/if}
    </p>
    <div class="flex items-center gap-2 text-md text-gray-500 dark:text-gray-400">
        <ZoomOut class="h-4 w-4 cursor-pointer" onclick={() => zoom = Math.max(0, zoom - 2)} />
        <input class="w-full accent-blue-500"  type="range" min="0" {max} bind:value={zoom} />
        <ZoomIn class="h-4 w-4 cursor-pointer" onclick={() => zoom = Math.min(max, zoom + 2)} />
    </div>
</div>
