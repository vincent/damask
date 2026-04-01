<script lang="ts">
    import { formatBytes, variantApi, type Asset, type Variant } from '$lib/api'
    import { authStore } from '$lib/stores/auth.svelte'
    import { Download, Trash } from '@lucide/svelte'

    interface Props {
        asset: Asset
        variants: Variant[]
        deleteVariant: (variantId: string) => void
    }

    let { asset, variants, deleteVariant }: Props = $props()

    function variantLabel(v: Variant): string {
        const params = v.transform_params.Valid
            ? (() => { try { return JSON.parse(v.transform_params.String) } catch { return {} } })()
            : {}
            switch (v.type) {
            case 'resize': return `Resize ${params.width ?? '?'}×${params.height ?? '?'}`
            case 'convert': return `Convert → ${(params.format ?? '?').toUpperCase()}`
            case 'crop': return `Crop ${params.width ?? '?'}×${params.height ?? '?'}`
            case 'watermark': return `Watermark ${params.opacity ?? 50}%`
            case 'video_thumbnail': return `Frame at ${params.timestamp ?? 1}s`
            case 'video_transcode': return `Transcode ${params.format ?? 'mp4'}${params.resolution ? ' ' + params.resolution : ''}`
            case 'bg_remove': return 'Background removed'
            default: return v.type
        }
    }

    function variantVersion(index: number) {
        return `v1.${index}`
    }

    function handleDeleteVariant(variantId: string) {
        if (confirm('Are you sure you want to delete this variant?')) {
            deleteVariant(variantId)
        }
    }
</script>

<div class="grid grid-cols-2 gap-3">
    {#each variants as v, i}
        <div class="group flex flex-col overflow-hidden rounded-xl border border-gray-200 dark:border-gray-700">
            <!-- Preview tile -->
            <div class="relative flex h-28 items-center justify-center bg-sky-200/60 dark:bg-sky-900/30">
                <span class="rounded-full bg-white/80 px-2 py-0.5 text-[11px] font-semibold text-gray-700 shadow-sm dark:bg-gray-800/80 dark:text-gray-300">
                    {variantVersion(i)}
                </span>
                <!-- Download overlay -->
                <a
                    href={variantApi.fileUrl(asset.id, v.id)}
                    download
                    class="absolute right-2 top-2 flex h-7 w-7 items-center justify-center rounded-lg bg-white/70 text-gray-600 opacity-0 transition-opacity group-hover:opacity-100 hover:bg-white dark:bg-gray-800/70 dark:text-gray-300"
                    aria-label="Download variant"
                >
                    <Download class="h-3.5 w-3.5" />
                </a>
                {#if authStore.role !== 'viewer'}
                    <button
                        type="button"
                        class="absolute left-2 top-2 flex h-7 w-7 items-center justify-center rounded-lg bg-white/70 text-red-400 opacity-0 transition-opacity group-hover:opacity-100 hover:bg-white dark:bg-gray-800/70"
                        onclick={() => handleDeleteVariant(v.id)}
                        aria-label="Delete variant"
                        >
                        <Trash class="h-3.5 w-3.5" />
                    </button>
                {/if}
            </div>
            <!-- Info -->
            <div class="px-3 py-2.5">
                <p class="truncate text-xs font-semibold text-gray-800 dark:text-gray-200">{variantLabel(v)}</p>
                <p class="mt-0.5 text-[10px] text-gray-400 dark:text-gray-500">
                    {v.size.Valid ? formatBytes(v.size.Int64) + ' · ' : ''}{new Date(v.created_at).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })}
                </p>
            </div>
        </div>
    {/each}
</div>