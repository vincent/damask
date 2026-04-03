<script lang="ts">
  import { onMount } from 'svelte'
  import { type Asset, type MenuItem } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { assetsStore } from '$lib/stores/assets.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import { foldersStore } from '$lib/stores/folders.svelte'
  import { navigationStore } from '$lib/stores/navigation.svelte'
  import { selectionStore } from '$lib/stores/selection.svelte'
  import AssetCard from '$lib/components/AssetCard.svelte'
  import Lightbox from '$lib/components/Lightbox.svelte'
  import BulkActionBar from '$lib/components/BulkActionBar.svelte'
  import CommandPalette from '$lib/components/CommandPalette.svelte'
  import ShareModal from '$lib/components/ShareModal.svelte'
  import { CATEGORY_BORDER, CATEGORY_ICON_BG, CATEGORY_LABELS, CATEGORY_ORDER } from '$lib/stores/shared'
  import { Inbox, Loader, Share2 } from '@lucide/svelte'
  import SearchInput from '$lib/components/ui/SearchInput.svelte'
  import EmptyState from '$lib/components/ui/EmptyState.svelte'
  import Toast from '$lib/components/ui/Toast.svelte'
  import GridSkeleton from '../../lib/components/ui/GridSkeleton.svelte';
  import AssetIcon from '../../lib/components/AssetIcon.svelte'
  import OnboardingScreen from '$lib/components/OnboardingScreen.svelte'
  import { goto } from '$app/navigation'
  import LibraryStatusBar from '$lib/components/LibraryStatusBar.svelte'
  import SortButtons from '$lib/components/SortButtons.svelte'

  let selectedAsset = $state<Asset | null>(null)
  let sentinel = $state<HTMLDivElement | undefined>(undefined)
  let showPalette = $state(false)
  let showProjectShareModal = $state(false)
  let seenSplashScreen = $state(false)
  let zoom = $state(10)
  const maxZoom = 20
  let isDraggingFiles = $state(false)
  let mainEl = $state<HTMLElement | undefined>(undefined)
  let rubberBand = $state<{ startX: number; startY: number; x: number; y: number; w: number; h: number } | null>(null)
  let zoomOverlay = $state<{ src: string; vars: string, asset: Asset } | null>(null)

  let sort = $state<'mimetype' | 'created_at' | 'size'>('created_at')
  let asc = $state(false)

  const activeProject = $derived(
    navigationStore.activeProjectId
      ? projectsStore.projects.find((p) => p.id === navigationStore.activeProjectId) ?? null
      : null,
  )

  const projectShareTargets = $derived(
    activeProject
      ? [{ type: 'project' as const, id: activeProject.id, label: activeProject.name, assetCount: activeProject.asset_count }]
      : [],
  )

  function handleCardClick(asset: Asset, index: number, event: MouseEvent) {
    const handled = selectionStore.handleCardClick(
      asset,
      index,
      assetsStore.assets,
      event,
      authStore.role !== 'viewer',
    )
    if (!handled) {
      const cardEl = (event.currentTarget as HTMLElement).closest('button.asset-card') as HTMLElement | null
      const imgEl = cardEl?.querySelector('img') as HTMLImageElement | null
      const rect = cardEl?.getBoundingClientRect()

      if (rect && imgEl?.src) {
        // The overlay is centered on screen at 70vw × 70vh.
        // We start it visually at the card's position/size and animate it to center.
        // Compute the overlay's final rect so we can derive the correct transform origin.
        const ow = window.innerWidth * 0.70
        const oh = window.innerHeight * 0.70

        // Scale: make the overlay appear as small as the card
        const sx = rect.width / ow
        const sy = rect.height / oh

        // Translate: move overlay center to card center
        // (transform-origin is center center, so we offset from screen center)
        const cardCx = rect.left + rect.width / 2
        const cardCy = rect.top + rect.height / 2
        const tx = cardCx - window.innerWidth / 2
        const ty = cardCy - window.innerHeight / 2
        zoomOverlay = {
          src: imgEl.src,
          vars: `--tx:${tx}px; --ty:${ty}px; --sx:${sx}; --sy:${sy}`,
          asset,
        }
        setTimeout(() => {
          selectedAsset = asset
          zoomOverlay = null
        }, 380)
      } else {
        selectedAsset = asset
      }
    }
  }

  async function handleProjectSelect(item: MenuItem | null) {
    navigationStore.selectProject(item?.id ?? null)
    if (item?.id) await foldersStore.loadForProject(item.id)
    if (item?.url) return goto(item.url)
    await assetsStore.load(true)
  }

  async function handleBulkDone() {
    selectionStore.clear()
    await Promise.all([projectsStore.load(), assetsStore.load(true)])
  }

  function handleDeleted(id: string) {
    assetsStore.remove(id)
    selectionStore.remove(id)
    selectedAsset = null
  }

  function handleMainDragOver(e: DragEvent) {
    if (!e.dataTransfer?.types.includes('Files')) return
    if (authStore.role === 'viewer') return
    e.preventDefault()
    e.dataTransfer.dropEffect = 'copy'
    isDraggingFiles = true
  }

  function handleMainDragLeave(e: DragEvent) {
    if (!(e.currentTarget as HTMLElement).contains(e.relatedTarget as Node)) {
      isDraggingFiles = false
    }
  }

  function handleMainDrop(e: DragEvent) {
    isDraggingFiles = false
    if (!e.dataTransfer?.files.length) return
    if (authStore.role === 'viewer') return
    e.preventDefault()
    assetsStore.upload(Array.from(e.dataTransfer.files))
  }

  function handleMainMouseDown(e: MouseEvent) {
    if (!e.shiftKey || e.button !== 0) return
    if ((e.target as Element).closest('[data-asset-id]')) return
    e.preventDefault()
    rubberBand = { startX: e.clientX, startY: e.clientY, x: e.clientX, y: e.clientY, w: 0, h: 0 }
  }

  function handleWindowMouseMove(e: MouseEvent) {
    if (!rubberBand) return
    const x = Math.min(e.clientX, rubberBand.startX)
    const y = Math.min(e.clientY, rubberBand.startY)
    const w = Math.abs(e.clientX - rubberBand.startX)
    const h = Math.abs(e.clientY - rubberBand.startY)
    rubberBand = { ...rubberBand, x, y, w, h }
  }

  function handleWindowMouseUp() {
    if (!rubberBand || !mainEl) { rubberBand = null; return }
    const ids: string[] = []
    mainEl.querySelectorAll('[data-asset-id]').forEach((el) => {
      const r = el.getBoundingClientRect()
      if (r.right > rubberBand!.x && r.left < rubberBand!.x + rubberBand!.w &&
          r.bottom > rubberBand!.y && r.top < rubberBand!.y + rubberBand!.h) {
        ids.push((el as HTMLElement).dataset.assetId!)
      }
    })
    if (ids.length > 0) selectionStore.selectByIds(ids)
    rubberBand = null
  }

  function handleWindowKeydown(e: KeyboardEvent) {
    if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
      e.preventDefault()
      showPalette = !showPalette
    }
    if (e.key === 'Escape' && !showPalette) selectionStore.clear()
    if (selectedAsset && (e.key === 'ArrowLeft' || e.key === 'ArrowRight')) {
      e.preventDefault()
      const assets = assetsStore.assets
      const idx = assets.findIndex((a) => a.id === selectedAsset!.id)
      if (e.key === 'ArrowLeft' && idx > 0) selectedAsset = assets[idx - 1]
      if (e.key === 'ArrowRight' && idx < assets.length - 1) selectedAsset = assets[idx + 1]
    }
  }

  onMount(() => {
    seenSplashScreen = localStorage.getItem(`onboard_${authStore.workspace?.id}`) !== null
  })

  $effect(() => {
    if (!sentinel) return
    const el = sentinel
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && assetsStore.nextCursor && !assetsStore.loading) {
          assetsStore.load()
        }
      },
      { rootMargin: '200px' },
    )
    observer.observe(el)
    return () => observer.disconnect()
  })
</script>

<svelte:head>
  <title>Library — Damask DAM</title>
</svelte:head>

<svelte:window onkeydown={handleWindowKeydown} onmousemove={handleWindowMouseMove} onmouseup={handleWindowMouseUp} />

<style lang="scss">
  .library-content {
    opacity: 1;
  }
  .zoom-overlay + .library-content {
    opacity: 0.1;
  }
  .zoom-overlay {
    transform-origin: center center;
    will-change: transform, border-radius, opacity;
    animation: card-zoom-in 380ms cubic-bezier(0.32, 0.72, 0.3, 1) forwards;
  }

  @keyframes card-zoom-in {
    from {
      transform: translate(var(--tx), var(--ty)) scale(var(--sx), var(--sy));
      opacity: 1;
    }
    80% {
      opacity: 1;
    }
    to {
      transform: translate(0px, 0px) scale(1, 1);
      opacity: 0;
    }
  }
</style>

<!-- Top bar -->
<header class="flex items-center justify-between border-b border-gray-100 bg-white px-6 py-4 dark:border-gray-800 dark:bg-gray-900">
  <div class="flex items-center gap-3">
    <div>
      <h1 class="text-xl font-bold text-gray-900 dark:text-gray-50">
        {projectsStore.activeProjectName ?? 'Library'}
      </h1>
      <p class="mt-0.5 text-sm text-gray-400">
        All Assets{#if projectsStore.activeProjectName} / {projectsStore.activeProjectName}{/if}
      </p>
    </div>
    {#if activeProject}
      <button
        type="button"
        class="flex items-center gap-1.5 rounded-lg border border-gray-200 px-3 py-1.5 text-sm text-gray-600 transition-colors hover:border-indigo-300 hover:bg-indigo-50 hover:text-indigo-700 dark:border-gray-700 dark:text-gray-400 dark:hover:border-indigo-700 dark:hover:bg-indigo-900/20 dark:hover:text-indigo-400"
        onclick={() => { showProjectShareModal = true }}
        title="Share this project"
      >
        <Share2 class="h-3.5 w-3.5" />
        Share
      </button>
    {/if}
  </div>

  <div class="flex items-center gap-2">
    <SortButtons
      bind:value={sort} bind:asc
      keys={ { created_at: 'by date', mimetype: 'by type', size: 'by size' } }
      sort={(key, asc)  => assetsStore.sort(key, asc)}
    />
  </div>

  <div class="flex items-center gap-2">
    <!-- Search -->
    <SearchInput
      class="w-64"
      value={assetsStore.query}
      placeholder="Search anything..."
      onchange={(v) => { assetsStore.query = v; assetsStore.search() }}
    />

    {#if authStore.role !== 'viewer'}
      <label class="cursor-pointer rounded-lg bg-indigo-600 px-3 py-2 text-sm font-medium text-white hover:bg-indigo-700">
        Upload
        <input
          type="file"
          multiple
          class="hidden"
          onchange={(e) => {
            const files = Array.from((e.target as HTMLInputElement).files ?? [])
            assetsStore.upload(files)
            ;(e.target as HTMLInputElement).value = ''
          }}
        />
      </label>
    {/if}
  </div>
</header>

{#if zoomOverlay}
  <div class="zoom-overlay-bg fixed w-screen grid place-items-center p-40 inset-0 z-40 bg-black/70 backdrop-blur-lg"></div>
  <div class="zoom-overlay fixed w-[75%] grid place-items-center p-40 inset-0 z-42" style={zoomOverlay.vars}>
    <img src={zoomOverlay.src} width={zoomOverlay.asset.width || 640} alt="" class="object-cover min-w-xl max-w-3xl max-h-[80vh]" />
  </div>
{/if}

<!-- Content -->
<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<main
  bind:this={mainEl}
  class="library-content relative flex-1 overflow-y-auto px-6 py-6"
  ondragover={handleMainDragOver}
  ondragleave={handleMainDragLeave}
  ondrop={handleMainDrop}
  onmousedown={handleMainMouseDown}
>
  {#if isDraggingFiles}
    <div class="pointer-events-none absolute inset-0 z-20 flex items-center justify-center bg-indigo-50/80 ring-2 ring-inset ring-indigo-400 dark:bg-indigo-950/80">
      <div class="flex flex-col items-center gap-2 text-indigo-600 dark:text-indigo-400">
        <svg class="h-10 w-10" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5"
            d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
        </svg>
        <p class="text-sm font-medium">Drop to upload</p>
      </div>
    </div>
  {/if}

  {#if assetsStore.initialLoad}
    <GridSkeleton x={7} y={3} {zoom} {maxZoom} />
  {:else if !seenSplashScreen}
    <OnboardingScreen onDismiss={() => {
      seenSplashScreen = true
      localStorage.setItem(`onboard_${authStore.workspace?.id}`, 'true')
    }} />
  {:else if assetsStore.assets.length === 0}
    <EmptyState
      title={assetsStore.query ? `No results for "${assetsStore.query}"` : 'No assets yet'}
      description={assetsStore.query ? 'Try a different search term' : 'Upload files to get started'}
    >
      {#snippet icon()}<Inbox class="h-16 w-16" />{/snippet}
    </EmptyState>
  {:else if sort === 'mimetype'}
    <!-- Grouped by category -->
    {#each CATEGORY_ORDER as cat}
      {@const group = assetsStore.assetsByCategory[cat]}
      {#if group.length > 0}
        <div class="mb-10">
          <!-- Category header -->
          <div class="sticky top-[-25px] z-10 bg-gray-50 dark:bg-gray-950 py-2 flex items-center gap-3">
            <AssetIcon category={cat} class="h-8 w-8 items-center justify-center rounded-lg {CATEGORY_ICON_BG[cat].light} {CATEGORY_ICON_BG[cat].dark}"/>
            <h2 class="text-sm font-semibold text-gray-900 dark:text-gray-50">{CATEGORY_LABELS[cat]}</h2>
            <span class="text-sm text-gray-400 dark:text-gray-500">{group.length}</span>
          </div>

          <!-- Cards with left accent border -->
          <div class="border-l-2 {CATEGORY_BORDER[cat]} pl-4">
            <div class="grid gap-3 grid-cols-{1 + maxZoom - Math.floor(zoom)}">
              {#each group as asset (asset.id)}
                {@const globalIndex = assetsStore.assets.indexOf(asset)}
                <div class="relative" data-asset-id={asset.id}>
                  {#if selectionStore.selectedIds.has(asset.id)}
                    <div class="pointer-events-none absolute inset-0 z-5 rounded-xl ring-2 ring-indigo-500">
                      <div class="absolute right-1.5 top-1.5 flex h-5 w-5 items-center justify-center rounded-full bg-indigo-600">
                        <svg class="h-3 w-3 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7" />
                        </svg>
                      </div>
                    </div>
                  {/if}
                  <AssetCard {asset} {zoom} onclick={(e) => handleCardClick(asset, globalIndex, e)} />
                </div>
              {/each}
            </div>
          </div>
        </div>
      {/if}
    {/each}
    <!-- Infinite scroll sentinel -->
    <div bind:this={sentinel} class="flex justify-center py-6">
      {#if assetsStore.loading && assetsStore.nextCursor}
        <Loader class="h-6 w-6 animate-spin text-gray-400" />
      {/if}
    </div>
  {:else}
    <div class="mb-10">
      <div class="grid gap-3 grid-cols-{1 + maxZoom - Math.floor(zoom)}">
        {#each assetsStore.assets as asset, globalIndex (asset.id)}
          <AssetCard {asset} {zoom} onclick={(e) => handleCardClick(asset, globalIndex, e)} />
        {/each}
      </div>
    </div>
    <!-- Infinite scroll sentinel -->
    <div bind:this={sentinel} class="flex justify-center py-6">
      {#if assetsStore.loading && assetsStore.nextCursor}
        <Loader class="h-6 w-6 animate-spin text-gray-400" />
      {/if}
    </div>
  {/if}
</main>

<LibraryStatusBar bind:zoom={zoom} max={maxZoom - 1} />

{#if rubberBand && rubberBand.w > 2 && rubberBand.h > 2}
  <div
    class="pointer-events-none fixed z-30 rounded border border-indigo-500 bg-indigo-500/15"
    style="left:{rubberBand.x}px; top:{rubberBand.y}px; width:{rubberBand.w}px; height:{rubberBand.h}px"
  ></div>
{/if}

<div class="hidden
  grid-cols-1  grid-cols-2  grid-cols-3  grid-cols-4  grid-cols-5
  grid-cols-6  grid-cols-7  grid-cols-8  grid-cols-9  grid-cols-10
  grid-cols-11 grid-cols-12 grid-cols-13 grid-cols-14 grid-cols-15
  grid-cols-16 grid-cols-17 grid-cols-18 grid-cols-19 grid-cols-20 grid-cols-21
"></div>

<Toast />

<Lightbox
  asset={selectedAsset}
  onclose={() => (selectedAsset = null)}
  ondeleted={handleDeleted}
  ontagschanged={() => assetsStore.load(true)}
  onprojectchanged={() => { projectsStore.load(); assetsStore.load(true) }}
/>

<BulkActionBar
  selectedIds={selectionStore.selectedIds}
  projects={projectsStore.projects}
  ondone={handleBulkDone}
  onclear={() => selectionStore.clear()}
/>

{#if showPalette}
  <CommandPalette
    projects={projectsStore.projects}
    onselect={(item) => { handleProjectSelect(item); showPalette = false }}
    onclose={() => { showPalette = false }}
  />
{/if}

{#if showProjectShareModal && projectShareTargets.length > 0}
  <ShareModal
    bind:open={showProjectShareModal}
    targets={projectShareTargets}
    onclose={() => { showProjectShareModal = false }}
  />
{/if}
