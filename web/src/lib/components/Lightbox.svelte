<script lang="ts">
  import {
    assetApi,
    variantApi,
    mimeCategory,
    type Asset,
    type Variant,
  } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import ShareModal from './ShareModal.svelte'
  import { Copy, Inbox, Link, Share } from '@lucide/svelte'
  import AssetTags from './AssetTags.svelte';
  import AssetMetadata from './AssetMetadata.svelte';
  import VariantCreateResize from './VariantCreateResize.svelte';
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import SharedAsset from './SharedAsset.svelte'
  import Close from './ui/Close.svelte'
  import VariantCreateWatermark from './VariantCreateWatermark.svelte'
  import VariantCreateConvert from './VariantCreateConvert.svelte'
  import VariantCreateCrop from './VariantCreateCrop.svelte'
  import VariantCreateRemoveBackground from './VariantCreateRemoveBackground.svelte'
  import VariantCreateVideoThumbnail from './VariantCreateVideoThumbnail.svelte'
  import VariantCreateVideoTranscode from './VariantCreateVideoTranscode.svelte'
  import AssetProject from './AssetProject.svelte'
  import Pills from './ui/Pills.svelte'
  import Feedback from './ui/Feedback.svelte'
  import AssetVariantsGrid from './AssetVariantsGrid.svelte'
  import AssetMetadataPills from './AssetMetadataPills.svelte'
  import AssetExportImage from './AssetExportImage.svelte'
  import AssetDeleteButton from './AssetDeleteButton.svelte'

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
  type VariantTab = 'all' | 'resize' | 'watermark' | 'convert' | 'crop' | 'bg_remove' | 'video_transcode' | 'video_thumbnail'
  let activeVariantTab = $state<VariantTab>('all')

  // --- Asset state ---
  let showShareModal = $state(false)
  let tags = $state<string[]>([])
  let linkCopied = $state(false)

  // --- Variant state ---
  let variants = $state<Variant[]>([])
  let variantsLoading = $state(false)
  let creating = $state(false)
  let createError = $state('')
  let createSuccess = $state('')

  const category = $derived(asset ? mimeCategory(asset.mime_type) : 'document')

  const previewBg: Record<string, string> = {
    image: 'bg-sky-300 dark:bg-sky-700',
    video: 'bg-red-300 dark:bg-red-700',
    audio: 'bg-emerald-300 dark:bg-emerald-700',
    document: 'bg-blue-200 dark:bg-blue-700',
  }

  const isImage = $derived(asset?.mime_type?.startsWith('image/') ?? false)
  const isVideo = $derived(asset?.mime_type?.startsWith('video/') ?? false)

  $effect(() => {
    if (!asset) {
      tags = []
      variants = []
      return
    }
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
      { id: 'video_transcode' as VariantTab, label: 'Transcode' },
      { id: 'video_thumbnail' as VariantTab, label: 'Thumbnail' },
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
    <div class="damask-texture damask-texture-strong relative h-20 flex-shrink-0 flex items-center justify-center {previewBg[category]}">
      <h2 class="max-w-[500px] break-all text-lg font-bold leading-tight text-black dark:text-gray-50" title={asset.original_filename}>
        {asset.original_filename}
      </h2>
      <!-- Top-right controls -->
      <div class="absolute right-3 top-3 flex items-center gap-1.5">
        <Close close={onclose} />
      </div>
    </div>

    <div class="flex-shrink-0 border-b border-gray-100 px-5 py-3 dark:border-gray-800">
      <AssetMetadataPills {asset} {category} {previewBg} />
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
          <AssetMetadata {asset} />
          <AssetTags {asset} {ontagschanged} />

          {#if authStore.role !== 'viewer' || activeProject}
            <AssetProject {asset} {activeProject} {onprojectchanged} />
          {/if}
        </div>

      <!-- ═══ VARIANTS TAB ═══ -->
      {:else if activeTab === 'variants'}
        <div class="flex flex-col">
          <Pills pills={variantSubTabs} active={activeVariantTab} set={(p) => { activeVariantTab = p.id as VariantTab; createError = ''; createSuccess = '' }} />
          <Feedback error={createError} success={createSuccess} />

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
                <AssetVariantsGrid {asset} {variants} deleteVariant={handleDeleteVariant} />
              {/if}

            {:else if activeVariantTab === 'resize'}
              <VariantCreateResize {asset} {creating} {handleCreate} />
            {:else if activeVariantTab === 'watermark'}
              <VariantCreateWatermark {asset} {creating} {handleCreate} />
            {:else if activeVariantTab === 'convert'}
              <VariantCreateConvert {asset} {creating} {handleCreate} />
            {:else if activeVariantTab === 'crop'}
              <VariantCreateCrop {asset} {creating} {handleCreate} />
            {:else if activeVariantTab === 'bg_remove'}
              <VariantCreateRemoveBackground {asset} {creating} {handleCreate} />
            {:else if activeVariantTab === 'video_transcode'}
            <VariantCreateVideoTranscode {asset} {creating} {handleCreate} />
            {:else if activeVariantTab === 'video_thumbnail'}
              <VariantCreateVideoThumbnail {asset} {creating} {handleCreate} />
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

          <AssetExportImage {asset} />

          {#if authStore.role !== 'viewer'}
            <AssetDeleteButton {asset} {ondeleted} />
          {/if}

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
