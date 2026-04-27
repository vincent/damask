<script lang="ts">
    import { authStore } from '$lib/stores/auth.svelte'
    import Button from '$lib/components/ui/Button.svelte'
    import { type Asset } from '$lib/api'
  import { m } from '$lib/paraglide/messages'
  import ResolutionOptions from '../ResolutionOptions.svelte'

    interface Props {
        asset: Asset
        creating: boolean
        handleCreate: (kind: string, params: Record<string, unknown>) => void
    }

    let { asset, creating, handleCreate }: Props = $props()

    const kind = 'image_smartcrop'

    let width = $state(400)
    let height = $state(400)
    let format = $state<'jpeg' | 'png' | 'tiff'>('jpeg')
    let quality = $state(85)

    function useResolution(e: Event) {
        const o = (e.target as HTMLSelectElement)?.selectedOptions;
        if (!o) return;
        const wh = o[0]?.getAttribute('data-wh')?.split('x')
        if (wh?.length === 2) {
            width = +wh[0]
            height = +wh[1]
        }
    }
</script>

<div class="space-y-5">
    {#if asset.width != null && asset.height != null}
        <p class="text-sm text-gray-400 dark:text-gray-500">Original: {asset.width} × {asset.height} px</p>
    {/if}

    <div class="grid grid-cols-3 gap-4">
        <div>
            <label for="variant-{kind}-resolution" class="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-400">{m.resolution()}</label>
            <select id="variant-{kind}-resolution" onchange={useResolution}
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-md focus:border-indigo-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100">
                <ResolutionOptions />
            </select>
        </div>
        <div>
            <label for="variant-{kind}-width" class="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-400">{m.width()} (px)</label>
            <input id="variant-{kind}-width" type="number" min="1" max="8000" bind:value={width}
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-md focus:border-indigo-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100" />
        </div>
        <div>
            <label for="variant-{kind}-height" class="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-400">{m.height()} (px)</label>
            <input id="variant-{kind}-height" type="number" min="1" max="8000" bind:value={height}
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-md focus:border-indigo-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100" />
        </div>
    </div>

    <div>
        <label for="variant-{kind}-format" class="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-400">{m.format()}</label>
        <div class="flex gap-2">
            {#each ['jpeg', 'png', 'tiff'] as fmt}
                <button type="button"
                    class="flex-1 rounded-lg border py-2 text-sm font-medium transition-colors {format === fmt
                        ? 'border-indigo-500 bg-indigo-50 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300'
                        : 'border-gray-300 text-gray-600 hover:border-gray-400 dark:border-gray-600 dark:text-gray-400'}"
                    onclick={() => { format = fmt as typeof format }}
                >{fmt.toUpperCase()}</button>
            {/each}
        </div>
    </div>

    <div>
        <label for="variant-{kind}-quality" class="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-400">{m.quality()}: {quality}%</label>
        <input id="variant-{kind}-quality" type="range" min="1" max="100" bind:value={quality}
            class="w-full accent-indigo-500" />
    </div>

    <Button disabled={creating || authStore.role === 'viewer'} onclick={() => handleCreate(kind, { width, height, format, quality })} class="w-full">
        {creating ? m.queuing_() : m.variant_create_smartcrop()}
    </Button>
</div>
