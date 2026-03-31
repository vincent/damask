<script lang="ts">
  import {
    assetApi,
    tagApi,
    variantApi,
    formatBytes,
    mimeCategory,
    type Asset,
    type Variant,
  } from '$lib/api/client'
  import { authStore } from '$lib/stores/auth.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import ShareModal from './ShareModal.svelte'
  import {
    Copy,
    Download,
    Inbox,
    Link,
    Loader,
    Share,
    Trash,
    X,
  } from '@lucide/svelte'
  import Badge from '$lib/components/ui/Badge.svelte'
  import Chip from '$lib/components/ui/Chip.svelte'
  import ColorDot from '$lib/components/ui/ColorDot.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import SharedAsset from './SharedAsset.svelte'

  interface Props {
    asset: Asset | null
    onclose: () => void
    ondeleted: (id: string) => void
    ontagschanged: () => void
    onprojectchanged: () => void
  }

  let { asset, onclose, ondeleted, ontagschanged, onprojectchanged }: Props = $props()

  // --- Panel tabs ---
  type PanelTab = 'details' | 'variants' | 'actions'
  let activeTab = $state<PanelTab>('details')

  // --- Variant sub-tabs ---
  type VariantTab = 'all' | 'resize' | 'watermark' | 'convert' | 'crop' | 'bg_remove' | 'video'
  let activeVariantTab = $state<VariantTab>('all')

  // --- Asset state ---
  let deleting = $state(false)
  let showShareModal = $state(false)
  let tags = $state<string[]>([])
  let tagInput = $state('')
  let tagSuggestions = $state<string[]>([])
  let showTagInput = $state(false)
  let showProjectPicker = $state(false)
  let allTags = $state<{ id: string; name: string; asset_count: number }[]>([])
  let linkCopied = $state(false)

  // --- Variant state ---
  let variants = $state<Variant[]>([])
  let variantsLoading = $state(false)
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

  // Video params
  let videoTimestamp = $state(1)
  let transcodeFormat = $state<'mp4' | 'webm'>('mp4')
  let transcodeResolution = $state<'' | '1080p' | '720p' | '480p'>('')
  let transcodeStripAudio = $state(false)

  // Live preview
  let previewUrl = $state('')
  let previewTimeout: ReturnType<typeof setTimeout>

  // Export
  let exportFormat = $state('original')

  const category = $derived(asset ? mimeCategory(asset.mime_type) : 'document')

  const previewBg: Record<string, string> = {
    image: 'bg-sky-300 dark:bg-sky-700',
    video: 'bg-red-300 dark:bg-red-700',
    audio: 'bg-emerald-300 dark:bg-emerald-700',
    document: 'bg-blue-200 dark:bg-blue-700',
  }

  const typeLabel: Record<string, string> = {
    image: 'IMAGE',
    video: 'VIDEO',
    audio: 'AUDIO',
    document: 'DOCUMENT',
  }

  const isImage = $derived(asset?.mime_type?.startsWith('image/') ?? false)
  const isVideo = $derived(asset?.mime_type?.startsWith('video/') ?? false)

  $effect(() => {
    if (!asset) {
      tags = []
      variants = []
      previewUrl = ''
      return
    }
    tagApi.getForAsset(asset.id).then((t) => { tags = t }).catch(() => {})
    tagApi.list().then((t) => { allTags = t }).catch(() => {})
    loadVariants()
  })

  async function loadVariants() {
    if (!asset) return
    variantsLoading = true
    try {
      variants = await variantApi.list(asset.id)
    } catch {
      // silently ignore
    } finally {
      variantsLoading = false
    }
  }

  function formatDate(iso: string) {
    return new Date(iso).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })
  }

  async function handleDelete() {
    if (!asset) return
    deleting = true
    try {
      await assetApi.delete(asset.id)
      ondeleted(asset.id)
    } finally {
      deleting = false
    }
  }

  async function addTag(name: string) {
    if (!asset || !name.trim()) return
    const n = name.trim().toLowerCase()
    if (tags.includes(n)) return
    try {
      await tagApi.addToAsset(asset.id, n)
      tags = [...tags, n]
      tagInput = ''
      showTagInput = false
      ontagschanged()
    } catch { /* silently ignore */ }
  }

  async function removeTag(name: string) {
    if (!asset) return
    try {
      await tagApi.removeFromAsset(asset.id, name)
      tags = tags.filter((t) => t !== name)
      ontagschanged()
    } catch { /* silently ignore */ }
  }

  async function assignProject(projectId: string | null) {
    if (!asset) return
    try {
      await tagApi.bulkProject([asset.id], projectId)
      showProjectPicker = false
      onprojectchanged()
    } catch { /* silently ignore */ }
  }

  function updateSuggestions() {
    const q = tagInput.trim().toLowerCase()
    if (!q) { tagSuggestions = []; return }
    tagSuggestions = allTags
      .map((t) => t.name)
      .filter((n) => n.includes(q) && !tags.includes(n))
      .slice(0, 5)
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') onclose()
  }

  const activeProject = $derived(
    asset?.project_id.Valid
      ? projectsStore.projects.find((p) => p.id === asset.project_id.String) ?? null
      : null,
  )

  const shareTargets = $derived(
    asset ? [{ type: 'asset' as const, id: asset.id, label: 'Selected Asset' }] : [],
  )

  async function copyShareLink() {
    if (!asset) return
    try {
      await navigator.clipboard.writeText(window.location.origin + `/s/${asset.id}`)
      linkCopied = true
      setTimeout(() => { linkCopied = false }, 2000)
    } catch { /* silently ignore */ }
  }

  // --- Variant helpers ---
  async function handleCreate(type: string, params: object) {
    if (!asset) return
    creating = true
    createError = ''
    createSuccess = ''
    try {
      const result = await variantApi.create(asset.id, type, params)
      createSuccess = `Queued (job ${result.job_id.slice(0, 8)}). Refreshing shortly…`
      setTimeout(() => {
        activeVariantTab = 'all'
        loadVariants()
        createSuccess = ''
      }, 3000)
    } catch (e: unknown) {
      createError = e instanceof Error ? e.message : 'Failed to create variant'
    } finally {
      creating = false
    }
  }

  async function handleDeleteVariant(variantId: string) {
    if (!asset) return
    try {
      await variantApi.delete(asset.id, variantId)
      variants = variants.filter((v) => v.id !== variantId)
    } catch { /* silently ignore */ }
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

  // Visible variant sub-tabs based on asset type
  const variantSubTabs = $derived([
    { id: 'all' as VariantTab, label: 'All' },
    ...(isImage ? [
      { id: 'resize' as VariantTab, label: 'Resize' },
      { id: 'watermark' as VariantTab, label: 'Watermark' },
      { id: 'convert' as VariantTab, label: 'Convert' },
      { id: 'crop' as VariantTab, label: 'Crop' },
      { id: 'bg_remove' as VariantTab, label: 'Bg Remove' },
    ] : []),
    ...(isVideo ? [
      { id: 'video' as VariantTab, label: 'Video' },
    ] : []),
  ])
</script>

<svelte:window onkeydown={handleKeydown} />

{#if asset}
  <!-- Backdrop -->
  <div
    class="fixed w-[75%] grid place-items-center p-40 inset-0 z-40 bg-black/40 backdrop-blur-sm"
    role="button"
    tabindex="-1"
    onclick={onclose}
    onkeydown={(e) => e.key === 'Enter' && onclose()}
    aria-label="Close lightbox"
  >
    <SharedAsset
      {asset} {category}
      thumbUrl={assetApi.thumbUrl(asset.id)}
      assetUrl={assetApi.fileUrl(asset.id)}
    />
  </div>

  <!-- Panel: fixed 25% -->
  <div
    class="fixed w-[25%] inset-y-0 right-0 z-50 flex w-3xl flex-col bg-white shadow-2xl dark:bg-gray-900"
    role="dialog"
    aria-modal="true"
    aria-label={asset.original_filename}
  >
    <!-- Preview (h-20) -->
    <div class="relative h-20 flex-shrink-0 flex items-center justify-center {previewBg[category]}">
      <!-- Top-right controls -->
      <div class="absolute right-3 top-3 flex items-center gap-1.5">
        {#if authStore.role !== 'viewer'}
          <button
            type="button"
            class="flex h-8 w-8 items-center justify-center rounded-lg bg-white/20 text-white backdrop-blur-sm hover:bg-white/30 transition-colors"
            onclick={handleDelete}
            disabled={deleting}
            aria-label="Delete asset"
            title="Delete"
          >
            {#if deleting}
              <Loader class="h-4 w-4 animate-spin" />
            {:else}
              <Trash class="h-4 w-4" />
            {/if}
          </button>
        {/if}
        <button
          type="button"
          class="flex h-8 w-8 items-center justify-center rounded-lg bg-white/20 text-white backdrop-blur-sm hover:bg-white/30 transition-colors"
          onclick={onclose}
          aria-label="Close"
        >
          <X class="h-4 w-4" />
        </button>
      </div>
    </div>

    <!-- Compact info bar (~80px) -->
    <div class="flex-shrink-0 border-b border-gray-100 px-5 py-3 dark:border-gray-800">
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0">
          <!-- Pills row -->
          <div class="mb-1.5 flex flex-wrap items-center gap-1.5">
            <Badge variant={category as 'image'|'video'|'audio'|'document'|'neutral'} size="md">
              {typeLabel[category]}
            </Badge>
            <span class="rounded bg-gray-100 px-1.5 py-0.5 text-[11px] font-medium text-gray-500 dark:bg-gray-800 dark:text-gray-400">
              {formatBytes(asset.size)}
            </span>
            {#if asset.width.Valid && asset.height.Valid}
              <span class="rounded bg-gray-100 px-1.5 py-0.5 text-[11px] font-medium text-gray-500 dark:bg-gray-800 dark:text-gray-400">
                {asset.width.Int64} × {asset.height.Int64}
              </span>
            {/if}
          </div>
          <!-- Filename -->
          <h2 class="truncate text-base font-bold leading-tight text-gray-900 dark:text-gray-50" title={asset.original_filename}>
            {asset.original_filename}
          </h2>
          <!-- Author + date -->
          <p class="mt-0.5 text-xs text-gray-400 dark:text-gray-500">
            {formatDate(asset.created_at)}
          </p>
        </div>
        <!-- Download button -->
        <a
          href={assetApi.fileUrl(asset.id)}
          download={asset.original_filename}
          class="flex shrink-0 items-center justify-center rounded-xl bg-indigo-600 p-2.5 text-white hover:bg-indigo-700 transition-colors"
          aria-label="Download original"
        >
          <Download class="h-4 w-4" />
        </a>
      </div>
    </div>

    <!-- Animated tab bar -->
    <div class="relative flex-shrink-0 border-b border-gray-100 dark:border-gray-800">
      <div class="flex">
        {#each (['details', 'variants', 'actions'] as PanelTab[]) as tab}
          <button
            type="button"
            class="relative flex-1 py-3 text-sm font-medium transition-colors {activeTab === tab
              ? 'text-indigo-600 dark:text-indigo-400'
              : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'}"
            onclick={() => { activeTab = tab; createError = ''; createSuccess = '' }}
          >
            {tab.charAt(0).toUpperCase() + tab.slice(1)}
            {#if activeTab === tab}
              <span class="absolute bottom-0 left-0 right-0 h-0.5 rounded-t bg-indigo-600 dark:bg-indigo-400"></span>
            {/if}
          </button>
        {/each}
      </div>
    </div>

    <!-- Tab content (scrollable) -->
    <div class="flex-1 overflow-y-auto">

      <!-- ═══ DETAILS TAB ═══ -->
      {#if activeTab === 'details'}
        <div class="px-5 py-5 space-y-6">

          <!-- Metadata grid -->
          <div>
            <p class="mb-3 text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">Metadata</p>
            <div class="grid grid-cols-3 gap-3">
              <div class="rounded-xl bg-gray-50 px-3 py-3 dark:bg-gray-800">
                <p class="mb-1 text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">Created By</p>
                <p class="text-sm font-semibold text-gray-900 dark:text-gray-100">—</p>
              </div>
              <div class="rounded-xl bg-gray-50 px-3 py-3 dark:bg-gray-800">
                <p class="mb-1 text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">Date Added</p>
                <p class="text-sm font-semibold text-gray-900 dark:text-gray-100">
                  {new Date(asset.created_at).toLocaleDateString('en-CA')}
                </p>
              </div>
              <div class="rounded-xl bg-gray-50 px-3 py-3 dark:bg-gray-800">
                <p class="mb-1 text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">Modified</p>
                <p class="text-sm font-semibold text-gray-900 dark:text-gray-100">
                  {new Date(asset.updated_at ?? asset.created_at).toLocaleDateString('en-CA')}
                </p>
              </div>
              {#if asset.width.Valid && asset.height.Valid}
                <div class="rounded-xl bg-gray-50 px-3 py-3 dark:bg-gray-800">
                  <p class="mb-1 text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">Dimensions</p>
                  <p class="text-sm font-semibold text-gray-900 dark:text-gray-100">{asset.width.Int64} × {asset.height.Int64}</p>
                </div>
              {/if}
            </div>
          </div>

          <!-- Tags -->
          <div>
            <p class="mb-3 text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">Tags</p>
            <div class="flex flex-wrap gap-1.5">
              {#each tags as tag}
                <Chip
                  label={tag}
                  onremove={authStore.role !== 'viewer' ? () => removeTag(tag) : undefined}
                />
              {/each}

              {#if authStore.role !== 'viewer'}
                {#if showTagInput}
                  <div class="relative">
                    <form onsubmit={(e) => { e.preventDefault(); addTag(tagInput) }}>
                      <input
                        bind:value={tagInput}
                        oninput={updateSuggestions}
                        placeholder="Add tag…"
                        class="w-28 rounded-full border border-indigo-400 bg-white px-2.5 py-0.5 text-xs text-gray-900 outline-none dark:bg-gray-800 dark:text-gray-100"
                        onblur={() => { setTimeout(() => { showTagInput = false; tagSuggestions = [] }, 150) }}
                      />
                    </form>
                    {#if tagSuggestions.length > 0}
                      <ul class="absolute left-0 top-full z-20 mt-0.5 w-36 rounded-lg border border-gray-200 bg-white py-1 shadow-md dark:border-gray-700 dark:bg-gray-900">
                        {#each tagSuggestions as s}
                          <li>
                            <button
                              class="w-full px-3 py-1 text-left text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-gray-800"
                              onmousedown={() => addTag(s)}
                            >{s}</button>
                          </li>
                        {/each}
                      </ul>
                    {/if}
                  </div>
                {:else}
                  <button
                    class="rounded-full border border-dashed border-gray-300 px-2.5 py-0.5 text-xs text-gray-400 hover:border-indigo-400 hover:text-indigo-600 dark:border-gray-600 dark:text-gray-500 dark:hover:border-indigo-500 dark:hover:text-indigo-400"
                    onclick={() => { showTagInput = true }}
                  >+ Add Tag</button>
                {/if}
              {/if}
            </div>
          </div>

          <!-- Folder -->
          {#if authStore.role !== 'viewer' || activeProject}
            <div>
              <div class="mb-3 flex items-center justify-between">
                <p class="text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">Folder</p>
                {#if authStore.role !== 'viewer'}
                  <button
                    class="text-xs text-indigo-600 hover:underline dark:text-indigo-400"
                    onclick={() => { showProjectPicker = !showProjectPicker }}
                  >
                    {activeProject ? 'Change' : 'Assign'}
                  </button>
                {/if}
              </div>
              <div class="relative">
                {#if activeProject}
                  <span
                    class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium"
                    style="background-color: {activeProject.color.Valid ? activeProject.color.String + '22' : '#f3f4f6'}; color: {activeProject.color.Valid ? activeProject.color.String : '#6b7280'}"
                  >
                    <ColorDot color={activeProject.color.Valid ? activeProject.color.String : '#9ca3af'} size="sm" />
                    {activeProject.name}
                  </span>
                {:else}
                  <span class="text-xs text-gray-400 dark:text-gray-500">Not assigned</span>
                {/if}

                {#if showProjectPicker}
                  <div class="absolute left-0 top-full z-20 mt-1 min-w-[180px] rounded-xl border border-gray-200 bg-white py-1 shadow-lg dark:border-gray-700 dark:bg-gray-900">
                    <button
                      class="flex w-full items-center gap-2 px-3 py-1.5 text-sm text-gray-500 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800"
                      onclick={() => assignProject(null)}
                    >
                      <span class="h-2.5 w-2.5 rounded-full border border-gray-300"></span>
                      None
                    </button>
                    {#each projectsStore.projects as p}
                      <button
                        class="flex w-full items-center gap-2 px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-gray-800"
                        onclick={() => assignProject(p.id)}
                      >
                        <ColorDot color={p.color.Valid ? p.color.String : '#9ca3af'} size="sm" />
                        {p.name}
                      </button>
                    {/each}
                  </div>
                {/if}
              </div>
            </div>
          {/if}
        </div>

      <!-- ═══ VARIANTS TAB ═══ -->
      {:else if activeTab === 'variants'}
        <div class="flex flex-col">
          <!-- Pill sub-tabs -->
          <div class="flex flex-wrap gap-2 border-b border-gray-100 px-5 py-3 dark:border-gray-800">
            {#each variantSubTabs as tab}
              <button
                type="button"
                class="rounded-full px-3 py-1 text-xs font-medium transition-colors {activeVariantTab === tab.id
                  ? 'bg-indigo-600 text-white dark:bg-indigo-500'
                  : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-800 dark:text-gray-400 dark:hover:bg-gray-700'}"
                onclick={() => { activeVariantTab = tab.id; createError = ''; createSuccess = '' }}
              >
                {tab.label}
              </button>
            {/each}
          </div>

          <!-- Feedback -->
          {#if createError}
            <div class="mx-5 mt-4 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-900/30 dark:text-red-300">
              {createError}
            </div>
          {/if}
          {#if createSuccess}
            <div class="mx-5 mt-4 rounded-lg bg-green-50 px-4 py-3 text-sm text-green-700 dark:bg-green-900/30 dark:text-green-300">
              {createSuccess}
            </div>
          {/if}

          <div class="px-5 py-4">

            <!-- All variants grid -->
            {#if activeVariantTab === 'all'}
              {#if variantsLoading}
                <div class="flex justify-center py-12">
                  <Spinner size="md" />
                </div>
              {:else if variants.length === 0}
                <div class="flex flex-col items-center gap-3 py-12 text-center text-gray-400">
                  <Inbox class="h-10 w-10" />
                  <p class="text-sm">No variants yet.</p>
                  {#if authStore.role !== 'viewer' && (isImage || isVideo)}
                    <p class="text-xs">Use the tabs above to create a variant.</p>
                  {/if}
                </div>
              {:else}
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

                  <!-- Create variant card -->
                  {#if authStore.role !== 'viewer' && (isImage || isVideo)}
                    <button
                      type="button"
                      class="flex h-full min-h-[120px] flex-col items-center justify-center gap-2 rounded-xl border border-dashed border-gray-300 text-gray-400 transition-colors hover:border-indigo-400 hover:text-indigo-500 dark:border-gray-600 dark:text-gray-600 dark:hover:border-indigo-500 dark:hover:text-indigo-400"
                      onclick={() => { activeVariantTab = isImage ? 'resize' : 'video' }}
                    >
                      <span class="text-2xl leading-none">+</span>
                      <span class="text-xs">Create Variant</span>
                    </button>
                  {/if}
                </div>
              {/if}

            <!-- Resize -->
            {:else if activeVariantTab === 'resize'}
              <div class="space-y-5">
                {#if previewUrl}
                  <div class="flex justify-center rounded-xl border border-gray-200 bg-gray-50 p-3 dark:border-gray-700 dark:bg-gray-800" style="min-height:120px">
                    <img src={previewUrl} alt="Preview" class="max-h-48 max-w-full rounded object-contain" />
                  </div>
                {:else}
                  <div class="flex items-center justify-center rounded-xl border border-dashed border-gray-300 bg-gray-50 py-8 text-xs text-gray-400 dark:border-gray-600 dark:bg-gray-800">
                    Preview will appear after changing parameters
                  </div>
                {/if}

                <div class="grid grid-cols-2 gap-4">
                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">Width (px)</label>
                    <input type="number" min="1" max="8000" bind:value={resizeWidth} oninput={updatePreview}
                      class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-indigo-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100" />
                  </div>
                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">Height <span class="text-gray-400">(0=auto)</span></label>
                    <input type="number" min="0" max="8000" bind:value={resizeHeight} oninput={updatePreview}
                      class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-indigo-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100" />
                  </div>
                </div>

                <div>
                  <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">Fit</label>
                  <div class="flex gap-2">
                    {#each ['contain', 'cover', 'fill'] as f}
                      <button type="button"
                        class="flex-1 rounded-lg border py-2 text-xs font-medium transition-colors {resizeFit === f
                          ? 'border-indigo-500 bg-indigo-50 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300'
                          : 'border-gray-300 text-gray-600 hover:border-gray-400 dark:border-gray-600 dark:text-gray-400'}"
                        onclick={() => { resizeFit = f as typeof resizeFit; updatePreview() }}
                      >{f}</button>
                    {/each}
                  </div>
                </div>

                <div>
                  <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">Quality: {resizeQuality}%</label>
                  <input type="range" min="1" max="100" bind:value={resizeQuality} oninput={updatePreview}
                    class="w-full accent-indigo-500" />
                </div>

                <div>
                  <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">Format</label>
                  <div class="flex gap-2">
                    {#each ['jpeg', 'png', 'tiff'] as fmt}
                      <button type="button"
                        class="flex-1 rounded-lg border py-2 text-xs font-medium transition-colors {resizeFormat === fmt
                          ? 'border-indigo-500 bg-indigo-50 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300'
                          : 'border-gray-300 text-gray-600 hover:border-gray-400 dark:border-gray-600 dark:text-gray-400'}"
                        onclick={() => { resizeFormat = fmt as typeof resizeFormat; updatePreview() }}
                      >{fmt.toUpperCase()}</button>
                    {/each}
                  </div>
                </div>

                <Button disabled={creating || authStore.role === 'viewer'} onclick={() => handleCreate('resize', { width: resizeWidth || undefined, height: resizeHeight || undefined, fit: resizeFit, quality: resizeQuality, format: resizeFormat })} class="w-full">
                  {creating ? 'Queuing…' : 'Create Resize Variant'}
                </Button>
              </div>

            <!-- Watermark -->
            {:else if activeVariantTab === 'watermark'}
              <div class="space-y-5">
                <div class="grid grid-cols-2 gap-4">
                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">Opacity ({watermarkOpacity}%)</label>
                    <input type="range" min="1" max="100" bind:value={watermarkOpacity} class="w-full accent-indigo-500" />
                  </div>
                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">Quality: {watermarkQuality}%</label>
                    <input type="range" min="1" max="100" bind:value={watermarkQuality} class="w-full accent-indigo-500" />
                  </div>
                </div>

                <div>
                  <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">Format</label>
                  <div class="flex gap-2">
                    {#each ['jpeg', 'png', 'tiff'] as fmt}
                      <button type="button"
                        class="flex-1 rounded-lg border py-2 text-xs font-medium transition-colors {watermarkFormat === fmt
                          ? 'border-indigo-500 bg-indigo-50 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300'
                          : 'border-gray-300 text-gray-600 hover:border-gray-400 dark:border-gray-600 dark:text-gray-400'}"
                        onclick={() => { watermarkFormat = fmt as typeof watermarkFormat }}
                      >{fmt.toUpperCase()}</button>
                    {/each}
                  </div>
                </div>

                <Button disabled={creating || authStore.role === 'viewer'} onclick={() => handleCreate('watermark', { opacity: watermarkOpacity, quality: watermarkQuality, format: watermarkFormat })} class="w-full">
                  {creating ? 'Queuing…' : 'Create Watermark Variant'}
                </Button>
              </div>

            <!-- Convert -->
            {:else if activeVariantTab === 'convert'}
              <div class="space-y-5">
                <div>
                  <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">Output Format</label>
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
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">Quality: {convertQuality}%</label>
                    <input type="range" min="1" max="100" bind:value={convertQuality} class="w-full accent-indigo-500" />
                  </div>
                {/if}

                <Button disabled={creating || authStore.role === 'viewer'} onclick={() => handleCreate('convert', { format: convertFormat, quality: convertQuality })} class="w-full">
                  {creating ? 'Queuing…' : `Convert to ${convertFormat.toUpperCase()}`}
                </Button>
              </div>

            <!-- Crop -->
            {:else if activeVariantTab === 'crop'}
              <div class="space-y-5">
                {#if asset.width.Valid && asset.height.Valid}
                  <p class="text-xs text-gray-400 dark:text-gray-500">Original: {asset.width.Int64} × {asset.height.Int64} px</p>
                {/if}

                <div class="grid grid-cols-2 gap-4">
                  {#each [['X offset', 'cropX'], ['Y offset', 'cropY'], ['Width', 'cropWidth'], ['Height', 'cropHeight']] as [label, key]}
                    <div>
                      <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">{label}</label>
                      <input type="number" min="0"
                        value={key === 'cropX' ? cropX : key === 'cropY' ? cropY : key === 'cropWidth' ? cropWidth : cropHeight}
                        oninput={(e) => {
                          const v = parseInt((e.currentTarget as HTMLInputElement).value) || 0
                          if (key === 'cropX') cropX = v
                          else if (key === 'cropY') cropY = v
                          else if (key === 'cropWidth') cropWidth = v
                          else cropHeight = v
                        }}
                        class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-indigo-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100" />
                    </div>
                  {/each}
                </div>

                <div>
                  <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">Format</label>
                  <div class="flex gap-2">
                    {#each ['jpeg', 'png'] as fmt}
                      <button type="button"
                        class="flex-1 rounded-lg border py-2 text-xs font-medium transition-colors {cropFormat === fmt
                          ? 'border-indigo-500 bg-indigo-50 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300'
                          : 'border-gray-300 text-gray-600 hover:border-gray-400 dark:border-gray-600 dark:text-gray-400'}"
                        onclick={() => { cropFormat = fmt as typeof cropFormat }}
                      >{fmt.toUpperCase()}</button>
                    {/each}
                  </div>
                </div>

                <Button disabled={creating || authStore.role === 'viewer'} onclick={() => handleCreate('crop', { x: cropX, y: cropY, width: cropWidth, height: cropHeight, format: cropFormat, quality: 85 })} class="w-full">
                  {creating ? 'Queuing…' : 'Create Crop Variant'}
                </Button>
              </div>

            <!-- Bg Remove -->
            {:else if activeVariantTab === 'bg_remove'}
              <div class="space-y-5">
                <div class="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300">
                  <p class="mb-1 font-medium">Requires Remove.bg API key</p>
                  <p class="text-xs">Set <code class="rounded bg-amber-100 px-1 dark:bg-amber-800/50">REMOVEBG_API_KEY</code> in your server environment. Returns a transparent PNG.</p>
                </div>

                <Button disabled={creating || authStore.role === 'viewer'} onclick={() => handleCreate('bg_remove', {})} class="w-full">
                  {#if creating}<Spinner size="sm" />{/if}
                  {creating ? 'Queuing…' : 'Remove Background'}
                </Button>
              </div>

            <!-- Video -->
            {:else if activeVariantTab === 'video'}
              <div class="space-y-6">
                <div class="space-y-4 rounded-xl border border-gray-200 p-4 dark:border-gray-700">
                  <h3 class="text-sm font-semibold text-gray-800 dark:text-gray-200">Extract Frame</h3>
                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">Timestamp (seconds)</label>
                    <input type="number" min="0" step="0.1" bind:value={videoTimestamp}
                      class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-indigo-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100" />
                  </div>
                  <Button disabled={creating || authStore.role === 'viewer'} onclick={() => handleCreate('video_thumbnail', { timestamp: videoTimestamp })} class="w-full">
                    {creating ? 'Queuing…' : 'Extract Frame'}
                  </Button>
                </div>

                <div class="space-y-4 rounded-xl border border-gray-200 p-4 dark:border-gray-700">
                  <h3 class="text-sm font-semibold text-gray-800 dark:text-gray-200">Transcode Video</h3>
                  <p class="text-xs text-gray-500 dark:text-gray-400">Heavy operation — max 2 concurrent transcodes. Requires ffmpeg.</p>

                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">Output Format</label>
                    <div class="flex gap-2">
                      {#each ['mp4', 'webm'] as fmt}
                        <button type="button"
                          class="flex-1 rounded-lg border py-2 text-xs font-medium transition-colors {transcodeFormat === fmt
                            ? 'border-indigo-500 bg-indigo-50 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300'
                            : 'border-gray-300 text-gray-600 hover:border-gray-400 dark:border-gray-600 dark:text-gray-400'}"
                          onclick={() => { transcodeFormat = fmt as typeof transcodeFormat }}
                        >{fmt.toUpperCase()}</button>
                      {/each}
                    </div>
                  </div>

                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">Resolution <span class="text-gray-400">(optional)</span></label>
                    <select bind:value={transcodeResolution}
                      class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-indigo-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100">
                      <option value="">Original</option>
                      <option value="1080p">1080p</option>
                      <option value="720p">720p</option>
                      <option value="480p">480p</option>
                    </select>
                  </div>

                  <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                    <input type="checkbox" bind:checked={transcodeStripAudio} class="rounded" />
                    Strip audio track
                  </label>

                  <Button disabled={creating || authStore.role === 'viewer'} onclick={() => handleCreate('video_transcode', { format: transcodeFormat, resolution: transcodeResolution || undefined, strip_audio: transcodeStripAudio })} class="w-full">
                    {creating ? 'Queuing…' : 'Transcode Video'}
                  </Button>
                </div>
              </div>
            {/if}

          </div>
        </div>

      <!-- ═══ ACTIONS TAB ═══ -->
      {:else if activeTab === 'actions'}
        <div class="px-5 py-5 space-y-6">

          <!-- Quick Actions -->
          <div>
            <p class="mb-3 text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">Quick Actions</p>
            <div class="space-y-2">
              <button
                class="flex w-full items-center gap-3 rounded-xl border border-gray-200 px-4 py-3 text-sm text-gray-700 transition-colors hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800"
                onclick={() => { showShareModal = true }}
              >
                <Share class="h-4 w-4 shrink-0 text-gray-400 dark:text-gray-500" />
                Share…
              </button>
              <button
                class="flex w-full items-center gap-3 rounded-xl border border-gray-200 px-4 py-3 text-sm transition-colors hover:bg-gray-50 dark:border-gray-700 dark:hover:bg-gray-800 {linkCopied ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-700 dark:text-gray-300'}"
                onclick={copyShareLink}
              >
                {#if linkCopied}
                  <Copy class="h-4 w-4 shrink-0 text-emerald-500" />
                  Link copied!
                {:else}
                  <Link class="h-4 w-4 shrink-0 text-gray-400 dark:text-gray-500" />
                  Copy Share Link
                {/if}
              </button>
            </div>
          </div>

          <!-- Export -->
          <div>
            <p class="mb-3 text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">Export</p>
            <p class="mb-2 text-xs font-medium text-gray-500 dark:text-gray-400">Export Format</p>
            <div class="flex gap-2">
              <select
                bind:value={exportFormat}
                class="flex-1 rounded-xl border border-gray-200 bg-white px-3 py-2.5 text-sm text-gray-700 focus:border-indigo-400 focus:outline-none dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
              >
                <option value="original">Original</option>
                <option value="jpeg">JPEG</option>
                <option value="png">PNG</option>
                <option value="webp">WebP</option>
                <option value="pdf">PDF</option>
              </select>
              <a
                href={assetApi.fileUrl(asset.id)}
                download={asset.original_filename}
                class="flex items-center justify-center rounded-xl bg-indigo-600 px-3.5 py-2.5 text-white transition-colors hover:bg-indigo-700"
                aria-label="Download"
              >
                <Download class="h-4 w-4" />
              </a>
            </div>
          </div>

        </div>
      {/if}

    </div>
  </div>
{/if}

{#if showShareModal && asset}
  <ShareModal
    bind:open={showShareModal}
    targets={shareTargets}
    onclose={() => { showShareModal = false }}
  />
{/if}
