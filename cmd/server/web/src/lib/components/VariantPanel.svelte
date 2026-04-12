<script lang="ts">
  import {
    variantApi,
    formatBytes,
    type Variant,
    type Asset,
  } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { X, Download, Inbox, Trash } from '@lucide/svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import ResolutionOptions from './ResolutionOptions.svelte'
  import Feedback from './ui/Feedback.svelte'
  import ButtonDelete from './ui/ButtonDelete.svelte'

  interface Props {
    asset: Asset | null
    onclose: () => void
  }

  let { asset, onclose }: Props = $props()

  // Tabs
  type Tab = 'list' | 'resize' | 'convert' | 'crop' | 'watermark' | 'video' | 'bg_remove'
  let activeTab = $state<Tab>('list')

  // Variant list state
  let variants = $state<Variant[]>([])
  let rebuilding = $state(false)
  let loading = $state(false)
  let loadError = $state('')
  let rebuildPollTimer: ReturnType<typeof setInterval> | null = null

  // Create state
  let creating = $state(false)
  let createError = $state('')
  let createSuccess = $state('')

  // Resize params
  let resizeWidth = $state(800)
  let resizeHeight = $state(0)
  let resizeFit = $state<'contain' | 'cover' | 'fill'>('contain')
  let resizeQuality = $state(85)
  let resizeFormat = $state<'jpeg' | 'png' | 'tiff'>('jpeg')

  // Convert params
  let convertFormat = $state<'jpeg' | 'png' | 'tiff'>('png')
  let convertQuality = $state(90)

  // Watermark params
  let watermarkOpacity = $state(50)
  let watermarkQuality = $state(80)
  let watermarkFormat = $state<'jpeg' | 'png' | 'tiff'>('jpeg')

  // Crop params
  let cropX = $state(0)
  let cropY = $state(0)
  let cropWidth = $state(400)
  let cropHeight = $state(400)
  let cropFormat = $state<'jpeg' | 'png'>('jpeg')

  // Video thumbnail params
  let videoTimestamp = $state(1)

  // Video transcode params
  let transcodeFormat = $state<'mp4' | 'webm'>('mp4')
  let transcodeResolution = $state<'' | '1080p' | '720p' | '480p'>('')
  let transcodeStripAudio = $state(false)

  // Preview URL (debounced)
  let previewUrl = $state('')
  let previewTimeout: ReturnType<typeof setTimeout>

  const isImage = $derived(asset?.mime_type?.startsWith('image/') ?? false)
  const isVideo = $derived(asset?.mime_type?.startsWith('video/') ?? false)

  // Load variants when asset changes; start/stop rebuild polling accordingly.
  $effect(() => {
    if (!asset) {
      variants = []
      rebuilding = false
      stopRebuildPoll()
      return
    }
    loadVariants()
    return () => stopRebuildPoll()
  })

  function stopRebuildPoll() {
    if (rebuildPollTimer !== null) {
      clearInterval(rebuildPollTimer)
      rebuildPollTimer = null
    }
  }

  async function loadVariants() {
    if (!asset) return
    loading = true
    loadError = ''
    try {
      const result = await variantApi.list(asset.id)
      variants = result.variants
      rebuilding = result.rebuilding
      // Start polling if a rebuild is in flight; stop it once done.
      if (rebuilding && rebuildPollTimer === null) {
        rebuildPollTimer = setInterval(async () => {
          if (!asset) { stopRebuildPoll(); return }
          try {
            const r = await variantApi.list(asset.id)
            variants = r.variants
            rebuilding = r.rebuilding
            if (!r.rebuilding) stopRebuildPoll()
          } catch { /* ignore poll errors */ }
        }, 5000)
      } else if (!rebuilding) {
        stopRebuildPoll()
      }
    } catch {
      loadError = 'Failed to load variants'
    } finally {
      loading = false
    }
  }

  async function handleCreate(type: string, params: object) {
    if (!asset) return
    creating = true
    createError = ''
    createSuccess = ''
    try {
      const result = await variantApi.create(asset.id, type, params)
      createSuccess = `Queued (job ${result.job_id.slice(0, 8)}). Check back in a moment.`
      // Switch to list tab and refresh after a delay
      setTimeout(() => {
        activeTab = 'list'
        loadVariants()
        createSuccess = ''
      }, 3000)
    } catch (e: unknown) {
      createError = e instanceof Error ? e.message : 'Failed to create variant'
    } finally {
      creating = false
    }
  }

  async function handleDelete(variantId: string) {
    if (!asset) return
    try {
      await variantApi.delete(asset.id, variantId)
      variants = variants.filter(v => v.id !== variantId)
    } catch (e: unknown) {
      createError = e instanceof Error ? e.message : 'Failed to delete variant'
    }
  }

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

  function submitResize() {
    handleCreate('image_resize', {
      width: resizeWidth || undefined,
      height: resizeHeight || undefined,
      fit: resizeFit,
      quality: resizeQuality,
      format: resizeFormat,
    })
  }

  function submitConvert() {
    handleCreate('image_convert', {
      format: convertFormat,
      quality: convertQuality,
    })
  }

  function submitCrop() {
    handleCreate('image_crop', {
      x: cropX,
      y: cropY,
      width: cropWidth,
      height: cropHeight,
      format: cropFormat,
      quality: 85,
    })
  }

  function submitWatermark() {
    handleCreate('image_watermark', {
      opacity: watermarkOpacity,
      quality: watermarkQuality,
      format: watermarkFormat,
    })
  }

  function submitVideoThumbnail() {
    handleCreate('video_capture_image', { timestamp: videoTimestamp })
  }

  function submitTranscode() {
    handleCreate('video_transcode', {
      format: transcodeFormat,
      resolution: transcodeResolution || undefined,
      strip_audio: transcodeStripAudio,
    })
  }

  function submitBgRemove() {
    handleCreate('image_bg_remove', {})
  }

  function variantLabel(v: Variant): string {
    const params = v.transform_params
      ? (() => { try { return JSON.parse(v.transform_params) } catch { return {} } })()
      : {}
    switch (v.type) {
      case 'image_resize': return `Resize ${params.width ?? '?'}×${params.height ?? '?'} (${params.format ?? 'jpeg'})`
      case 'image_convert': return `Convert to ${params.format ?? '?'}`
      case 'image_crop': return `Crop ${params.width ?? '?'}×${params.height ?? '?'}`
      case 'image_watermark': return `Watermark ${params.opacity ?? 50}%`
      case 'video_capture_image': return `Frame at ${params.timestamp ?? 1}s`
      case 'video_transcode': return `Transcode ${params.format ?? 'mp4'}${params.resolution ? ' ' + params.resolution : ''}`
      case 'image_bg_remove': return 'Background removed'
      case 'image_smartcrop': return `Smart Crop ${params.width ?? '?'}×${params.height ?? '?'}`
      case 'manual': return 'Manual variant'
      default: return v.type
    }
  }

  const tabs: { id: Tab; label: string; show: boolean }[] = [
    { id: 'list',      label: 'Variants',  show: true },
    { id: 'resize',    label: 'Resize',    show: isImage },
    { id: 'watermark', label: 'Watermark', show: isImage },
    { id: 'convert',   label: 'Convert',   show: isImage },
    { id: 'crop',      label: 'Crop',      show: isImage },
    { id: 'video',     label: 'Video',     show: isVideo },
    { id: 'bg_remove', label: 'Bg Remove', show: isImage },
  ]
</script>

{#if asset}

  <!-- Panel -->
  <div>
    <!-- Header -->
    <div class="flex items-center justify-between border-b border-gray-200 px-5 py-4 dark:border-gray-800">
      <div>
        <h2 class="text-base font-semibold text-gray-900 dark:text-gray-50">Variants</h2>
        <p class="text-sm text-gray-500 truncate max-w-xs dark:text-gray-400">{asset.original_filename}</p>
      </div>
      <button
        type="button"
        class="ml-3 rounded-md p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-gray-800 dark:hover:text-gray-300"
        onclick={onclose}
        aria-label="Close"
      >
        <X class="h-5 w-5" />
      </button>
    </div>

    <!-- Tabs -->
    <div class="flex border-b border-gray-200 overflow-x-auto dark:border-gray-800">
      {#each tabs.filter(t => t.show) as tab}
        <button
          type="button"
          disabled={tab.id !== 'list' && rebuilding}
          title={tab.id !== 'list' && rebuilding ? 'Variants are rebuilding — please wait.' : undefined}
          class="shrink-0 px-4 py-2.5 text-md font-medium border-b-2 transition-colors disabled:opacity-40 disabled:cursor-not-allowed {activeTab === tab.id
            ? 'border-blue-500 text-blue-600'
            : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-200 dark:hover:border-gray-600'}"
          onclick={() => { if (!rebuilding || tab.id === 'list') { activeTab = tab.id; createError = ''; createSuccess = '' } }}
        >
          {tab.label}
        </button>
      {/each}
    </div>

    <!-- Body -->
    <div class="flex-1 overflow-y-auto p-5">

      <!-- Rebuilding banner (VV-4.1) -->
      {#if rebuilding}
        <div class="mb-4 flex items-center gap-3 rounded-lg bg-blue-50 px-4 py-3 text-md text-blue-700 dark:bg-blue-900/30 dark:text-blue-300">
          <Spinner size="sm" class="text-blue-500 shrink-0" />
          <span>Variants are being rebuilt for the new version…</span>
        </div>
      {/if}

      <!-- Feedback -->
      <Feedback error={createError} success={createSuccess} />

      <!-- List tab -->
      {#if activeTab === 'list'}
        {#if loading}
          <div class="flex justify-center py-8">
            <Spinner size="md" class="text-gray-400" />
          </div>
        {:else if loadError}
          <Feedback error={loadError} />
        {:else if variants.length === 0}
          <div class="flex flex-col items-center gap-3 py-12 text-center text-gray-400">
            <Inbox class="h-12 w-12" />
            <p class="text-md">No variants yet.</p>
            {#if authStore.role !== 'viewer'}
              <p class="text-sm">Use the tabs above to create your first variant.</p>
            {/if}
          </div>
        {:else}
          <ul class="space-y-2">
            {#each variants as v}
              <li class="flex items-center gap-3 rounded-xl border border-gray-200 bg-gray-50 px-4 py-3 dark:border-gray-700 dark:bg-gray-800">
                <div class="flex-1 min-w-0">
                  <p class="text-md font-medium text-gray-900 truncate dark:text-gray-100">{variantLabel(v)}</p>
                  <p class="text-sm text-gray-500 dark:text-gray-400">
                    {v.size != null ? formatBytes(v.size) : 'unknown size'} ·
                    {new Date(v.created_at).toLocaleDateString()}
                  </p>
                </div>
                <a
                  href={variantApi.fileUrl(asset.id, v.id)}
                  download
                  target="_blank"
                  class="shrink-0 rounded-lg border border-gray-300 p-1.5 text-gray-500 hover:bg-white hover:text-gray-700 dark:border-gray-600 dark:text-gray-400 dark:hover:bg-gray-700 dark:hover:text-gray-200"
                  aria-label="Download variant"
                >
                  <Download class="h-4 w-4" />
                </a>
                {#if authStore.role !== 'viewer'}
                  <ButtonDelete title="Delete variant" onclick={() => handleDelete(v.id)} />
                {/if}
              </li>
            {/each}
          </ul>
        {/if}

      <!-- Resize tab -->
      {:else if activeTab === 'resize'}
        <div class="space-y-5">
          <!-- Live preview -->
          {#if previewUrl}
            <div class="flex justify-center rounded-xl bg-gray-50 p-3 border border-gray-200 dark:bg-gray-800 dark:border-gray-700" style="min-height:120px">
              <img src={previewUrl} alt="Preview" class="max-h-48 max-w-full rounded object-contain" />
            </div>
          {:else}
            <div class="flex items-center justify-center rounded-xl bg-gray-50 border border-dashed border-gray-300 py-8 text-sm text-gray-400 dark:bg-gray-800 dark:border-gray-600">
              Preview will appear after changing parameters
            </div>
          {/if}

          <div class="grid grid-cols-2 gap-4">
            <div>
              <label for="variant-resize-width" class="block text-sm font-medium text-gray-600 dark:text-gray-400 mb-1">Width (px)</label>
              <input id="variant-resize-width" type="number" min="1" max="8000" bind:value={resizeWidth}
                oninput={updatePreview}
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-md focus:border-blue-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100" />
            </div>
            <div>
              <label for="variant-resize-height" class="block text-sm font-medium text-gray-600 dark:text-gray-400 mb-1">Height (px) <span class="text-gray-400">(0=auto)</span></label>
              <input id="variant-resize-height" type="number" min="0" max="8000" bind:value={resizeHeight}
                oninput={updatePreview}
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-md focus:border-blue-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100" />
            </div>
          </div>

          <div>
            <label for="variant-resize-fit" class="block text-sm font-medium text-gray-600 dark:text-gray-400 mb-1">Fit</label>
            <div class="flex gap-2">
              {#each ['contain', 'cover', 'fill'] as f}
                <button type="button"
                  class="flex-1 rounded-lg border py-2 text-sm font-medium transition-colors {resizeFit === f
                    ? 'border-blue-500 bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
                    : 'border-gray-300 text-gray-600 hover:border-gray-400 dark:border-gray-600 dark:text-gray-400 dark:hover:border-gray-500'}"
                  onclick={() => { resizeFit = f as typeof resizeFit; updatePreview() }}
                >{f}</button>
              {/each}
            </div>
          </div>

          <div>
            <label for="variant-resize-quality" class="block text-sm font-medium text-gray-600 dark:text-gray-400 mb-1">Quality: {resizeQuality}%</label>
            <input id="variant-resize-quality" type="range" min="1" max="100" bind:value={resizeQuality}
              oninput={updatePreview}
              class="w-full accent-blue-500" />
          </div>

          <div>
            <label for="variant-resize-format" class="block text-sm font-medium text-gray-600 dark:text-gray-400 mb-1">Format</label>
            <div class="flex gap-2">
              {#each ['jpeg', 'png', 'tiff'] as fmt}
                <button type="button"
                  class="flex-1 rounded-lg border py-2 text-sm font-medium transition-colors {resizeFormat === fmt
                    ? 'border-blue-500 bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
                    : 'border-gray-300 text-gray-600 hover:border-gray-400 dark:border-gray-600 dark:text-gray-400 dark:hover:border-gray-500'}"
                  onclick={() => { resizeFormat = fmt as typeof resizeFormat; updatePreview() }}
                >{fmt.toUpperCase()}</button>
              {/each}
            </div>
          </div>

          <Button
            type="button"
            disabled={creating || authStore.role === 'viewer'}
            onclick={submitResize}
            class="w-full"
          >
            {creating ? 'Queuing…' : 'Create Resize Variant'}
          </Button>
        </div>

      <!-- Watermark tab -->
      {:else if activeTab === 'watermark'}
        <div class="space-y-5">
          <!-- Live preview -->
          {#if previewUrl}
            <div class="flex justify-center rounded-xl bg-gray-50 p-3 border border-gray-200 dark:bg-gray-800 dark:border-gray-700" style="min-height:120px">
              <img src={previewUrl} alt="Preview" class="max-h-48 max-w-full rounded object-contain" />
            </div>
          {:else}
            <div class="flex items-center justify-center rounded-xl bg-gray-50 border border-dashed border-gray-300 py-8 text-sm text-gray-400 dark:bg-gray-800 dark:border-gray-600">
              Preview will appear after changing parameters
            </div>
          {/if}

          <div class="grid grid-cols-2 gap-4">
            <div>
              <label for="variant-watermark-opacity" class="block text-sm font-medium text-gray-600 dark:text-gray-400 mb-1">Opacity ({watermarkOpacity}%)</label>
              <input id="variant-watermark-opacity" type="range" min="1" max="100" bind:value={watermarkOpacity}
                oninput={updatePreview}
                class="w-full accent-blue-500" />
            </div>
            <div>
              <label for="variant-watermark-quality" class="block text-sm font-medium text-gray-600 dark:text-gray-400 mb-1">Quality: {watermarkQuality}%</label>
              <input id="variant-watermark-quality" type="range" min="1" max="100" bind:value={watermarkQuality}
                oninput={updatePreview}
                class="w-full accent-blue-500" />
            </div>
          </div>

          <div>
            <label for="variant-watermark-format" class="block text-sm font-medium text-gray-600 dark:text-gray-400 mb-1">Format</label>
            <div class="flex gap-2">
              {#each ['jpeg', 'png', 'tiff'] as fmt}
                <button type="button"
                  class="flex-1 rounded-lg border py-2 text-sm font-medium transition-colors {watermarkFormat === fmt
                    ? 'border-blue-500 bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
                    : 'border-gray-300 text-gray-600 hover:border-gray-400 dark:border-gray-600 dark:text-gray-400 dark:hover:border-gray-500'}"
                  onclick={() => { watermarkFormat = fmt as typeof watermarkFormat; updatePreview() }}
                >{fmt.toUpperCase()}</button>
              {/each}
            </div>
          </div>

          <Button
            type="button"
            disabled={creating || authStore.role === 'viewer'}
            onclick={submitWatermark}
            class="w-full"
          >
            {creating ? 'Queuing…' : 'Create Watermark Variant'}
          </Button>
        </div>

      <!-- Convert tab -->
      {:else if activeTab === 'convert'}
        <div class="space-y-5">
          <div>
            <label for="variant-convert-format" class="block text-sm font-medium text-gray-600 dark:text-gray-400 mb-1">Output Format</label>
            <div class="flex gap-2">
              {#each ['jpeg', 'png', 'tiff'] as fmt}
                <button type="button"
                  class="flex-1 rounded-lg border py-2.5 text-md font-medium transition-colors {convertFormat === fmt
                    ? 'border-blue-500 bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
                    : 'border-gray-300 text-gray-600 hover:border-gray-400 dark:border-gray-600 dark:text-gray-400 dark:hover:border-gray-500'}"
                  onclick={() => { convertFormat = fmt as typeof convertFormat }}
                >{fmt.toUpperCase()}</button>
              {/each}
            </div>
          </div>

          {#if convertFormat === 'jpeg'}
            <div>
              <label for="variant-convert-quality" class="block text-sm font-medium text-gray-600 dark:text-gray-400 mb-1">Quality: {convertQuality}%</label>
              <input id="variant-convert-quality" type="range" min="1" max="100" bind:value={convertQuality}
                class="w-full accent-blue-500" />
            </div>
          {/if}

          <Button
            type="button"
            disabled={creating || authStore.role === 'viewer'}
            onclick={submitConvert}
            class="w-full"
          >
            {creating ? 'Queuing…' : `Convert to ${convertFormat.toUpperCase()}`}
          </Button>
        </div>

      <!-- Crop tab -->
      {:else if activeTab === 'crop'}
        <div class="space-y-5">
          <p class="text-sm text-gray-500 dark:text-gray-400">Enter crop coordinates relative to the original image dimensions.</p>

          {#if asset.width != null && asset.height != null}
            <p class="text-sm text-gray-400">Original: {asset.width} × {asset.height} px</p>
          {/if}

          <div class="grid grid-cols-2 gap-4">
            <div>
              <label for="variant-crop-x" class="block text-sm font-medium text-gray-600 dark:text-gray-400 mb-1">X offset</label>
              <input id="variant-crop-x" type="number" min="0" bind:value={cropX}
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-md focus:border-blue-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100" />
            </div>
            <div>
              <label for="variant-crop-y" class="block text-sm font-medium text-gray-600 dark:text-gray-400 mb-1">Y offset</label>
              <input id="variant-crop-y" type="number" min="0" bind:value={cropY}
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-md focus:border-blue-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100" />
            </div>
            <div>
              <label for="variant-crop-width" class="block text-sm font-medium text-gray-600 dark:text-gray-400 mb-1">Width</label>
              <input id="variant-crop-width" type="number" min="1" bind:value={cropWidth}
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-md focus:border-blue-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100" />
            </div>
            <div>
              <label for="variant-crop-height" class="block text-sm font-medium text-gray-600 dark:text-gray-400 mb-1">Height</label>
              <input id="variant-crop-height" type="number" min="1" bind:value={cropHeight}
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-md focus:border-blue-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100" />
            </div>
          </div>

          <div>
            <label for="variant-crop-format" class="block text-sm font-medium text-gray-600 dark:text-gray-400 mb-1">Format</label>
            <div class="flex gap-2">
              {#each ['jpeg', 'png'] as fmt}
                <button type="button"
                  class="flex-1 rounded-lg border py-2 text-sm font-medium transition-colors {cropFormat === fmt
                    ? 'border-blue-500 bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
                    : 'border-gray-300 text-gray-600 hover:border-gray-400 dark:border-gray-600 dark:text-gray-400 dark:hover:border-gray-500'}"
                  onclick={() => { cropFormat = fmt as typeof cropFormat }}
                >{fmt.toUpperCase()}</button>
              {/each}
            </div>
          </div>

          <Button
            type="button"
            disabled={creating || authStore.role === 'viewer'}
            onclick={submitCrop}
            class="w-full"
          >
            {creating ? 'Queuing…' : 'Create Crop Variant'}
          </Button>
        </div>

      <!-- Video tab -->
      {:else if activeTab === 'video'}
        <div class="space-y-6">
          <!-- Video thumbnail section -->
          <div class="rounded-xl border border-gray-200 p-4 space-y-4 dark:border-gray-700">
            <h3 class="text-md font-semibold text-gray-800 dark:text-gray-200">Extract Frame</h3>
            <div>
              <label for="variant-video-timestamp" class="block text-sm font-medium text-gray-600 dark:text-gray-400 mb-1">Timestamp (seconds)</label>
              <input id="variant-video-timestamp" type="number" min="0" step="0.1" bind:value={videoTimestamp}
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-md focus:border-blue-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100" />
            </div>
            <Button
              type="button"
              disabled={creating || authStore.role === 'viewer'}
              onclick={submitVideoThumbnail}
              class="w-full"
            >
              {creating ? 'Queuing…' : 'Extract Frame'}
            </Button>
          </div>

          <!-- Transcode section -->
          <div class="rounded-xl border border-gray-200 p-4 space-y-4 dark:border-gray-700">
            <h3 class="text-md font-semibold text-gray-800 dark:text-gray-200">Transcode Video</h3>
            <p class="text-sm text-gray-500 dark:text-gray-400">Heavy operation — max 2 concurrent transcodes. Requires ffmpeg.</p>

            <div>
              <label for="variant-transcode-format" class="block text-sm font-medium text-gray-600 dark:text-gray-400 mb-1">Output Format</label>
              <div class="flex gap-2">
                {#each ['mp4', 'webm'] as fmt}
                  <button type="button"
                    class="flex-1 rounded-lg border py-2 text-sm font-medium transition-colors {transcodeFormat === fmt
                      ? 'border-blue-500 bg-blue-50 text-blue-700'
                      : 'border-gray-300 text-gray-600 hover:border-gray-400'}"
                    onclick={() => { transcodeFormat = fmt as typeof transcodeFormat }}
                  >{fmt.toUpperCase()}</button>
                {/each}
              </div>
            </div>

            <div>
              <label for="variant-transcode-resolution" class="block text-sm font-medium text-gray-600 dark:text-gray-400 mb-1">Resolution <span class="text-gray-400">(optional)</span></label>
              <select id="variant-transcode-resolution" bind:value={transcodeResolution}
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-md focus:border-blue-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100">
                <ResolutionOptions />
              </select>
            </div>

            <label class="flex items-center gap-2 text-md text-gray-700 dark:text-gray-300">
              <input type="checkbox" bind:checked={transcodeStripAudio} class="rounded" />
              Strip audio track
            </label>

            <Button
              type="button"
              disabled={creating || authStore.role === 'viewer'}
              onclick={submitTranscode}
              class="w-full"
            >
              {creating ? 'Queuing…' : 'Transcode Video'}
            </Button>
          </div>
        </div>

      <!-- Background remove tab -->
      {:else if activeTab === 'bg_remove'}
        <div class="space-y-5">
          <div class="rounded-xl bg-amber-50 border border-amber-200 px-4 py-3 text-md text-amber-800 dark:bg-amber-900/20 dark:border-amber-800 dark:text-amber-300">
            <p class="font-medium mb-1">Requires Remove.bg API key</p>
            <p class="text-sm">Set <code class="bg-amber-100 px-1 rounded dark:bg-amber-800/50">REMOVEBG_API_KEY</code> in your server environment. Returns a transparent PNG.</p>
          </div>

          <Button
            type="button"
            disabled={creating || authStore.role === 'viewer'}
            onclick={submitBgRemove}
            class="w-full"
          >
            {#if creating}
              <Spinner size="sm" class="text-current" />
              Queuing…
            {:else}
              Remove Background
            {/if}
          </Button>
        </div>
      {/if}

    </div>
  </div>
{/if}
