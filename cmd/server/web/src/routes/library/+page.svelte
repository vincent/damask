<script lang="ts">
  import { onMount } from 'svelte'
  import { assetApi, type Asset, type MenuItem } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { assetsStore } from '$lib/stores/assets.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import { foldersStore } from '$lib/stores/folders.svelte'
  import { navigationStore } from '$lib/stores/navigation.svelte'
  import { selectionStore } from '$lib/stores/selection.svelte'
  import { createRubberBand } from '$lib/stores/rubberBand.svelte'
  import LibraryHeader from '$lib/components/LibraryHeader.svelte'
  import AssetGrid from '$lib/components/AssetGrid.svelte'
  import ZoomOverlay from '$lib/components/ui/ZoomOverlay.svelte'
  import Lightbox from '$lib/components/Lightbox.svelte'
  import AssetBulkActionBar from '$lib/components/AssetBulkActionBar.svelte'
  import CommandPalette from '$lib/components/CommandPalette.svelte'
  import ShareModal from '$lib/components/ShareModal.svelte'
  import LibraryStatusBar from '$lib/components/LibraryStatusBar.svelte'
  import ProjectInfoPanel from '$lib/components/ProjectInfoPanel.svelte'
  import TagFilterBar from '$lib/components/TagFilterBar.svelte'
  import CustomFieldFilters from '$lib/components/CustomFieldFilters.svelte'
  import UploadsTray from '$lib/components/UploadsTray.svelte'
  import { goto } from '$app/navigation'
  import { Box } from '@lucide/svelte'

  let selectedAsset = $state<Asset | null>(null)
  let showPalette = $state(false)
  let showProjectShareModal = $state(false)
  let seenSplashScreen = $state(false)
  let zoom = $state(10)
  const maxZoom = 20
  let isDraggingFiles = $state(false)
  let mainEl = $state<HTMLElement | undefined>(undefined)
  let zoomOverlay = $state<{ src: string; vars: string; asset: Asset } | null>(null)
  let draggingProjectCover = $state(false)

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

  const rb = createRubberBand(() => mainEl)

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
        const ow = window.innerWidth * 0.70
        const oh = window.innerHeight * 0.70
        const sx = rect.width / ow
        const sy = rect.height / oh
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
    assetsStore.upload(Array.from(e.dataTransfer.files), activeProject?.id ?? null, navigationStore.activeFolderId ?? null)
  }

  function handleWindowKeydown(e: KeyboardEvent) {
    if (e.target instanceof HTMLInputElement 
      || e.target instanceof HTMLTextAreaElement) return;

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

  function onDropProjectCover(e: DragEvent) {
    console.log('drop')
    e.preventDefault()
    draggingProjectCover = false
    const cover_asset_id = e.dataTransfer?.getData('text/plain')
    if (authStore.role === 'viewer') return;
    if (!activeProject) return;
    if (!cover_asset_id) return;
    projectsStore.update(activeProject.id, { cover_asset_id })
  }
  function onDraggingProjectCover(e: DragEvent) {
    draggingProjectCover = true
    e.preventDefault()
  }
  function onDragLeaveProjectCover(e: DragEvent) {
    draggingProjectCover = false
    e.preventDefault()
  }

  onMount(() => {
    seenSplashScreen = localStorage.getItem(`onboard_${authStore.workspace?.id}`) !== null
  })
</script>

<svelte:head>
  <title>Library — Damask</title>
</svelte:head>

<svelte:window
  onkeydown={handleWindowKeydown}
  onmousemove={(e) => rb.onMouseMove(e)}
  onmouseup={() => rb.onMouseUp((ids) => selectionStore.selectByIds(ids))}
/>

<LibraryHeader
  bind:sort
  bind:asc
  showShareButton={!!activeProject}
  onShareProject={() => { showProjectShareModal = true }}
>
  {#snippet prefix()}
    <div role="img" class={`rounded p-2 border-1 transition-colors ${activeProject ? '' : 'hidden'} ${draggingProjectCover ? 'border-green-500' : 'border-transparent'} `}
      ondragleave={onDragLeaveProjectCover}
      ondragover={onDraggingProjectCover}
      ondrop={onDropProjectCover}
    >
      {#if activeProject?.cover_asset_id}
        <img
          src={assetApi.thumbUrl(activeProject.cover_asset_id)}
          class="h-10 w-10 rounded object-cover"
          alt="Project cover"
        />
      {:else}
        <Box class="h-10 w-10 rounded text-gray-800 dark:text-gray-500" />
      {/if}
    </div>
  {/snippet}
</LibraryHeader>

{#if activeProject}
  <ProjectInfoPanel project={activeProject} />
{/if}

<TagFilterBar
  activeTags={assetsStore.activeTags}
  onchange={(tags) => assetsStore.setActiveTags(tags)}
/>

<CustomFieldFilters
  activeFilters={assetsStore.fieldFilters}
  onchange={(filters) => assetsStore.setFieldFilters(filters)}
/>

{#if zoomOverlay}
  <ZoomOverlay src={zoomOverlay.src} vars={zoomOverlay.vars} asset={zoomOverlay.asset} />
{/if}

<AssetGrid
  bind:mainEl
  {sort}
  {zoom}
  {maxZoom}
  {isDraggingFiles}
  {seenSplashScreen}
  onCardClick={handleCardClick}
  onDismissOnboarding={() => {
    seenSplashScreen = true
    localStorage.setItem(`onboard_${authStore.workspace?.id}`, 'true')
  }}
  onDragOver={handleMainDragOver}
  onDragLeave={handleMainDragLeave}
  onDrop={handleMainDrop}
  onMouseDown={(e) => rb.onMouseDown(e)}
/>

<LibraryStatusBar bind:zoom max={maxZoom - 1} />

{#if rb.band && rb.band.w > 2 && rb.band.h > 2}
  <div
    class="pointer-events-none fixed z-30 rounded border border-indigo-500 bg-indigo-500/15"
    style="left:{rb.band.x}px; top:{rb.band.y}px; width:{rb.band.w}px; height:{rb.band.h}px"
  ></div>
{/if}

<Lightbox
  asset={selectedAsset}
  onclose={() => (selectedAsset = null)}
  ondeleted={handleDeleted}
  ontagschanged={() => assetsStore.load(true)}
  onprojectchanged={() => { projectsStore.load(); assetsStore.load(true) }}
  onassetupdated={(updated) => {
    selectedAsset = updated
    assetsStore.patch(updated.id, updated)
    assetsStore.reloadResources(updated.id)
  }}
/>

<AssetBulkActionBar
  selectedIds={selectionStore.selectedIds}
  projects={projectsStore.projects}
  ondone={handleBulkDone}
  onclear={() => selectionStore.clear()}
/>

<UploadsTray />

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
