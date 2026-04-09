<script lang="ts">
    import { File, Play, Music } from '@lucide/svelte'
    import { formatBytes, mimeCategory } from '$lib/api'
    import Badge from '$lib/components/ui/Badge.svelte'
    import { TYPES_BACKGROUNDS } from '$lib/stores/publicView.svelte'

    let { asset, openAsset, thumbUrl } = $props()

    function formatDate(iso: string) {
        return new Date(iso).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
    }
</script>

{#if asset}
    {@const category = mimeCategory(asset.mime_type)}
    <button
        type="button"
        class="group flex w-full flex-col overflow-hidden rounded-xl bg-white text-left shadow-sm transition-shadow hover:shadow-md focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-400 dark:bg-gray-800"
        onclick={() => openAsset?.()}
    >
        <!-- Thumbnail -->
        <div
        class="relative w-full overflow-hidden rounded-t-xl {TYPES_BACKGROUNDS[category]}"
        style="aspect-ratio: 4/3"
        >
        <!-- Category badge -->
        <div class="absolute right-2 top-2 z-10">
            <Badge variant={category} size="sm">
            {category.toUpperCase()}
            </Badge>
        </div>

        {#if (category === 'image' || category === 'video' || category === 'audio' || category === 'document') && thumbUrl}
            <img
            src={thumbUrl}
            data-asset-dynamic-resource={asset.id}
            alt={asset.original_filename}
            class="h-full w-full object-cover"
            loading="lazy"
            onerror={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none' }}
            />
        {:else}
            <div class="flex h-full items-center justify-center">
            <div class="flex h-12 w-12 items-center justify-center rounded-xl bg-white/25">
                {#if category === 'video'}
                    <Play class="h-7 w-7 text-white" />
                {:else if category === 'audio'}
                    <Music class="h-7 w-7 text-white" />
                {:else}
                    <File class="h-7 w-7 text-white" />
                {/if}
            </div>
            </div>
        {/if}
        </div>

        <!-- Info -->
        <div class="flex flex-col gap-1 px-3 py-2.5">
        <p class="truncate text-sm font-medium text-gray-900 dark:text-gray-100" title={asset.original_filename}>
            {asset.original_filename}
        </p>
        <div class="flex items-center justify-between">
            <span class="text-sm text-gray-400">{formatBytes(asset.size)}</span>
            <span class="text-sm text-gray-400">{formatDate(asset.created_at)}</span>
        </div>
        </div>
    </button>
{/if}

