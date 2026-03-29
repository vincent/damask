<script lang="ts">
    import { assetsStore } from "$lib/stores/assets.svelte"
  import { ZoomIn, ZoomOut } from "@lucide/svelte"

    let { zoom = $bindable<number>() } = $props();
</script>

<div class="absolute z-10 bottom-0 right-0 left-0 flex items-center justify-between border-t border-gray-200 bg-white px-4 py-3">
    <p class="text-sm text-gray-500">
        {assetsStore.assets.length} assets
    </p>
    <p class="text-sm text-gray-500">
        <ZoomIn class="h-4 w-4 cursor-pointer" onclick={() => zoom = zoom * 1.25} />
        ---
        <ZoomOut class="h-4 w-4 cursor-pointer" onclick={() => zoom = zoom * 0.25} />
    </p>
</div>
