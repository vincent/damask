<script lang="ts">
    import { File, Music, Play } from "@lucide/svelte"

    let { category, asset, thumbUrl } = $props();
</script>

{#if category === 'image'}
    <img
    src={thumbUrl}
    alt={asset.original_filename}
    class="object-cover max-w-3xl"
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
