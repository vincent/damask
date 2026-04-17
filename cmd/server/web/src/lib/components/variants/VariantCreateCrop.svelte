<script lang="ts">
    import { authStore } from '$lib/stores/auth.svelte'
    import Button from '$lib/components/ui/Button.svelte'
    import { variantApi, type Asset } from '$lib/api'
  import { m } from '$lib/paraglide/messages'
    
    interface Props {
        asset: Asset
        creating: boolean
        handleCreate: (kind: string, params: Record<string, unknown>) => void
    }

    let { asset, creating, handleCreate }: Props = $props()

    const kind = 'image_crop'

    // Crop params
    let cropX = $state(0)
    let cropY = $state(0)
    let cropWidth = $state(400)
    let cropHeight = $state(400)
    let cropFormat = $state<'jpeg' | 'png'>('jpeg')

</script>

<div class="space-y-5">
    {#if asset.width != null && asset.height != null}
        <p class="text-sm text-gray-400 dark:text-gray-500">Original: {asset.width} × {asset.height} px</p>
    {/if}

    <div class="grid grid-cols-2 gap-4">
        {#each [['X offset', 'cropX'], ['Y offset', 'cropY'], ['Width', 'cropWidth'], ['Height', 'cropHeight']] as [label, key]}
        <div>
            <label for="variant-{kind}-{key}" class="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-400">{label}</label>
            <input id="variant-{kind}-{key}" type="number" min="0"
            value={key === 'cropX' ? cropX : key === 'cropY' ? cropY : key === 'cropWidth' ? cropWidth : cropHeight}
            oninput={(e) => {
                const v = parseInt((e.currentTarget as HTMLInputElement).value) || 0
                if (key === 'cropX') cropX = v
                else if (key === 'cropY') cropY = v
                else if (key === 'cropWidth') cropWidth = v
                else cropHeight = v
            }}
            class="w-full rounded-lg border border-gray-300 px-3 py-2 text-md focus:border-indigo-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100" />
        </div>
        {/each}
    </div>

    <div>
        <label for="variant-{kind}-format" class="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-400">{m.format()}</label>
        <div class="flex gap-2">
            {#each ['jpeg', 'png'] as fmt}
                <button type="button"
                class="flex-1 rounded-lg border py-2 text-sm font-medium transition-colors {cropFormat === fmt
                    ? 'border-indigo-500 bg-indigo-50 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300'
                    : 'border-gray-300 text-gray-600 hover:border-gray-400 dark:border-gray-600 dark:text-gray-400'}"
                onclick={() => { cropFormat = fmt as typeof cropFormat }}
                >{fmt.toUpperCase()}</button>
            {/each}
        </div>
    </div>

    <Button disabled={creating || authStore.role === 'viewer'} onclick={() => handleCreate('crop', { x: cropX, y: cropY, width: cropWidth, height: cropHeight, format: cropFormat, quality: 85 })} class="w-full">
        {creating ? m.queuing_() : m.variant_create_crop()}
    </Button>
</div>
