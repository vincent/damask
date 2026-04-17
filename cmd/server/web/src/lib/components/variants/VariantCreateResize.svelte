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

  const kind = 'image_resize'

  // Resize params
  let resizeWidth = $state(800)
  let resizeHeight = $state(0)
  let resizeFit = $state<'contain' | 'cover' | 'fill'>('contain')
  let resizeQuality = $state(85)
  let resizeFormat = $state<'jpeg' | 'png' | 'tiff'>('jpeg')

  // Live preview
  let previewUrl = $state('')
  let previewTimeout: ReturnType<typeof setTimeout>

  const isImage = $derived(asset?.mime_type?.startsWith('image/') ?? false)
  const isVideo = $derived(asset?.mime_type?.startsWith('video/') ?? false)

  function updatePreview() {
    clearTimeout(previewTimeout)
    if (!asset || !isImage) return
    previewTimeout = setTimeout(() => {
      if (!asset) return
      previewUrl = variantApi.previewUrl(asset.id, {
        w: resizeWidth || undefined,
        h: resizeHeight || undefined,
        fit: resizeFit,
        format: resizeFormat,
        q: resizeQuality,
      })
    }, 400)
  }
</script>

<div class="space-y-5">
    {#if previewUrl}
        <div class="flex justify-center rounded-xl border border-gray-200 bg-gray-50 p-3 dark:border-gray-700 dark:bg-gray-800" style="min-height:120px">
            <img src={previewUrl} alt="Preview" class="max-h-48 max-w-full rounded object-contain" />
        </div>
    {:else}
        <div class="flex items-center justify-center rounded-xl border border-dashed border-gray-300 bg-gray-50 py-8 text-sm text-gray-400 dark:border-gray-600 dark:bg-gray-800">
            Preview will appear after changing parameters
        </div>
    {/if}

    <div class="grid grid-cols-2 gap-4">
        <div>
            <label for="variant-{kind}-width" class="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-400">{m.width()} (px)</label>
            <input id="variant-{kind}-width" type="number" min="1" max="8000" bind:value={resizeWidth} oninput={updatePreview}
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-md focus:border-indigo-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100" />
        </div>
        <div>
            <label for="variant-{kind}-height" class="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-400">{m.height()} <span class="text-gray-400">(0=auto)</span></label>
            <input id="variant-{kind}-height" type="number" min="0" max="8000" bind:value={resizeHeight} oninput={updatePreview}
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-md focus:border-indigo-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100" />
        </div>
    </div>

    <div>
        <label for="variant-{kind}-fit" class="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-400">Fit</label>
        <div class="flex gap-2">
            {#each ['contain', 'cover', 'fill'] as f}
                <button type="button"
                class="flex-1 rounded-lg border py-2 text-sm font-medium transition-colors {resizeFit === f
                    ? 'border-indigo-500 bg-indigo-50 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300'
                    : 'border-gray-300 text-gray-600 hover:border-gray-400 dark:border-gray-600 dark:text-gray-400'}"
                onclick={() => { resizeFit = f as typeof resizeFit; updatePreview() }}
                >{f}</button>
            {/each}
        </div>
    </div>

    <div>
        <label for="variant-{kind}-quality" class="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-400">{m.quality()}: {resizeQuality}%</label>
        <input id="variant-{kind}-quality" type="range" min="1" max="100" bind:value={resizeQuality} oninput={updatePreview}
        class="w-full accent-indigo-500" />
    </div>

    <div>
        <label for="variant-{kind}-format" class="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-400">{m.format()}</label>
        <div class="flex gap-2">
            {#each ['jpeg', 'png', 'tiff'] as fmt}
                <button type="button"
                class="flex-1 rounded-lg border py-2 text-sm font-medium transition-colors {resizeFormat === fmt
                    ? 'border-indigo-500 bg-indigo-50 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300'
                    : 'border-gray-300 text-gray-600 hover:border-gray-400 dark:border-gray-600 dark:text-gray-400'}"
                onclick={() => { resizeFormat = fmt as typeof resizeFormat; updatePreview() }}
                >{fmt.toUpperCase()}</button>
            {/each}
        </div>
    </div>

    <Button disabled={creating || authStore.role === 'viewer'} onclick={() => handleCreate?.(kind, { width: resizeWidth || undefined, height: resizeHeight || undefined, fit: resizeFit, quality: resizeQuality, format: resizeFormat })} class="w-full">
        {creating ? m.queuing_() : m.variant_create_resize()}
    </Button>
</div>

