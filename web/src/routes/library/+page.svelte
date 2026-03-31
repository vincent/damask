<script lang="ts">
  import { onMount } from 'svelte'
  import { type Asset } from '$lib/api/client'
  import { authStore } from '$lib/stores/auth.svelte'
  import { assetsStore } from '$lib/stores/assets.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import { foldersStore } from '$lib/stores/folders.svelte'
  import { navigationStore } from '$lib/stores/navigation.svelte'
  import { selectionStore } from '$lib/stores/selection.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { sharesStore } from '$lib/stores/shares.svelte'
  import AssetCard from '$lib/components/AssetCard.svelte'
  import Lightbox from '$lib/components/Lightbox.svelte'
  import ProjectSidebar from '$lib/components/ProjectSidebar.svelte'
  import BulkActionBar from '$lib/components/BulkActionBar.svelte'
  import CommandPalette from '$lib/components/CommandPalette.svelte'
  import ShareModal from '$lib/components/ShareModal.svelte'
  import { CATEGORY_BORDER, CATEGORY_ICON_BG, CATEGORY_LABELS, CATEGORY_ORDER } from '$lib/stores/shared'
  import { Book, Inbox, Loader, LogOut, Plus, Search, Share2 } from '@lucide/svelte'
  import ThemeToggle from '$lib/components/ThemeToggle.svelte'
  import LibraryStatusBar from '$lib/components/LibraryStatusBar.svelte'
  import SearchInput from '$lib/components/ui/SearchInput.svelte'
  import EmptyState from '$lib/components/ui/EmptyState.svelte'
  import Toast from '$lib/components/ui/Toast.svelte'
  import GridSkeleton from './GridSkeleton.svelte';
  import WorkspaceSwitcher from '$lib/components/WorkspaceSwitcher.svelte'
  import AssetIcon from '../../lib/components/AssetIcon.svelte'

  let selectedAsset = $state<Asset | null>(null)
  let sentinel = $state<HTMLDivElement | undefined>(undefined)
  let showPalette = $state(false)
  let sidebarCreating = $state(false)
  let showProjectShareModal = $state(false)
  let zoom = $state(10)
  const maxZoom = 20

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
    if (!handled) selectedAsset = asset
  }

  async function handleProjectSelect(id: string | null) {
    navigationStore.selectProject(id)
    if (id) await foldersStore.loadForProject(id)
    await assetsStore.load(true)
  }

  async function handleFolderSelect(_projectId: string, folderId: string | null) {
    navigationStore.selectFolder(folderId)
    await assetsStore.load(true)
  }

  async function handleAssetsDropped(assetIds: string[], folderId: string | null, projectId: string) {
    try {
      await foldersStore.moveAssets(assetIds, folderId, projectId)
      toastStore.show(`Moved ${assetIds.length} asset${assetIds.length > 1 ? 's' : ''}`)
      await assetsStore.load(true)
    } catch {
      toastStore.show('Could not move assets', 'error')
    }
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

  function handleWindowKeydown(e: KeyboardEvent) {
    if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
      e.preventDefault()
      showPalette = !showPalette
    }
    if (e.key === 'Escape' && !showPalette) selectionStore.clear()
  }

  onMount(() => {
    projectsStore.load()
    assetsStore.load(true)
    sharesStore.load()

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && assetsStore.nextCursor && !assetsStore.loading) {
          assetsStore.load()
        }
      },
      { rootMargin: '200px' },
    )

    if (sentinel) observer.observe(sentinel)
    return () => observer.disconnect()
  })
</script>

<svelte:head>
  <title>Library — Damask DAM</title>
</svelte:head>

<svelte:window onkeydown={handleWindowKeydown} />

<div class="bg-[var(--bg-app)] flex h-screen bg-gray-50 dark:bg-gray-950">
  <!-- Sidebar -->
  <aside class="damask-texture relative flex w-64 shrink-0 flex-col border-r border-gray-100 bg-white dark:border-gray-800 dark:bg-gray-900">

    <!-- Workspace switcher -->
    <WorkspaceSwitcher class="px-3 py-3" />

    <!-- All Assets button -->
    <div class="px-3 pb-2">
      <button
        class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors
          {navigationStore.activeProjectId === null ? 'bg-gray-100 font-medium text-gray-900 dark:bg-gray-800 dark:text-gray-50' : 'text-gray-600 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800'}"
        onclick={() => handleProjectSelect(null)}
      >
        <Book class="h-4 w-4 shrink-0 text-gray-400" />
        <span class="flex-1 text-left">All Assets</span>
        {#if projectsStore.totalAssetCount > 0}
          <span class="shrink-0 text-xs text-gray-400">{projectsStore.totalAssetCount}</span>
        {/if}
      </button>
    </div>

    <!-- Folders section -->
    <div class="flex flex-1 flex-col overflow-hidden px-3">
      <div class="mb-2 flex items-center justify-between px-2">
        <span class="text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">Folders</span>
        {#if authStore.role !== 'viewer'}
          <button
            class="rounded p-0.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600"
            onclick={() => { sidebarCreating = true }}
            aria-label="New folder"
          >
            <Plus class="h-3.5 w-3.5" />
          </button>
        {/if}
      </div>

      <nav class="flex-1 overflow-y-auto">
        <ProjectSidebar
          selectedAssetIds={selectionStore.selectedIds}
          creating={sidebarCreating}
          oncreatingchange={(v) => { sidebarCreating = v }}
          onselect={handleProjectSelect}
          onfolderselect={handleFolderSelect}
          onassetsDropped={handleAssetsDropped}
        />
      </nav>
    </div>

    <!-- Bottom sign out + theme toggle -->
    <div class="flex items-center justify-between border-t border-gray-200 px-4 py-3 dark:border-gray-800">
      <ThemeToggle />
      <a href="/logout" class="flex items-center gap-2 rounded-lg px-2 text-sm text-gray-400 hover:text-gray-700 dark:hover:text-gray-200">
        <LogOut class="h-3.5 w-3.5" />
        Sign out
      </a>
    </div>
  </aside>

  <!-- Main -->
  <div class="relative flex flex-1 flex-col overflow-hidden">
    <!-- Top bar -->
    <header class="flex items-center justify-between border-b border-gray-100 bg-white px-6 py-4 dark:border-gray-800 dark:bg-gray-900">
      <div class="flex items-center gap-3">
        <div>
          <h1 class="text-xl font-bold text-gray-900 dark:text-gray-50">
            {projectsStore.activeProjectName ?? 'Library'}
          </h1>
          <p class="mt-0.5 text-xs text-gray-400">
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
        <!-- Search -->
        <SearchInput
          class="w-64"
          value={assetsStore.query}
          placeholder="Search anything..."
          onchange={(v) => { assetsStore.query = v; assetsStore.search() }}
        />

        <!-- Filter icon -->
        <button class="flex h-9 w-9 items-center justify-center rounded-lg border border-gray-200 text-gray-400 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-500 dark:hover:bg-gray-800">
          <Search class="h-4 w-4" />
        </button>

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

    <!-- Content -->
    <main class="flex-1 overflow-y-auto px-6 py-6">
      {#if assetsStore.initialLoad}
        <GridSkeleton x={7} y={3} {zoom} {maxZoom} />
      {:else if assetsStore.assets.length === 0}
        <EmptyState
          title={assetsStore.query ? `No results for "${assetsStore.query}"` : 'No assets yet'}
          description={assetsStore.query ? 'Try a different search term' : 'Upload files to get started'}
        >
          {#snippet icon()}<Inbox class="h-16 w-16" />{/snippet}
        </EmptyState>
      {:else}
        <!-- Grouped by category -->
        {#each CATEGORY_ORDER as cat}
          {@const group = assetsStore.assetsByCategory[cat]}
          {#if group.length > 0}
            <div class="mb-10">
              <!-- Category header -->
              <div class="mb-4 flex items-center gap-3">
                <AssetIcon category={cat} class="h-8 w-8 items-center justify-center rounded-lg {CATEGORY_ICON_BG[cat].light} {CATEGORY_ICON_BG[cat].dark}"/>
                <h2 class="text-sm font-semibold text-gray-900 dark:text-gray-50">{CATEGORY_LABELS[cat]}</h2>
                <span class="text-sm text-gray-400 dark:text-gray-500">{group.length}</span>
              </div>

              <!-- Cards with left accent border -->
              <div class="border-l-2 {CATEGORY_BORDER[cat]} pl-4">
                <div class="grid gap-3 grid-cols-{1 + maxZoom - Math.floor(zoom)}">
                  {#each group as asset (asset.id)}
                    {@const globalIndex = assetsStore.assets.indexOf(asset)}
                    <div class="relative">
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
        {#if assetsStore.nextCursor}
          <div bind:this={sentinel} class="flex justify-center py-6">
            {#if assetsStore.loading}
              <Loader class="h-6 w-6 animate-spin text-gray-400" />
            {/if}
          </div>
        {:else}
          <div bind:this={sentinel}></div>
        {/if}
      {/if}
    </main>

    <LibraryStatusBar bind:zoom={zoom} max={maxZoom - 1} />
  </div>
</div>

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
    onselect={(id) => { handleProjectSelect(id); showPalette = false }}
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
