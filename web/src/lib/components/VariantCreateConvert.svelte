<script lang="ts">
    import { authStore } from '$lib/stores/auth.svelte'
    import Button from '$lib/components/ui/Button.svelte'
    import { variantApi, type Asset } from '$lib/api'
    
    interface Props {
        asset: Asset
        creating: boolean
        handleCreate: (kind: string, params: Record<string, unknown>) => void
    }

    let { asset, creating, handleCreate }: Props = $props()

    const kind = 'image_convert'

    // Convert params
    let convertFormat = $state<'jpeg' | 'png' | 'tiff'>('png')
    let convertQuality = $state(90)

</script>

<div class="space-y-5">
    <div>
        <label for="variant-{kind}-format" class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">Output Format</label>
        <div class="flex gap-2">
        {#each ['jpeg', 'png', 'tiff'] as fmt}
            <button type="button"
            class="flex-1 rounded-lg border py-2.5 text-sm font-medium transition-colors {convertFormat === fmt
                ? 'border-indigo-500 bg-indigo-50 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300'
                : 'border-gray-300 text-gray-600 hover:border-gray-400 dark:border-gray-600 dark:text-gray-400'}"
            onclick={() => { convertFormat = fmt as typeof convertFormat }}
            >{fmt.toUpperCase()}</button>
        {/each}
        </div>
    </div>

    {#if convertFormat === 'jpeg'}
        <div>
            <label for="variant-{kind}-quality" class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">Quality: {convertQuality}%</label>
            <input id="variant-{kind}-quality" type="range" min="1" max="100" bind:value={convertQuality} class="w-full accent-indigo-500" />
        </div>
    {/if}

    <Button disabled={creating || authStore.role === 'viewer'} onclick={() => handleCreate(kind, { format: convertFormat, quality: convertQuality })} class="w-full">
        {creating ? 'Queuing…' : `Convert to ${convertFormat.toUpperCase()}`}
    </Button>
</div>