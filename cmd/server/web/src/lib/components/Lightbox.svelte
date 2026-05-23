<script lang="ts">
  import {
    assetApi,
    variantApi,
    mimeCategory,
    type Asset,
    type Variant,
  } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { assetsStore } from '$lib/stores/assets.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import AssetMetadata from './AssetMetadata.svelte'
  import AssetTags from './AssetTags.svelte'
  import AssetCollections from './AssetCollections.svelte'
  import AssetProject from './AssetProject.svelte'
  import AssetCustomFields from './AssetCustomFields.svelte'
  import AssetMetadataPills from './AssetMetadataPills.svelte'
  import SharedAsset from './SharedAsset.svelte'
  import MediaTagsTab from './MediaTagsTab.svelte'
  import TextTrackPanel from './text-tracks/TextTrackPanel.svelte'
  import AssetComments from './AssetComments.svelte'
  import AssetActivity from './AssetActivity.svelte'
  import LightboxPreviewPane from './LightboxPreviewPane.svelte'
  import LightboxPanelHeader from './LightboxPanelHeader.svelte'
  import LightboxTabBar, { type PanelTab } from './LightboxTabBar.svelte'
  import LightboxVariantsTab from './LightboxVariantsTab.svelte'
  import LightboxHistoryTab from './LightboxHistoryTab.svelte'
  import LightboxActionsTab from './LightboxActionsTab.svelte'
  import { type VariantTab } from './variants/VariantsTool.svelte'
  import { deriveVariantMime } from '$lib/utils/variantMime'
  import { ASSET_BACKGROUND_COLORS } from '$lib/stores/shared'
  import { useShortcuts } from '$lib/shortcuts'
  import {
    isAudio as mimeIsAudio,
    isVideo as mimeIsVideo,
  } from '$lib/utils/mime'
  import { viewportStore } from '$lib/stores/viewport.svelte'
  import { fly } from 'svelte/transition'
  import { cubicOut } from 'svelte/easing'
  import { tick } from 'svelte'

  /** /
  Lightbox (orchestrator, ~160 lines)
  ├── LightboxPreviewPane    ← selectedTool, showDraftOverlay, preview urls; bind:zoomIn/Out/Reset
  │   ├── SharedAsset / VariantDraftSession
  │   ├── VariantToolSidebar → onToolSelect → Lightbox relay → LightboxVariantsTab
  │   ├── VariantToolPanel   → oncreate → Lightbox relay → LightboxVariantsTab.handleCreate
  │   └── PreviewToolbar
  └── Panel div
      ├── LightboxPanelHeader   → onAssetRenamed → asset = updated
      ├── AssetMetadataPills
      ├── SharedAsset (mobile)
      ├── LightboxTabBar        → onTabChange → activeTab
      └── Tab content
          ├── [details]  inline (~15 lines)
          ├── [tags]     MediaTagsTab
          ├── [variants] LightboxVariantsTab  bind:selectedVariant/Tool/DraftOverlay/DraftAssetId/oncreate
          ├── [text]     TextTrackPanel
          ├── [comments] AssetComments
          ├── [history]  LightboxHistoryTab   → onVersionChanged → asset = updated
          ├── [activity] AssetActivity
          └── [actions]  LightboxActionsTab   → ondeleted
  /**/

  interface Props {
    asset: Asset | null
    onClose: () => void
    onDeleted: (id: string) => void
    onAssetUpdated?: (updated: Asset) => void
  }

  let {
    asset = $bindable(),
    onClose,
    onDeleted,
    onAssetUpdated,
  }: Props = $props()

  // --- Zoom refs for keyboard shortcuts ---
  let zoomIn = $state<(() => void) | undefined>(undefined)
  let zoomOut = $state<(() => void) | undefined>(undefined)
  let zoomReset = $state<(() => void) | undefined>(undefined)

  useShortcuts({
    'lightbox.zoom-in': () => zoomIn?.(),
    'lightbox.zoom-out': () => zoomOut?.(),
    'view.zoom-reset': () => {
      if (asset) zoomReset?.()
    },
  })

  // --- Panel tab routing ---
  let activeTab = $state<PanelTab>('details')
  const isMediaTagsAsset = $derived(
    asset ? mimeIsAudio(asset.mime_type) || mimeIsVideo(asset.mime_type) : false
  )

  // --- Variant relay state (written by LightboxVariantsTab, read by LightboxPreviewPane) ---
  let selectedVariant = $state<Variant | null>(null)
  let selectedTool = $state<VariantTab | null>(null)
  let showDraftOverlay = $state(false)
  let variantsTabRef = $state<
    ReturnType<typeof LightboxVariantsTab> | undefined
  >(undefined)

  // --- Asset-level derived values ---
  const category = $derived(asset ? mimeCategory(asset.mime_type) : 'document')
  const isImage = $derived(asset?.mime_type?.startsWith('image/') ?? false)
  const isVideo = $derived(asset ? mimeIsVideo(asset.mime_type) : false)
  const isAudio = $derived(asset ? mimeIsAudio(asset.mime_type) : false)

  const previewMimeType = $derived(
    selectedVariant
      ? deriveVariantMime(selectedVariant, asset?.mime_type ?? '')
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

  let assetDetail = $state<Asset | null>(null)

  async function loadAssetDetail() {
    if (!asset) return
    try {
      assetDetail = await assetApi.get(asset.id)
    } catch {
      // silently ignore — fall back to the prop
    }
  }

  const activeProject = $derived(
    asset?.project_id
      ? (projectsStore.projects.find((p) => p.id === asset?.project_id) ?? null)
      : null
  )

  async function onOpenAsset(assetId: string) {
    const nextAsset =
      assetsStore.assets.find((candidate) => candidate.id === assetId) ??
      (await assetApi.get(assetId))
    asset = nextAsset
    onAssetUpdated?.(nextAsset)
  }

  function onDraftStarted(_nonce: string) {
    showDraftOverlay = true
  }

  function onDraftDone() {
    showDraftOverlay = false
    selectedTool = null
    variantsTabRef?.notifyDraftDone()
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

  function onToolSelect(tool: VariantTab | null) {
    selectedTool = tool
    if (tool === null) showDraftOverlay = false
  }

  async function handleCreate(type: string, params: object) {
    activeTab = 'variants'
    await tick()
    await variantsTabRef?.handleCreate(type, params)
  }

  function onAssetRenamed(updated: Asset) {
    asset = updated
    onAssetUpdated?.(updated)
  }

  function onVersionChanged(updated: Asset) {
    asset = updated
    onAssetUpdated?.(updated)
  }

  function onThumbnailUpdated() {
    asset = { ...asset!, thumbnail_key: 'updated' }
    onAssetUpdated?.(asset!)
  }

  $effect(() => {
    if (!asset) {
      selectedVariant = null
      selectedTool = null
      showDraftOverlay = false
      assetDetail = null
      return
    }
    selectedVariant = null
    selectedTool = null
    loadAssetDetail()
  })
</script>

<svelte:window onkeydown={handleWindowKeydown} />

{#if asset}
  <LightboxPreviewPane
    {asset}
    {previewMimeType}
    {previewCategory}
    {previewThumbUrl}
    {previewFileUrl}
    {selectedTool}
    {showDraftOverlay}
    draftAssetId={asset.id}
    creating={false}
    {onDraftStarted}
    {handleCreate}
    {onToolSelect}
    {onDraftDone}
    {onClose}
    bind:zoomIn
    bind:zoomOut
    bind:zoomReset
  />

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
    <LightboxPanelHeader {asset} {category} {onClose} {onAssetRenamed} />

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

    <LightboxTabBar
      {activeTab}
      {asset}
      {isMediaTagsAsset}
      onTabChange={(tab) => (activeTab = tab)}
    />

    <!-- Tab content (scrollable) -->
    <div class="flex-1 overflow-y-auto">
      {#if activeTab === 'details'}
        <div class="space-y-6 px-5 py-5">
          <AssetMetadata asset={assetDetail ?? asset} {onOpenAsset} />
          <AssetTags {asset} />
          <AssetCollections {asset} />
          {#if authStore.role !== 'viewer' || activeProject}
            <AssetProject {asset} {activeProject} />
          {/if}
          <AssetCustomFields {asset} />
        </div>
      {:else if activeTab === 'tags'}
        <MediaTagsTab assetId={asset.id} />
      {:else if activeTab === 'variants'}
        <LightboxVariantsTab
          bind:this={variantsTabRef}
          {asset}
          {isImage}
          {isVideo}
          {isAudio}
          bind:selectedTool
          bind:selectedVariant
          bind:showDraftOverlay
          {onThumbnailUpdated}
          onNavigate={(tab) => (activeTab = tab)}
          {onDraftStarted}
        />
      {:else if activeTab === 'text'}
        <TextTrackPanel {asset} />
      {:else if activeTab === 'comments'}
        <AssetComments {asset} />
      {:else if activeTab === 'history'}
        <LightboxHistoryTab {asset} {onVersionChanged} />
      {:else if activeTab === 'activity'}
        <AssetActivity {asset} />
      {:else if activeTab === 'actions'}
        <LightboxActionsTab {asset} {onDeleted} />
      {/if}
    </div>
  </div>
{/if}
