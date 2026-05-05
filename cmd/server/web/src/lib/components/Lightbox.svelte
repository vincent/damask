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
  import ShareModal from './ShareModal.svelte'
  import PreviewToolbar from './ui/PreviewToolbar.svelte'
  import { Inbox, RefreshCw, Share, Upload } from '@lucide/svelte'
  import AssetTags from './AssetTags.svelte'
  import AssetMetadata from './AssetMetadata.svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import SharedAsset from './SharedAsset.svelte'
  import Close from './ui/Close.svelte'
  import VariantsTool, { type VariantTab } from './variants/VariantsTool.svelte'
  import AssetProject from './AssetProject.svelte'
  import Pills from './ui/Pills.svelte'
  import Feedback from './ui/Feedback.svelte'
  import AssetVariantsGrid from './AssetVariantsGrid.svelte'
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
  import Backdrop from './ui/Backdrop.svelte'
  import ButtonCopy from './ui/ButtonCopy.svelte'
  import { ASSET_BACKGROUND_COLORS } from '$lib/stores/shared'
  import { m } from '$lib/paraglide/messages'
  import { useShortcuts } from '$lib/shortcuts'
  import {
    isAudio as mimeIsAudio,
    isVideo as mimeIsVideo,
  } from '$lib/utils/mime'

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
    variants: { label: m.tab_variants(), icon: null },
    comments: { label: m.tab_comments(), icon: null },
    history: { label: m.tab_history(), icon: null },
    activity: { label: m.tab_activity(), icon: null },
    actions: { label: m.tab_actions(), icon: null },
  }
  type PanelTab = keyof typeof panelTabs
  let activeTab = $state<PanelTab>('details')

  let activeVariantTab = $state<VariantTab>('all')

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

  // --- Variant state ---
  let variants = $state<Variant[]>([])
  let variantsLoading = $state(false)
  let creating = $state(false)
  let createError = $state('')
  let createSuccess = $state('')

  const category = $derived(asset ? mimeCategory(asset.mime_type) : 'document')
  const isImage = $derived(asset?.mime_type?.startsWith('image/') ?? false)
  const isVideo = $derived(asset ? mimeIsVideo(asset.mime_type) : false)
  const isAudio = $derived(asset ? mimeIsAudio(asset.mime_type) : false)

  $effect(() => {
    if (!asset) {
      variants = []
      return
    }
    loadVariants()
  })

  async function loadVariants() {
    if (!asset) return
    variantsLoading = true
    try {
      variants = (await variantApi.list(asset.id))?.variants ?? []
    } catch {
      // silently ignore
    } finally {
      variantsLoading = false
    }
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
      createSuccess = `Queued (job ${result.job_id.slice(0, 8)}). Refreshing shortly…`
      setTimeout(() => {
        activeVariantTab = 'all'
        loadVariants()
        createSuccess = ''
      }, 3000)
    } catch (e: unknown) {
      createError = e instanceof Error ? e.message : m.variant_create_failed()
    } finally {
      creating = false
    }
  }

  function handleClose(e: MouseEvent) {
    const src = e.target as HTMLElement
    if (src.classList.contains('asset-preview-full')) return
    if (src.classList.contains('asset-preview-toolbar')) return
    onclose?.()
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
        ]
      : []),
    ...(isVideo
      ? [
          { id: 'video_transcode' as VariantTab, label: m.transcode() },
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

{#if asset}
  <Backdrop class="asset-lightbox-bg w-screen" {onclose}>
    <div
      bind:this={previewContainer}
      class="asset-preview-container fixed inset-0 grid w-[75%] place-items-center p-40"
      role="button"
      tabindex="-1"
      onclick={handleClose}
      onkeydown={(e) => e.key === 'Enter' && onclose()}
      onwheel={(e) => zoomWheel?.(e)}
      onmousemove={() => showToolbar?.()}
      aria-label={m.close()}
    >
      <SharedAsset
        {asset}
        {category}
        thumbUrl={assetApi.thumbUrl(asset.id)}
        assetUrl={assetApi.fileUrl(asset.id)}
        bind:zoomIn
        bind:zoomOut
        bind:zoomReset
        bind:onwheel={zoomWheel}
        bind:rotateRight
      />
      <div
        class="pointer-events-auto absolute bottom-8 left-1/2 z-20 -translate-x-1/2"
      >
        <PreviewToolbar
          {zoomIn}
          {zoomOut}
          {rotateRight}
          fullscreenTarget={previewContainer}
          bind:show={showToolbar}
        />
      </div>
    </div>
  </Backdrop>

  <!-- Panel: fixed 25% -->
  <div
    transition:fly={{ x: 420, duration: 380, easing: cubicOut }}
    class="asset-lightbox fixed inset-y-0 right-0 z-50 flex w-3xl w-[25%] flex-col bg-white shadow-2xl dark:bg-gray-900"
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

    <!-- Animated tab bar -->
    <div
      class="relative flex-shrink-0 border-b border-gray-100 dark:border-gray-800"
    >
      <div
        class="flex overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
      >
        {#each Object.keys(panelTabs) as tab}
          {@const tabInfo = panelTabs[tab as PanelTab]}
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
        {/each}
      </div>
    </div>

    <!-- Tab content (scrollable) -->
    <div class="flex-1 overflow-y-auto">
      <!-- ═══ DETAILS TAB ═══ -->
      {#if activeTab === 'details'}
        <div class="space-y-6 px-5 py-5">
          <AssetMetadata {asset} />
          <AssetTags {asset} />
          <AssetCollections {asset} />

          {#if authStore.role !== 'viewer' || activeProject}
            <AssetProject {asset} {activeProject} />
          {/if}

          <AssetCustomFields {asset} />
        </div>

        <!-- ═══ VARIANTS TAB ═══ -->
      {:else if activeTab === 'variants'}
        <div class="flex flex-col">
          <Pills
            pills={variantSubTabs}
            active={activeVariantTab}
            set={(p) => {
              activeVariantTab = p.id as VariantTab
              createError = ''
              createSuccess = ''
            }}
          />
          <Feedback error={createError} success={createSuccess} />

          <div class="px-5 py-4">
            <!-- All variants grid -->
            {#if activeVariantTab === 'all'}
              {#if variantsLoading}
                <div class="flex justify-center py-12">
                  <Spinner size="md" />
                </div>
              {:else if variants.length === 0}
                <div
                  class="flex flex-col items-center gap-3 py-12 text-center text-gray-400"
                >
                  <Inbox class="h-10 w-10" />
                  <p class="text-md">{m.no_variants_yet()}</p>
                  {#if authStore.role !== 'viewer' && (isImage || isVideo)}
                    <p class="text-sm">{m.variant_use_tabs()}</p>
                  {/if}
                </div>
              {:else}
                <AssetVariantsGrid
                  {asset}
                  {variants}
                  deleteVariant={handleDeleteVariant}
                />
              {/if}
            {:else}
              <VariantsTool
                {asset}
                {creating}
                tool={activeVariantTab}
                {handleCreate}
              />
            {/if}
          </div>
        </div>

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
