<script lang="ts">
    import { assetsStore } from "$lib/stores/assets.svelte"
    import { ZoomIn, ZoomOut } from "@lucide/svelte"

    let { zoom = $bindable<number>(), max = 20 } = $props();
</script>

<div class="absolute z-10 bottom-0 right-0 left-0 flex items-center justify-between border-t border-gray-200 bg-white px-4 py-3 dark:border-gray-800 dark:bg-gray-900">
    <p class="text-sm text-gray-500 dark:text-gray-400">
        {assetsStore.assets.length} assets
    </p>
    <div class="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
        <ZoomOut class="h-4 w-4 cursor-pointer" onclick={() => zoom = Math.max(0, zoom - 2)} />
        <input class="w-full accent-blue-500"  type="range" min="0" {max} bind:value={zoom} />
        <ZoomIn class="h-4 w-4 cursor-pointer" onclick={() => zoom = Math.min(max, zoom + 2)} />
    </div>
</div>
