<script lang="ts">
    import type { Asset, PublicAsset } from "$lib/api"
    import AssetIcon from "./AssetIcon.svelte"

    type Props = {
        asset: Asset | PublicAsset
        category: "document" | "audio" | "video" | "image"
        thumbUrl: string
        assetUrl: string
    }

    let { category, asset, thumbUrl, assetUrl }: Props = $props();
    let isPdf = $derived(asset.mime_type.includes('/pdf'))
    let haveSlashImage = $derived(thumbUrl && (category === 'audio' || (category === 'document' && !isPdf)))
</script>

<div class="min-w-xl max-w-3xl max-h-[80vh]">
    {#if category === 'image'}
        <img
            src={assetUrl}
            alt={asset.original_filename}
            class="object-cover w-full h-full"
            loading="lazy"
            onerror={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none' }}
        />
    {:else if haveSlashImage}
        <img
            src={thumbUrl}
            alt={asset.original_filename}
            class="object-cover min-w-xl  { category === 'audio' ? 'invert' : '' }"
            loading="lazy"
            onerror={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none' }}
        />
    {/if}
    {#if isPdf}
        <iframe class="w-full min-w-3xl min-h-[80vh]" src={assetUrl} title={asset.original_filename}></iframe>
    {:else if category === 'video'}
        <video class="w-full" controls>
            <source src={assetUrl} type={asset.mime_type} />
            Your browser does not support the video tag.
        </video>
    {:else if category === 'audio'}
        <audio class="w-full" controls>
            <source src={assetUrl} type={asset.mime_type} />
            Your browser does not support the audio element.
        </audio>
    {:else if !thumbUrl}
        <div class="flex h-full items-center justify-center">
            <div class="flex h-12 w-12 items-center justify-center rounded-xl bg-white/25">
                <AssetIcon {category} class="h-7 w-7 text-white" />
            </div>
        </div>
    {/if}
</div>