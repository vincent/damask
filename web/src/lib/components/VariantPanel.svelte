<script lang="ts">
  import {
    variantApi,
    formatBytes,
    type Variant,
    type Asset,
  } from '$lib/api/client'
  import { authStore } from '$lib/stores/auth'

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
  let loading = $state(false)
  let loadError = $state('')

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

  // Load variants when asset changes
  $effect(() => {
    if (!asset) { variants = []; return }
    loadVariants()
  })

  async function loadVariants() {
    if (!asset) return
    loading = true
    loadError = ''
    try {
      variants = await variantApi.list(asset.id)
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
    } catch {
      // silently ignore
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
    handleCreate('resize', {
      width: resizeWidth || undefined,
      height: resizeHeight || undefined,
      fit: resizeFit,
      quality: resizeQuality,
      format: resizeFormat,
    })
  }

  function submitConvert() {
    handleCreate('convert', {
      format: convertFormat,
      quality: convertQuality,
    })
  }

  function submitCrop() {
    handleCreate('crop', {
      x: cropX,
      y: cropY,
      width: cropWidth,
      height: cropHeight,
      format: cropFormat,
      quality: 85,
    })
  }

  function submitWatermark() {
    handleCreate('watermark', {
      opacity: watermarkOpacity,
      quality: watermarkQuality,
      format: watermarkFormat,
    })
  }

  function submitVideoThumbnail() {
    handleCreate('video_thumbnail', { timestamp: videoTimestamp })
  }

  function submitTranscode() {
    handleCreate('video_transcode', {
      format: transcodeFormat,
      resolution: transcodeResolution || undefined,
      strip_audio: transcodeStripAudio,
    })
  }

  function submitBgRemove() {
    handleCreate('bg_remove', {})
  }

  function variantLabel(v: Variant): string {
    const params = v.transform_params.Valid
      ? (() => { try { return JSON.parse(v.transform_params.String) } catch { return {} } })()
      : {}
    switch (v.type) {
      case 'resize': return `Resize ${params.width ?? '?'}×${params.height ?? '?'} (${params.format ?? 'jpeg'})`
      case 'convert': return `Convert to ${params.format ?? '?'}`
      case 'crop': return `Crop ${params.width ?? '?'}×${params.height ?? '?'}`
      case 'watermark': return `Watermark ${params.opacity ?? 50}%`
      case 'video_thumbnail': return `Frame at ${params.timestamp ?? 1}s`
      case 'video_transcode': return `Transcode ${params.format ?? 'mp4'}${params.resolution ? ' ' + params.resolution : ''}`
      case 'bg_remove': return 'Background removed'
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
  <!-- Backdrop -->
  <div
    class="fixed inset-0 z-50 bg-black/40"
    role="button"
    tabindex="-1"
    onclick={onclose}
    onkeydown={(e) => e.key === 'Escape' && onclose()}
    aria-label="Close variant panel"
  ></div>

  <!-- Panel -->
  <div
    class="fixed inset-y-0 right-0 z-[60] flex w-full max-w-xl flex-col bg-white shadow-2xl"
    role="dialog"
    aria-modal="true"
    aria-label="Variants"
  >
    <!-- Header -->
    <div class="flex items-center justify-between border-b border-gray-200 px-5 py-4">
      <div>
        <h2 class="text-base font-semibold text-gray-900">Variants</h2>
        <p class="text-xs text-gray-500 truncate max-w-xs">{asset.original_filename}</p>
      </div>
      <button
        type="button"
        class="ml-3 rounded-md p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600"
        onclick={onclose}
        aria-label="Close"
      >
        <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>

    <!-- Tabs -->
    <div class="flex border-b border-gray-200 overflow-x-auto">
      {#each tabs.filter(t => t.show) as tab}
        <button
          type="button"
          class="shrink-0 px-4 py-2.5 text-sm font-medium border-b-2 transition-colors {activeTab === tab.id
            ? 'border-blue-500 text-blue-600'
            : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'}"
          onclick={() => { activeTab = tab.id; createError = ''; createSuccess = '' }}
        >
          {tab.label}
        </button>
      {/each}
    </div>

    <!-- Body -->
    <div class="flex-1 overflow-y-auto p-5">

      <!-- Feedback -->
      {#if createError}
        <div class="mb-4 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700">{createError}</div>
      {/if}
      {#if createSuccess}
        <div class="mb-4 rounded-lg bg-green-50 px-4 py-3 text-sm text-green-700">{createSuccess}</div>
      {/if}

      <!-- List tab -->
      {#if activeTab === 'list'}
        {#if loading}
          <div class="flex justify-center py-8">
            <svg class="h-6 w-6 animate-spin text-gray-400" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
            </svg>
          </div>
        {:else if loadError}
          <p class="text-sm text-red-600">{loadError}</p>
        {:else if variants.length === 0}
          <div class="flex flex-col items-center gap-3 py-12 text-center text-gray-400">
            <svg class="h-12 w-12" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5"
                d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
            </svg>
            <p class="text-sm">No variants yet.</p>
            {#if $authStore.role !== 'viewer'}
              <p class="text-xs">Use the tabs above to create your first variant.</p>
            {/if}
          </div>
        {:else}
          <ul class="space-y-2">
            {#each variants as v}
              <li class="flex items-center gap-3 rounded-xl border border-gray-200 bg-gray-50 px-4 py-3">
                <div class="flex-1 min-w-0">
                  <p class="text-sm font-medium text-gray-900 truncate">{variantLabel(v)}</p>
                  <p class="text-xs text-gray-500">
                    {v.size.Valid ? formatBytes(v.size.Int64) : 'unknown size'} ·
                    {new Date(v.created_at).toLocaleDateString()}
                  </p>
                </div>
                <a
                  href={variantApi.fileUrl(asset.id, v.id)}
                  download
                  class="shrink-0 rounded-lg border border-gray-300 p-1.5 text-gray-500 hover:bg-white hover:text-gray-700"
                  aria-label="Download variant"
                >
                  <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                      d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
                  </svg>
                </a>
                {#if $authStore.role !== 'viewer'}
                  <button
                    type="button"
                    class="shrink-0 rounded-lg p-1.5 text-red-400 hover:bg-red-50 hover:text-red-600"
                    onclick={() => handleDelete(v.id)}
                    aria-label="Delete variant"
                  >
                    <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                        d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                    </svg>
                  </button>
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
            <div class="flex justify-center rounded-xl bg-gray-50 p-3 border border-gray-200" style="min-height:120px">
              <img src={previewUrl} alt="Preview" class="max-h-48 max-w-full rounded object-contain" />
            </div>
          {:else}
            <div class="flex items-center justify-center rounded-xl bg-gray-50 border border-dashed border-gray-300 py-8 text-xs text-gray-400">
              Preview will appear after changing parameters
            </div>
          {/if}

          <div class="grid grid-cols-2 gap-4">
            <div>
              <label class="block text-xs font-medium text-gray-600 mb-1">Width (px)</label>
              <input type="number" min="1" max="8000" bind:value={resizeWidth}
                oninput={updatePreview}
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-blue-400 focus:outline-none" />
            </div>
            <div>
              <label class="block text-xs font-medium text-gray-600 mb-1">Height (px) <span class="text-gray-400">(0=auto)</span></label>
              <input type="number" min="0" max="8000" bind:value={resizeHeight}
                oninput={updatePreview}
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-blue-400 focus:outline-none" />
            </div>
          </div>

          <div>
            <label class="block text-xs font-medium text-gray-600 mb-1">Fit</label>
            <div class="flex gap-2">
              {#each ['contain', 'cover', 'fill'] as f}
                <button type="button"
                  class="flex-1 rounded-lg border py-2 text-xs font-medium transition-colors {resizeFit === f
                    ? 'border-blue-500 bg-blue-50 text-blue-700'
                    : 'border-gray-300 text-gray-600 hover:border-gray-400'}"
                  onclick={() => { resizeFit = f as typeof resizeFit; updatePreview() }}
                >{f}</button>
              {/each}
            </div>
          </div>

          <div>
            <label class="block text-xs font-medium text-gray-600 mb-1">Quality: {resizeQuality}%</label>
            <input type="range" min="1" max="100" bind:value={resizeQuality}
              oninput={updatePreview}
              class="w-full accent-blue-500" />
          </div>

          <div>
            <label class="block text-xs font-medium text-gray-600 mb-1">Format</label>
            <div class="flex gap-2">
              {#each ['jpeg', 'png', 'tiff'] as fmt}
                <button type="button"
                  class="flex-1 rounded-lg border py-2 text-xs font-medium transition-colors {resizeFormat === fmt
                    ? 'border-blue-500 bg-blue-50 text-blue-700'
                    : 'border-gray-300 text-gray-600 hover:border-gray-400'}"
                  onclick={() => { resizeFormat = fmt as typeof resizeFormat; updatePreview() }}
                >{fmt.toUpperCase()}</button>
              {/each}
            </div>
          </div>

          <button
            type="button"
            disabled={creating || $authStore.role === 'viewer'}
            onclick={submitResize}
            class="w-full rounded-xl bg-blue-600 py-2.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
          >
            {creating ? 'Queuing…' : 'Create Resize Variant'}
          </button>
        </div>

      <!-- Resize tab -->
      {:else if activeTab === 'watermark'}
        <div class="space-y-5">
          <!-- Live preview -->
          {#if previewUrl}
            <div class="flex justify-center rounded-xl bg-gray-50 p-3 border border-gray-200" style="min-height:120px">
              <img src={previewUrl} alt="Preview" class="max-h-48 max-w-full rounded object-contain" />
            </div>
          {:else}
            <div class="flex items-center justify-center rounded-xl bg-gray-50 border border-dashed border-gray-300 py-8 text-xs text-gray-400">
              Preview will appear after changing parameters
            </div>
          {/if}

          <div class="grid grid-cols-2 gap-4">
            <div>
              <label class="block text-xs font-medium text-gray-600 mb-1">Opacity (%)</label>
              <input type="number" min="1" max="100" bind:value={watermarkOpacity}
                oninput={updatePreview}
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-blue-400 focus:outline-none" />
            </div>
            <div>
              <label class="block text-xs font-medium text-gray-600 mb-1">Quality: {watermarkQuality}%</label>
              <input type="range" min="1" max="100" bind:value={watermarkQuality}
                oninput={updatePreview}
                class="w-full accent-blue-500" />
            </div>
          </div>

          <div>
            <label class="block text-xs font-medium text-gray-600 mb-1">Format</label>
            <div class="flex gap-2">
              {#each ['jpeg', 'png', 'tiff'] as fmt}
                <button type="button"
                  class="flex-1 rounded-lg border py-2 text-xs font-medium transition-colors {watermarkFormat === fmt
                    ? 'border-blue-500 bg-blue-50 text-blue-700'
                    : 'border-gray-300 text-gray-600 hover:border-gray-400'}"
                  onclick={() => { watermarkFormat = fmt as typeof watermarkFormat; updatePreview() }}
                >{fmt.toUpperCase()}</button>
              {/each}
            </div>
          </div>

          <button
            type="button"
            disabled={creating || $authStore.role === 'viewer'}
            onclick={submitWatermark}
            class="w-full rounded-xl bg-blue-600 py-2.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
          >
            {creating ? 'Queuing…' : 'Create Watermark Variant'}
          </button>
        </div>

      <!-- Convert tab -->
      {:else if activeTab === 'convert'}
        <div class="space-y-5">
          <div>
            <label class="block text-xs font-medium text-gray-600 mb-1">Output Format</label>
            <div class="flex gap-2">
              {#each ['jpeg', 'png', 'tiff'] as fmt}
                <button type="button"
                  class="flex-1 rounded-lg border py-2.5 text-sm font-medium transition-colors {convertFormat === fmt
                    ? 'border-blue-500 bg-blue-50 text-blue-700'
                    : 'border-gray-300 text-gray-600 hover:border-gray-400'}"
                  onclick={() => { convertFormat = fmt as typeof convertFormat }}
                >{fmt.toUpperCase()}</button>
              {/each}
            </div>
          </div>

          {#if convertFormat === 'jpeg'}
            <div>
              <label class="block text-xs font-medium text-gray-600 mb-1">Quality: {convertQuality}%</label>
              <input type="range" min="1" max="100" bind:value={convertQuality}
                class="w-full accent-blue-500" />
            </div>
          {/if}

          <button
            type="button"
            disabled={creating || $authStore.role === 'viewer'}
            onclick={submitConvert}
            class="w-full rounded-xl bg-blue-600 py-2.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
          >
            {creating ? 'Queuing…' : `Convert to ${convertFormat.toUpperCase()}`}
          </button>
        </div>

      <!-- Crop tab -->
      {:else if activeTab === 'crop'}
        <div class="space-y-5">
          <p class="text-xs text-gray-500">Enter crop coordinates relative to the original image dimensions.</p>

          {#if asset.width.Valid && asset.height.Valid}
            <p class="text-xs text-gray-400">Original: {asset.width.Int64} × {asset.height.Int64} px</p>
          {/if}

          <div class="grid grid-cols-2 gap-4">
            <div>
              <label class="block text-xs font-medium text-gray-600 mb-1">X offset</label>
              <input type="number" min="0" bind:value={cropX}
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-blue-400 focus:outline-none" />
            </div>
            <div>
              <label class="block text-xs font-medium text-gray-600 mb-1">Y offset</label>
              <input type="number" min="0" bind:value={cropY}
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-blue-400 focus:outline-none" />
            </div>
            <div>
              <label class="block text-xs font-medium text-gray-600 mb-1">Width</label>
              <input type="number" min="1" bind:value={cropWidth}
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-blue-400 focus:outline-none" />
            </div>
            <div>
              <label class="block text-xs font-medium text-gray-600 mb-1">Height</label>
              <input type="number" min="1" bind:value={cropHeight}
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-blue-400 focus:outline-none" />
            </div>
          </div>

          <div>
            <label class="block text-xs font-medium text-gray-600 mb-1">Format</label>
            <div class="flex gap-2">
              {#each ['jpeg', 'png'] as fmt}
                <button type="button"
                  class="flex-1 rounded-lg border py-2 text-xs font-medium transition-colors {cropFormat === fmt
                    ? 'border-blue-500 bg-blue-50 text-blue-700'
                    : 'border-gray-300 text-gray-600 hover:border-gray-400'}"
                  onclick={() => { cropFormat = fmt as typeof cropFormat }}
                >{fmt.toUpperCase()}</button>
              {/each}
            </div>
          </div>

          <button
            type="button"
            disabled={creating || $authStore.role === 'viewer'}
            onclick={submitCrop}
            class="w-full rounded-xl bg-blue-600 py-2.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
          >
            {creating ? 'Queuing…' : 'Create Crop Variant'}
          </button>
        </div>

      <!-- Video tab -->
      {:else if activeTab === 'video'}
        <div class="space-y-6">
          <!-- Video thumbnail section -->
          <div class="rounded-xl border border-gray-200 p-4 space-y-4">
            <h3 class="text-sm font-semibold text-gray-800">Extract Frame</h3>
            <div>
              <label class="block text-xs font-medium text-gray-600 mb-1">Timestamp (seconds)</label>
              <input type="number" min="0" step="0.1" bind:value={videoTimestamp}
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-blue-400 focus:outline-none" />
            </div>
            <button
              type="button"
              disabled={creating || $authStore.role === 'viewer'}
              onclick={submitVideoThumbnail}
              class="w-full rounded-xl bg-blue-600 py-2.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
            >
              {creating ? 'Queuing…' : 'Extract Frame'}
            </button>
          </div>

          <!-- Transcode section -->
          <div class="rounded-xl border border-gray-200 p-4 space-y-4">
            <h3 class="text-sm font-semibold text-gray-800">Transcode Video</h3>
            <p class="text-xs text-gray-500">Heavy operation — max 2 concurrent transcodes. Requires ffmpeg.</p>

            <div>
              <label class="block text-xs font-medium text-gray-600 mb-1">Output Format</label>
              <div class="flex gap-2">
                {#each ['mp4', 'webm'] as fmt}
                  <button type="button"
                    class="flex-1 rounded-lg border py-2 text-xs font-medium transition-colors {transcodeFormat === fmt
                      ? 'border-blue-500 bg-blue-50 text-blue-700'
                      : 'border-gray-300 text-gray-600 hover:border-gray-400'}"
                    onclick={() => { transcodeFormat = fmt as typeof transcodeFormat }}
                  >{fmt.toUpperCase()}</button>
                {/each}
              </div>
            </div>

            <div>
              <label class="block text-xs font-medium text-gray-600 mb-1">Resolution <span class="text-gray-400">(optional)</span></label>
              <select bind:value={transcodeResolution}
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-blue-400 focus:outline-none">
                <option value="">Original</option>
                <option value="1080p">1080p</option>
                <option value="720p">720p</option>
                <option value="480p">480p</option>
              </select>
            </div>

            <label class="flex items-center gap-2 text-sm text-gray-700">
              <input type="checkbox" bind:checked={transcodeStripAudio} class="rounded" />
              Strip audio track
            </label>

            <button
              type="button"
              disabled={creating || $authStore.role === 'viewer'}
              onclick={submitTranscode}
              class="w-full rounded-xl bg-blue-600 py-2.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
            >
              {creating ? 'Queuing…' : 'Transcode Video'}
            </button>
          </div>
        </div>

      <!-- Background remove tab -->
      {:else if activeTab === 'bg_remove'}
        <div class="space-y-5">
          <div class="rounded-xl bg-amber-50 border border-amber-200 px-4 py-3 text-sm text-amber-800">
            <p class="font-medium mb-1">Requires Remove.bg API key</p>
            <p class="text-xs">Set <code class="bg-amber-100 px-1 rounded">REMOVEBG_API_KEY</code> in your server environment. Returns a transparent PNG.</p>
          </div>

          <button
            type="button"
            disabled={creating || $authStore.role === 'viewer'}
            onclick={submitBgRemove}
            class="w-full rounded-xl bg-blue-600 py-2.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
          >
            {#if creating}
              <span class="flex items-center justify-center gap-2">
                <svg class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
                </svg>
                Queuing…
              </span>
            {:else}
              Remove Background
            {/if}
          </button>
        </div>
      {/if}

    </div>
  </div>
{/if}
