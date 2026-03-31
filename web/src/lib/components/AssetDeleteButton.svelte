<script lang="ts">
    import { Loader, Trash } from "@lucide/svelte"
    import { assetApi, type Asset } from "$lib/api/client"

    type Props = {
        asset: Asset
        ondeleted: (id: string) => void
    }
    let { asset, ondeleted }: Props = $props()

    let deleting = $state(false)

    async function handleDelete() {
        if (!asset) return
            deleting = true
        try {
            await assetApi.delete(asset.id)
            ondeleted(asset.id)
        } finally {
            deleting = false
        }
    }
</script>

<div>
    <p class="mb-3 text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">Delete</p>
        <button
            class="flex w-full items-center gap-3 rounded-xl border border-gray-200 px-4 py-3 text-sm text-black transition-colors bg-red-500 hover:bg-red-600 dark:border-red-700"
            onclick={handleDelete}
            disabled={deleting}
            aria-label="Delete asset"
            title="Delete"
        >
        {#if deleting}
            <Loader class="h-4 w-4 animate-spin" />
        {:else}
            <Trash class="h-4 w-4" />
        {/if}
        Delete permanently
    </button>
</div>