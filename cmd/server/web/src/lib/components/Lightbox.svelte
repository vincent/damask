<script lang="ts">
  import {
    assetApi,
    variantApi,
    mimeCategory,
    type Asset,
    type CoveringWorkflow,
    type Variant,
  } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { assetsStore, sseEvents } from '$lib/stores/assets.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import ShareModal from './ShareModal.svelte'
  import PreviewToolbar from './ui/PreviewToolbar.svelte'
  import { Inbox, RefreshCw, Share, Upload, Zap } from '@lucide/svelte'
  import AssetTags from './AssetTags.svelte'
  import AssetMetadata from './AssetMetadata.svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import ProgressBar from '$lib/components/ui/ProgressBar.svelte'
  import SharedAsset from './SharedAsset.svelte'
  import VariantsTool, { type VariantTab } from './variants/VariantsTool.svelte'
  import AssetProject from './AssetProject.svelte'
  import Pills from './ui/Pills.svelte'
  import Feedback from './ui/Feedback.svelte'
  import AssetVariantsGrid from './AssetVariantsGrid.svelte'
  import VariantSharingHeader from './variants/VariantSharingHeader.svelte'
  import VariantPromoteForm from './variants/VariantPromoteForm.svelte'
  import CoveringWorkflowBanner from './variants/CoveringWorkflowBanner.svelte'
  import CreateVariantAutomationModal from './variants/CreateVariantAutomationModal.svelte'
  import AssetMetadataPills from './AssetMetadataPills.svelte'
  import AssetExportImage from './AssetExportImage.svelte'
  import AssetDeleteButton from './AssetDeleteButton.svelte'
  import AssetCustomFields from './AssetCustomFields.svelte'
  import AssetCollections from './AssetCollections.svelte'
  import VersionHistory from './VersionHistory.svelte'
  import UploadVersionModal from './UploadVersionModal.svelte'
  import AssetActivity from './AssetActivity.svelte'
  import { undoStore } from '$lib/stores/undo.svelte'
  import { RenameAsset } from '$lib/commands/RenameAsset'
  import InlineEditForm from '$lib/components/ui/InlineEditForm.svelte'
  import { fly } from 'svelte/transition'
  import { cubicOut } from 'svelte/easing'
  import SubSectionTitle from './ui/SubSectionTitle.svelte'
  import AssetComments from './AssetComments.svelte'
  import MediaTagsTab from './MediaTagsTab.svelte'
  import TextTrackPanel from './text-tracks/TextTrackPanel.svelte'
  import { ALL_VARIANT_TOOLS } from './variants/toolDefs'
  import VariantToolPanel from './variants/VariantToolPanel.svelte'
  import VariantToolSidebar from './variants/VariantToolSidebar.svelte'
  import VariantDraftSession from './variants/VariantDraftSession.svelte'
  import Backdrop from './ui/Backdrop.svelte'
  import { ASSET_BACKGROUND_COLORS } from '$lib/stores/shared'
  import { m } from '$lib/paraglide/messages'
  import { useShortcuts } from '$lib/shortcuts'
  import {
    isAudio as mimeIsAudio,
    isVideo as mimeIsVideo,
  } from '$lib/utils/mime'
  import { viewportStore } from '$lib/stores/viewport.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'

  interface Props {
    asset: Asset | null
    onclose: () => void
    ondeleted: (id: string) => void
    onassetupdated?: (updated: Asset) => void
  }

  let {
    asset = $bindable(),
    onclose,
    ondeleted,
    onassetupdated,
  }: Props = $props()

  // --- Zoom/rotate refs bound to SharedAsset ---
  let zoomIn = $state<(() => void) | undefined>(undefined)
  let zoomOut = $state<(() => void) | undefined>(undefined)
  let zoomReset = $state<(() => void) | undefined>(undefined)
  let zoomWheel = $state<((e: WheelEvent) => void) | undefined>(undefined)
  let rotateRight = $state<(() => void) | undefined>(undefined)

  // --- Toolbar ---
  let showToolbar = $state<(() => void) | undefined>(undefined)
  let previewContainer = $state<HTMLElement | null>(null)

  useShortcuts({
    'lightbox.zoom-in': () => zoomIn?.(),
    'lightbox.zoom-out': () => zoomOut?.(),
    'view.zoom-reset': () => {
      if (asset) zoomReset?.()
    },
  })

  // --- Panel tabs ---
  const panelTabs = {
    details: { label: m.tab_details(), icon: null },
    tags: { label: m.media_tags_tab_label(), icon: null },
    variants: { label: m.tab_variants(), icon: null },
    text: { label: m.text_tracks_panel_title(), icon: null },
    comments: { label: m.tab_comments(), icon: null },
    history: { label: m.tab_history(), icon: null },
    activity: { label: m.tab_activity(), icon: null },
    actions: { label: m.tab_actions(), icon: null },
  }
  type PanelTab = keyof typeof panelTabs
  let activeTab = $state<PanelTab>('details')
  let isMediaTagsAsset = $derived(
    asset ? mimeIsAudio(asset.mime_type) || mimeIsVideo(asset.mime_type) : false
  )

  $effect(() => {
    if (activeTab !== 'variants') selectedVariant = null
  })

  let activeVariantTab = $state<VariantTab>('all')
  let selectedTool = $state<VariantTab | null>(null)

  // --- Asset state ---
  let showShareModal = $state(false)
  let showUploadVersionModal = $state(false)
  let linkCopied = $state(false)
  let renamingAsset = $state(false)
  let renameBusy = $state(false)
  let regenThumbLoading = $state(false)

  function stemOf(filename: string) {
    const dot = filename.lastIndexOf('.')
    return dot > 0 ? filename.slice(0, dot) : filename
  }

  function extOf(filename: string) {
    const dot = filename.lastIndexOf('.')
    return dot > 0 ? filename.slice(dot) : ''
  }

  async function submitRename(stem: string) {
    if (!asset) return
    if (stem === stemOf(asset.original_filename)) {
      renamingAsset = false
      return
    }
    renameBusy = true
    try {
      const before = asset.original_filename
      await undoStore.execute(new RenameAsset(asset.id, before, stem))
      const updated = {
        ...asset,
        original_filename:
          assetsStore.assets.find((a) => a.id === asset!.id)
            ?.original_filename ?? asset.original_filename,
      }
      asset = updated
      onassetupdated?.(updated)
    } finally {
      renameBusy = false
      renamingAsset = false
    }
  }

  async function handleDerivedAssetOpen(assetId: string) {
    const nextAsset =
      assetsStore.assets.find((candidate) => candidate.id === assetId) ??
      (await assetApi.get(assetId))

    asset = nextAsset
    onassetupdated?.(nextAsset)
  }

  // --- Asset detail (enriched, includes created_by + authors) ---
  let assetDetail = $state<typeof asset>(null)

  async function loadAssetDetail() {
    if (!asset) return
    try {
      assetDetail = await assetApi.get(asset.id)
    } catch {
      // silently ignore — fall back to the prop
    }
  }

  // --- Variant state ---
  let variants = $state<Variant[]>([])
  let coveringWorkflow = $state<CoveringWorkflow | null>(null)
  let variantsLoading = $state(false)
  let showAutomationModal = $state(false)
  let creating = $state(false)
  let pendingVariantAssetId = $state<string | null>(null)
  let variantRefreshProgress = $state(0)
  let createError = $state('')
  let createSuccess = $state('')
  let variantRefreshTimer: ReturnType<typeof setInterval> | null = null
  let variantRefreshTimeout: ReturnType<typeof setTimeout> | null = null
  let variantPanelState = $state<
    { mode: 'list' } | { mode: 'promote'; variant: Variant }
  >({ mode: 'list' })

  // --- Draft overlay (replaces asset preview when a draft tool is active) ---
  let draftSessionRef = $state<
    ReturnType<typeof VariantDraftSession> | undefined
  >(undefined)
  let showDraftOverlay = $state(false)
  let draftAssetId = $state<string | null>(null)
  let draftDoneTimer: ReturnType<typeof setTimeout> | null = null

  function handleDraftStarted(nonce: string) {
    draftAssetId = asset?.id ?? null
    showDraftOverlay = true
    // Give Svelte a tick to mount VariantDraftSession before adding the draft
    Promise.resolve().then(() => draftSessionRef?.addDraft(nonce))
  }

  function handleDraftDone() {
    if (draftDoneTimer) clearTimeout(draftDoneTimer)
    draftDoneTimer = setTimeout(() => {
      showDraftOverlay = false
      selectedTool = null
      draftDoneTimer = null
      loadVariants()
    }, 1200)
  }

  function handleDraftAddMore() {
    // session stays open; tool panel form is already visible
  }

  $effect(() => {
    if (!asset || selectedTool === null) {
      showDraftOverlay = false
      if (draftDoneTimer) {
        clearTimeout(draftDoneTimer)
        draftDoneTimer = null
      }
    }
  })

  const category = $derived(asset ? mimeCategory(asset.mime_type) : 'document')
  const isImage = $derived(asset?.mime_type?.startsWith('image/') ?? false)
  const isVideo = $derived(asset ? mimeIsVideo(asset.mime_type) : false)
  const isAudio = $derived(asset ? mimeIsAudio(asset.mime_type) : false)
  const visibleVariantTools = $derived.by(() => {
    if (!asset) return []
    const mimeType = asset.mime_type
    return ALL_VARIANT_TOOLS.filter((tool) => tool.showFor(mimeType))
  })
  const automatableVariants = $derived(
    variants.filter((variant) => variant.type !== 'manual')
  )
  const canCreateVariantAutomation = $derived(
    authStore.role === 'owner' &&
      automatableVariants.length > 0 &&
      !coveringWorkflow
  )

  // --- Variant preview selection ---
  let selectedVariant = $state<Variant | null>(null)

  function deriveVariantMime(v: Variant): string {
    // Derive from storage_key extension — thumbnail_content_type is the poster
    // image mime, not the variant file mime, so it can't be used here.
    const key = v.storage_key ?? ''
    if (key.endsWith('.mp4') || key.endsWith('.m4v')) return 'video/mp4'
    if (key.endsWith('.webm')) return 'video/webm'
    if (key.endsWith('.mov')) return 'video/quicktime'
    if (key.endsWith('.mp3')) return 'audio/mpeg'
    if (key.endsWith('.aac')) return 'audio/aac'
    if (key.endsWith('.ogg')) return 'audio/ogg'
    if (key.endsWith('.wav')) return 'audio/wav'
    if (key.endsWith('.flac')) return 'audio/flac'
    if (key.endsWith('.pdf')) return 'application/pdf'
    if (key.endsWith('.png')) return 'image/png'
    if (key.endsWith('.jpg') || key.endsWith('.jpeg')) return 'image/jpeg'
    if (key.endsWith('.webp')) return 'image/webp'
    if (key.endsWith('.gif')) return 'image/gif'
    if (key.endsWith('.avif')) return 'image/avif'
    // Fallback: infer from variant type string
    const t = v.type ?? ''
    if (t.startsWith('video_'))
      return asset?.mime_type?.startsWith('video/')
        ? asset.mime_type
        : 'video/mp4'
    if (t.startsWith('audio_'))
      return asset?.mime_type?.startsWith('audio/')
        ? asset.mime_type
        : 'audio/mpeg'
    if (t === 'video_capture_image') return 'image/jpeg'
    return asset?.mime_type ?? 'application/octet-stream'
  }

  const previewMimeType = $derived(
    selectedVariant
      ? deriveVariantMime(selectedVariant)
      : (asset?.mime_type ?? '')
  )
  const previewCategory = $derived(mimeCategory(previewMimeType))
  const previewThumbUrl = $derived(
    asset
      ? selectedVariant
        ? variantApi.thumbUrl(asset.id, selectedVariant.id)
        : assetApi.thumbUrl(asset.id)
      : ''
  )
  const previewFileUrl = $derived(
    asset
      ? selectedVariant
        ? variantApi.fileUrl(asset.id, selectedVariant.id)
        : assetApi.fileUrl(asset.id)
      : ''
  )

  $effect(() => {
    if (!asset) {
      variants = []
      coveringWorkflow = null
      showAutomationModal = false
      variantPanelState = { mode: 'list' }
      selectedVariant = null
      selectedTool = null
      assetDetail = null
      return
    }
    selectedVariant = null
    selectedTool = null
    loadVariants()
    loadAssetDetail()
  })

  async function loadVariants() {
    if (!asset) return
    variantsLoading = true
    try {
      const response = await variantApi.list(asset.id)
      variants = response?.variants ?? []
      coveringWorkflow = response?.covering_workflow ?? null
    } catch {
      // silently ignore
    } finally {
      variantsLoading = false
    }
  }

  function replaceVariant(updatedVariant: Variant) {
    variants = variants.map((variant) =>
      variant.id === updatedVariant.id ? updatedVariant : variant
    )
    if (selectedVariant?.id === updatedVariant.id) {
      selectedVariant = updatedVariant
    }
  }

  function replaceVariants(nextVariants: Variant[]) {
    variants = nextVariants
    if (selectedVariant) {
      selectedVariant =
        nextVariants.find((variant) => variant.id === selectedVariant?.id) ??
        null
    }
  }

  function clearVariantRefreshCountdown() {
    if (variantRefreshTimer) {
      clearInterval(variantRefreshTimer)
      variantRefreshTimer = null
    }
    if (variantRefreshTimeout) {
      clearTimeout(variantRefreshTimeout)
      variantRefreshTimeout = null
    }
    variantRefreshProgress = 0
  }

  function scheduleVariantRefresh(assetId: string) {
    clearVariantRefreshCountdown()
    variantRefreshProgress = 100

    const startedAt = Date.now()
    const durationMs = 10000

    variantRefreshTimer = setInterval(() => {
      const elapsed = Date.now() - startedAt
      const remainingRatio = Math.max(0, 1 - elapsed / durationMs)
      variantRefreshProgress = remainingRatio * 100
    }, 100)

    variantRefreshTimeout = setTimeout(async () => {
      clearVariantRefreshCountdown()
      if (!asset || asset.id !== assetId) return
      await loadVariants()
    }, durationMs)
  }

  const activeProject = $derived(
    asset?.project_id
      ? (projectsStore.projects.find((p) => p.id === asset?.project_id) ?? null)
      : null
  )

  const shareTargets = $derived(
    asset
      ? [{ type: 'asset' as const, id: asset.id, label: m.selected_asset() }]
      : []
  )

  async function copyShareLink() {
    if (!asset) return
    try {
      await navigator.clipboard.writeText(
        window.location.origin + `/s/${asset.id}`
      )
      linkCopied = true
      setTimeout(() => {
        linkCopied = false
      }, 2000)
    } catch {
      /* silently ignore */
    }
  }

  // --- Variant helpers ---
  async function handleCreate(type: string, params: object) {
    if (!asset) return
    creating = true
    createError = ''
    createSuccess = ''
    try {
      const result = await variantApi.create(asset.id, type, params)
      pendingVariantAssetId = asset.id
      createSuccess = `Queued (job ${result.job_id.slice(0, 8)}). Waiting for completion…`
      toastStore.show(createSuccess, 'success')
      activeTab = 'variants'
    } catch (e: unknown) {
      pendingVariantAssetId = null
      createError = e instanceof Error ? e.message : m.variant_create_failed()
      toastStore.show(createError, 'error')
    } finally {
      creating = false
    }
  }

  $effect(() => {
    if (!asset) {
      clearVariantRefreshCountdown()
      pendingVariantAssetId = null
      return
    }

    const ev = sseEvents.last
    if (pendingVariantAssetId !== asset.id) return
    if (ev?.type !== 'variant_ready' || ev.asset_id !== asset.id) return

    pendingVariantAssetId = null
    activeVariantTab = 'all'
    selectedTool = null
    createSuccess = 'Almost ready. Refreshing shortly…'
    scheduleVariantRefresh(asset.id)
  })

  $effect(() => {
    if (!asset) return

    const ev = sseEvents.last
    if (pendingVariantAssetId !== asset.id) return
    if (ev?.type !== 'variant_failed' || ev.asset_id !== asset.id) return

    clearVariantRefreshCountdown()
    pendingVariantAssetId = null
    createSuccess = ''
    createError = ev.error || m.variant_create_failed()
  })

  $effect(() => {
    return () => {
      clearVariantRefreshCountdown()
      activeVariantTab = 'all'
      createSuccess = ''
      createError = ''
    }
  })

  function handleClose(e: MouseEvent) {
    if (showDraftOverlay) return
    const src = e.target as HTMLElement
    if (src.classList.contains('asset-preview-full')) return
    if (src.classList.contains('asset-preview-toolbar')) return
    onclose?.()
  }

  function handleWindowKeydown(event: KeyboardEvent) {
    if (
      !asset ||
      event.key !== 'Escape' ||
      !viewportStore.isXl ||
      selectedTool === null
    )
      return
    selectedTool = null
  }

  async function handleRegenerateThumbnail() {
    if (!asset) return
    regenThumbLoading = true
    try {
      await assetApi.regenerateThumbnail(asset.id)
    } catch {
      /* silently ignore */
    } finally {
      regenThumbLoading = false
    }
  }

  async function handleDeleteVariant(variantId: string) {
    if (!asset) return
    try {
      await variantApi.delete(asset.id, variantId)
      variants = variants.filter((v) => v.id !== variantId)
    } catch {
      /* silently ignore */
    }
  }

  function openPromoteVariant(variant: Variant) {
    variantPanelState = { mode: 'promote', variant }
  }

  async function handleThumbnailUpdated() {
    if (!asset) return
    asset = { ...asset, thumbnail_key: 'updated' }
    onassetupdated?.(asset)
  }

  async function handleRerunRefresh() {
    await loadVariants()
  }

  // Visible variant sub-tabs based on asset type
  const variantSubTabs = $derived([
    { id: 'all' as VariantTab, label: m.all() },
    ...(isImage
      ? [
          { id: 'resize' as VariantTab, label: m.resize() },
          { id: 'watermark' as VariantTab, label: m.watermark() },
          { id: 'convert' as VariantTab, label: m.convert() },
          { id: 'smart_crop' as VariantTab, label: m.smart_crop() },
          { id: 'crop' as VariantTab, label: m.crop() },
          { id: 'bg_remove' as VariantTab, label: m.bg_remove() },
          {
            id: 'image_with_prompt' as VariantTab,
            label: 'AI Transform',
          },
        ]
      : []),
    ...(isVideo
      ? [
          { id: 'video_transcode' as VariantTab, label: m.transcode() },
          { id: 'video_watermark' as VariantTab, label: m.watermark() },
          { id: 'video_capture_image' as VariantTab, label: m.thumbnail() },
          { id: 'audio_extract' as VariantTab, label: m.audio_extract_tab() },
        ]
      : []),
    ...(isAudio
      ? [
          {
            id: 'audio_transcode' as VariantTab,
            label: m.audio_transcode_tab(),
          },
          {
            id: 'audio_normalize' as VariantTab,
            label: m.audio_normalize_tab(),
          },
        ]
      : []),
  ])
</script>

<svelte:window onkeydown={handleWindowKeydown} />

{#if asset}
  <Backdrop class="asset-lightbox-bg w-screen" {onclose}>
    <div
      bind:this={previewContainer}
      class="asset-preview-container fixed inset-y-0 left-0 hidden w-[75%] flex-col md:flex"
      role="button"
      tabindex="-1"
      onclick={handleClose}
      onkeydown={(e) => e.key === 'Enter' && onclose()}
      onwheel={(e) => zoomWheel?.(e)}
      onmousemove={() => showToolbar?.()}
      aria-label={m.close()}
    >
      <div class="relative grid min-h-0 flex-1 place-items-center p-40">
        {#if showDraftOverlay && draftAssetId}
          <div
            class="pointer-events-auto absolute inset-y-0 right-0 overflow-y-auto p-8"
            style="left: calc(60px + 350px);"
          >
            <div class="flex min-h-full flex-col items-center justify-center">
              <div class="w-full" style="max-width: 560px;">
                <VariantDraftSession
                  bind:this={draftSessionRef}
                  assetId={draftAssetId}
                  gridMode={true}
                  onDone={handleDraftDone}
                  onAddMore={handleDraftAddMore}
                />
              </div>
            </div>
          </div>
        {:else}
          <SharedAsset
            asset={{ ...asset, mime_type: previewMimeType }}
            category={previewCategory}
            thumbUrl={previewThumbUrl}
            assetUrl={previewFileUrl}
            bind:zoomIn
            bind:zoomOut
            bind:zoomReset
            bind:onwheel={zoomWheel}
            bind:rotateRight
          />
        {/if}

        {#if viewportStore.isXl && visibleVariantTools.length > 0}
          <VariantToolSidebar
            {asset}
            activeTool={selectedTool}
            {creating}
            onSelect={(tool) => {
              selectedTool = tool
              createError = ''
              createSuccess = ''
              showDraftOverlay = false
            }}
          />
        {/if}

        {#if viewportStore.isXl && selectedTool !== null}
          <VariantToolPanel
            tool={selectedTool}
            {asset}
            creating={creating || pendingVariantAssetId === asset.id}
            {handleCreate}
            onClose={() => {
              selectedTool = null
              showDraftOverlay = false
            }}
            onDraftStarted={handleDraftStarted}
            sessionActive={showDraftOverlay}
          />
        {/if}

        {#if !showDraftOverlay}
          <div
            class="pointer-events-auto absolute bottom-8 left-1/2 z-30 -translate-x-1/2"
          >
            <PreviewToolbar
              {zoomIn}
              {zoomOut}
              {rotateRight}
              fullscreenTarget={previewContainer}
              bind:show={showToolbar}
            />
          </div>
        {/if}
      </div>
    </div>
  </Backdrop>

  <!-- Panel: fixed 25% -->
  <div
    transition:fly={{
      x: viewportStore.isMobile ? 0 : 420,
      y: viewportStore.isMobile ? 600 : 0,
      duration: 380,
      easing: cubicOut,
    }}
    class="asset-lightbox fixed inset-y-0 right-0 z-50 flex w-full flex-col bg-white shadow-2xl md:w-[25%] dark:bg-gray-900"
    role="dialog"
    aria-modal="true"
    aria-label={asset.original_filename}
  >
    <!-- Header -->
    <div
      class="damask-texture damask-texture-strong relative flex h-24 flex-shrink-0 items-center justify-center px-4 {ASSET_BACKGROUND_COLORS[
        category
      ]}"
    >
      {#if renamingAsset}
        <div class="flex w-full items-center gap-1 pr-10">
          <InlineEditForm
            value={stemOf(asset.original_filename)}
            busy={renameBusy}
            onsubmit={submitRename}
            oncancel={() => (renamingAsset = false)}
            size="md"
            autofocus
          />
          <span class="flex-shrink-0 text-sm text-black/50 dark:text-white/40"
            >{extOf(asset.original_filename)}</span
          >
        </div>
      {:else}
        <button
          type="button"
          class="damask-asset-name w-full cursor-pointer pr-10 text-left text-base leading-snug font-semibold break-words text-[var(--text-primary)] hover:underline"
          title="Click to rename"
          onclick={() => (renamingAsset = true)}
        >
          {asset.original_filename}
        </button>
      {/if}
      <!-- Top-right controls -->
      <div class="absolute top-3 right-3 flex items-center gap-1.5">
        <button
          type="button"
          class="flex h-8 w-8 items-center justify-center rounded-lg text-black/50 transition-colors hover:bg-black/10 hover:text-black/80 dark:text-white/50 dark:hover:bg-black/20 dark:hover:text-white/90"
          onclick={onclose}
          aria-label="Close panel"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
            ><line x1="18" y1="6" x2="6" y2="18" /><line
              x1="6"
              y1="6"
              x2="18"
              y2="18"
            /></svg
          >
        </button>
      </div>
    </div>

    <div
      class="flex-shrink-0 border-b border-gray-100 px-5 py-3 dark:border-gray-800"
    >
      <AssetMetadataPills {asset} {category} />
    </div>

    <!-- Mobile-only inline preview -->
    <div
      class="relative z-0 block flex-shrink-0 overflow-hidden md:hidden {ASSET_BACKGROUND_COLORS[
        category
      ]}"
      style="height: 220px"
    >
      <div class="flex h-full items-center justify-center">
        <SharedAsset
          asset={{ ...asset, mime_type: previewMimeType }}
          category={previewCategory}
          thumbUrl={previewThumbUrl}
          assetUrl={previewFileUrl}
        />
      </div>
    </div>

    <!-- Animated tab bar -->
    <div
      class="relative flex-shrink-0 border-b border-gray-100 dark:border-gray-800"
    >
      <div
        class="flex overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
      >
        {#each Object.keys(panelTabs) as tab}
          {@const tabInfo = panelTabs[tab as PanelTab]}
          {#if tab !== 'tags' || isMediaTagsAsset}
            <button
              type="button"
              class="relative min-w-fit flex-1 px-2 py-2.5 text-xs font-medium whitespace-nowrap transition-colors {activeTab ===
              tab
                ? 'text-indigo-600 dark:text-indigo-400'
                : 'text-[var(--text-muted)] hover:text-[var(--text-secondary)]'}"
              onclick={() => {
                activeTab = tab as PanelTab
                createError = ''
                createSuccess = ''
              }}
            >
              {#if tabInfo.label}
                {tabInfo.label.charAt(0).toUpperCase() + tabInfo.label.slice(1)}
              {/if}
              {#if tab === 'history' && (asset?.version_count ?? 0) > 1}
                <span
                  class="ml-1 rounded-full bg-indigo-100 px-1.5 py-0.5 text-xs font-medium text-indigo-600 dark:bg-indigo-900/50 dark:text-indigo-400"
                >
                  {asset.version_count}
                </span>
              {/if}
              {#if activeTab === tab}
                <span
                  class="absolute right-0 bottom-0 left-0 h-0.5 rounded-t bg-indigo-600 dark:bg-indigo-400"
                ></span>
              {/if}
            </button>
          {/if}
        {/each}
      </div>
    </div>

    <!-- Tab content (scrollable) -->
    <div class="flex-1 overflow-y-auto">
      <!-- ═══ DETAILS TAB ═══ -->
      {#if activeTab === 'details'}
        <div class="space-y-6 px-5 py-5">
          <AssetMetadata
            asset={assetDetail ?? asset}
            onOpenAsset={handleDerivedAssetOpen}
          />
          <AssetTags {asset} />
          <AssetCollections {asset} />

          {#if authStore.role !== 'viewer' || activeProject}
            <AssetProject {asset} {activeProject} />
          {/if}

          <AssetCustomFields {asset} />
        </div>
      {:else if activeTab === 'tags'}
        <MediaTagsTab assetId={asset.id} />

        <!-- ═══ VARIANTS TAB ═══ -->
      {:else if activeTab === 'variants'}
        <div class="flex flex-col">
          {#if !viewportStore.isXl}
            <Pills
              pills={variantSubTabs}
              active={activeVariantTab}
              set={(p) => {
                activeVariantTab = p.id as VariantTab
                createError = ''
                createSuccess = ''
              }}
            />
          {/if}
          <Feedback error={createError} success={createSuccess} />

          <div class="px-5 py-4">
            <!-- All variants grid -->
            {#if (viewportStore.isXl || activeVariantTab === 'all') && variantPanelState.mode === 'promote'}
              <VariantPromoteForm
                assetId={asset.id}
                assetFilename={asset.original_filename}
                variant={variantPanelState.variant}
                onCancel={() => {
                  variantPanelState = { mode: 'list' }
                }}
                onSuccess={async () => {
                  variantPanelState = { mode: 'list' }
                  await loadVariants()
                }}
              />
            {:else if viewportStore.isXl || activeVariantTab === 'all'}
              {#if coveringWorkflow}
                <CoveringWorkflowBanner workflow={coveringWorkflow} />
              {/if}
              <VariantSharingHeader
                assetId={asset.id}
                {variants}
                onUpdate={replaceVariants}
              />
              {#if canCreateVariantAutomation}
                <button
                  type="button"
                  class="mb-4 flex w-full items-center justify-center gap-2 rounded-lg border border-emerald-200 px-4 py-2 text-sm font-medium text-emerald-700 transition-colors hover:bg-emerald-50 dark:border-emerald-900/60 dark:text-emerald-300 dark:hover:bg-emerald-950/30"
                  onclick={() => {
                    showAutomationModal = true
                  }}
                >
                  <Zap class="h-4 w-4" />
                  {m.variant_automation_button()}
                </button>
              {/if}
              {#if variantRefreshProgress > 0}
                <div class="mb-4 space-y-2">
                  <p
                    class="text-xs font-medium tracking-[0.08em] text-[var(--text-muted)] uppercase"
                  >
                    Refreshing variants in 10 seconds
                  </p>
                  <ProgressBar value={variantRefreshProgress} />
                </div>
              {/if}
              {#if variantsLoading}
                <div class="flex justify-center py-12">
                  <Spinner size="md" />
                </div>
              {:else if variants.length === 0}
                <div class="flex flex-col items-center gap-4 py-10 text-center">
                  <div
                    class="flex h-14 w-14 items-center justify-center rounded-2xl bg-indigo-50 text-indigo-400 dark:bg-indigo-900/25 dark:text-indigo-400"
                  >
                    <Inbox class="h-7 w-7" />
                  </div>
                  <div>
                    <p class="text-md font-medium text-[var(--text-primary)]">
                      {m.no_variants_yet()}
                    </p>
                    {#if authStore.role !== 'viewer' && (isImage || isVideo)}
                      <p class="mt-1 text-sm text-[var(--text-muted)]">
                        {m.variant_use_tabs()}
                      </p>
                    {/if}
                  </div>
                </div>
              {:else}
                <AssetVariantsGrid
                  {asset}
                  {variants}
                  {selectedVariant}
                  onSelectVariant={(v) => {
                    selectedVariant = selectedVariant?.id === v.id ? null : v
                  }}
                  onVariantUpdated={replaceVariant}
                  onVariantsUpdated={replaceVariants}
                  deleteVariant={handleDeleteVariant}
                  promoteVariant={openPromoteVariant}
                  thumbnailUpdated={handleThumbnailUpdated}
                  rerunVariant={handleRerunRefresh}
                />
              {/if}
            {:else}
              <VariantsTool
                {asset}
                creating={creating || pendingVariantAssetId === asset.id}
                tool={activeVariantTab}
                {handleCreate}
                onDone={async () => {
                  activeVariantTab = 'all'
                  await loadVariants()
                }}
              />
            {/if}
          </div>
        </div>
      {:else if activeTab === 'text'}
        <TextTrackPanel {asset} />

        <!-- ═══ COMMENTS TAB ═══ -->
      {:else if activeTab === 'comments'}
        <AssetComments {asset} />

        <!-- ═══ HISTORY TAB ═══ -->
      {:else if activeTab === 'history'}
        <div class="flex flex-col">
          {#if authStore.role !== 'viewer'}
            <div
              class="border-b border-gray-100 px-5 py-3 dark:border-gray-800"
            >
              <button
                type="button"
                class="text-md flex w-full items-center justify-center gap-2 rounded-xl border border-indigo-200 px-4 py-2 font-medium text-indigo-600 transition-colors hover:bg-indigo-50 dark:border-indigo-700 dark:text-indigo-400 dark:hover:bg-indigo-900/20"
                onclick={() => {
                  showUploadVersionModal = true
                }}
              >
                <Upload class="h-4 w-4" />
                {m.upload_new_version()}
              </button>
            </div>
          {/if}
          <VersionHistory
            {asset}
            onversionchanged={(updated) => {
              asset = updated
              onassetupdated?.(updated)
            }}
          />
        </div>

        <!-- ═══ ACTIVITY TAB ═══ -->
      {:else if activeTab === 'activity'}
        <AssetActivity {asset} />

        <!-- ═══ ACTIONS TAB ═══ -->
      {:else if activeTab === 'actions'}
        <div class="space-y-6 px-5 py-5">
          <!-- Quick Actions -->
          <div>
            <SubSectionTitle>{m.quick_actions()}</SubSectionTitle>
            <div class="space-y-2">
              <button
                class="text-md flex w-full items-center gap-3 rounded-xl border border-gray-200 px-4 py-3 text-gray-700 transition-colors hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800"
                onclick={() => {
                  showShareModal = true
                }}
              >
                <Share
                  class="h-4 w-4 shrink-0 text-gray-400 dark:text-gray-500"
                />
                {m.share()}
              </button>
              {#if authStore.role !== 'viewer'}
                <button
                  class="text-md flex w-full items-center gap-3 rounded-xl border border-gray-200 px-4 py-3 text-gray-700 transition-colors hover:bg-gray-50 disabled:opacity-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800"
                  onclick={handleRegenerateThumbnail}
                  disabled={regenThumbLoading}
                >
                  <RefreshCw
                    class="h-4 w-4 shrink-0 text-gray-400 dark:text-gray-500"
                  />
                  {regenThumbLoading ? 'Queuing…' : 'Regenerate Thumbnail'}
                </button>
              {/if}
            </div>
          </div>

          <AssetExportImage {asset} />

          {#if authStore.role !== 'viewer'}
            <AssetDeleteButton {asset} {ondeleted} />
          {/if}
        </div>
      {/if}
    </div>
  </div>
{/if}

{#if asset && showAutomationModal}
  <CreateVariantAutomationModal
    assetId={asset.id}
    assetProjectId={asset.project_id}
    assetFolderId={asset.folder_id}
    assetVariants={automatableVariants}
    onClose={() => {
      showAutomationModal = false
    }}
  />
{/if}

{#if showUploadVersionModal && asset}
  <UploadVersionModal
    {asset}
    onclose={() => {
      showUploadVersionModal = false
    }}
    onuploaded={(updated) => {
      asset = updated
      onassetupdated?.(updated)
      showUploadVersionModal = false
      activeTab = 'history'
    }}
  />
{/if}

{#if showShareModal && asset}
  <ShareModal
    bind:open={showShareModal}
    targets={shareTargets}
    onclose={() => {
      showShareModal = false
    }}
  />
{/if}
