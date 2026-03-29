<script lang="ts">
  import { onMount } from 'svelte'
  import { mimeCategory, type Asset } from '$lib/api/client'
  import { authStore } from '$lib/stores/auth'
  import { assetsStore } from '$lib/stores/assets.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import { foldersStore } from '$lib/stores/folders.svelte'
  import { navigationStore } from '$lib/stores/navigation.svelte'
  import { selectionStore } from '$lib/stores/selection.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import AssetCard from '$lib/components/AssetCard.svelte'
  import Lightbox from '$lib/components/Lightbox.svelte'
  import ProjectSidebar from '$lib/components/ProjectSidebar.svelte'
  import BulkActionBar from '$lib/components/BulkActionBar.svelte'
  import CommandPalette from '$lib/components/CommandPalette.svelte'
  import { CATEGORY_BORDER, CATEGORY_ICON_BG, CATEGORY_LABELS, CATEGORY_ORDER } from '$lib/stores/shared'

  let selectedAsset = $state<Asset | null>(null)
  let sentinel = $state<HTMLDivElement | undefined>(undefined)
  let showPalette = $state(false)
  let sidebarCreating = $state(false)

  function handleCardClick(asset: Asset, index: number, event: MouseEvent) {
    const handled = selectionStore.handleCardClick(
      asset,
      index,
      assetsStore.assets,
      event,
      $authStore.role !== 'viewer',
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
  <title>Library — Creativo DAM</title>
</svelte:head>

<svelte:window onkeydown={handleWindowKeydown} />

<div class="flex h-screen bg-gray-50">
  <!-- Sidebar -->
  <aside class="flex w-64 shrink-0 flex-col border-r border-gray-100 bg-white">
    <!-- Workspace logo + name -->
    <div class="flex items-center gap-2.5 px-4 py-4">
      <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-gradient-to-br from-indigo-500 to-purple-600 text-sm font-bold text-white">
        {$authStore.workspace?.name?.[0]?.toUpperCase() ?? 'S'}
      </div>
      <span class="truncate text-sm font-semibold text-gray-900">
        {$authStore.workspace?.name ?? 'Workspace'}
      </span>
    </div>

    <!-- All Assets button -->
    <div class="px-3 pb-2">
      <button
        class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors
          {navigationStore.activeProjectId === null ? 'bg-gray-100 font-medium text-gray-900' : 'text-gray-600 hover:bg-gray-50'}"
        onclick={() => handleProjectSelect(null)}
      >
        <svg class="h-4 w-4 shrink-0 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
            d="M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z" />
        </svg>
        <span class="flex-1 text-left">All Assets</span>
        {#if projectsStore.totalAssetCount > 0}
          <span class="shrink-0 text-xs text-gray-400">{projectsStore.totalAssetCount}</span>
        {/if}
      </button>
    </div>

    <!-- Folders section -->
    <div class="flex flex-1 flex-col overflow-hidden px-3">
      <div class="mb-2 flex items-center justify-between px-2">
        <span class="text-[10px] font-semibold uppercase tracking-widest text-gray-400">Folders</span>
        {#if $authStore.role !== 'viewer'}
          <button
            class="rounded p-0.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600"
            onclick={() => { sidebarCreating = true }}
            aria-label="New folder"
          >
            <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
            </svg>
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

    <!-- Bottom sign out -->
    <div class="border-t border-gray-100 px-4 py-3">
      <a href="/logout" class="flex items-center gap-2 text-xs text-gray-400 hover:text-gray-600">
        <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
        </svg>
        Sign out
      </a>
    </div>
  </aside>

  <!-- Main -->
  <div class="relative flex flex-1 flex-col overflow-hidden">
    <!-- Top bar -->
    <header class="flex items-center justify-between border-b border-gray-100 bg-white px-6 py-4">
      <div>
        <h1 class="text-xl font-bold text-gray-900">
          {projectsStore.activeProjectName ?? 'Library'}
        </h1>
        <p class="mt-0.5 text-xs text-gray-400">
          All Assets{#if projectsStore.activeProjectName} / {projectsStore.activeProjectName}{/if}
        </p>
      </div>

      <div class="flex items-center gap-2">
        <!-- Search -->
        <div class="relative w-64">
          <svg
            class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400"
            fill="none" viewBox="0 0 24 24" stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          <input
            type="search"
            placeholder="Search anything..."
            value={assetsStore.query}
            oninput={(e) => { assetsStore.query = e.currentTarget.value; assetsStore.search() }}
            class="w-full rounded-lg border border-gray-200 bg-gray-50 py-2 pl-9 pr-3 text-sm placeholder-gray-400 focus:border-indigo-400 focus:bg-white focus:outline-none focus:ring-1 focus:ring-indigo-400"
          />
        </div>

        <!-- Filter icon -->
        <button class="flex h-9 w-9 items-center justify-center rounded-lg border border-gray-200 text-gray-400 hover:bg-gray-50">
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z" />
          </svg>
        </button>

        {#if $authStore.role !== 'viewer'}
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
        <!-- Loading skeleton -->
        <div class="mb-10">
          <div class="mb-4 flex items-center gap-3">
            <div class="h-8 w-8 animate-pulse rounded-lg bg-gray-200"></div>
            <div class="h-4 w-32 animate-pulse rounded bg-gray-200"></div>
          </div>
          <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-5">
            {#each { length: 10 } as _}
              <div class="overflow-hidden rounded-xl bg-white shadow-sm">
                <div class="animate-pulse bg-gray-200" style="aspect-ratio: 4/3"></div>
                <div class="p-3">
                  <div class="mb-2 h-3 w-3/4 animate-pulse rounded bg-gray-200"></div>
                  <div class="h-3 w-1/2 animate-pulse rounded bg-gray-200"></div>
                </div>
              </div>
            {/each}
          </div>
        </div>
      {:else if assetsStore.assets.length === 0}
        <div class="flex flex-col items-center justify-center py-24 text-center">
          <svg class="mb-4 h-16 w-16 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
          </svg>
          {#if assetsStore.query}
            <p class="text-sm font-medium text-gray-600">No results for "{assetsStore.query}"</p>
            <p class="mt-1 text-xs text-gray-400">Try a different search term</p>
          {:else}
            <p class="text-sm font-medium text-gray-600">No assets yet</p>
            <p class="mt-1 text-xs text-gray-400">Upload files to get started</p>
          {/if}
        </div>
      {:else}
        <!-- Grouped by category -->
        {#each CATEGORY_ORDER as cat}
          {@const group = assetsStore.assetsByCategory[cat]}
          {#if group.length > 0}
            <div class="mb-10">
              <!-- Category header -->
              <div class="mb-4 flex items-center gap-3">
                <div class="flex h-8 w-8 items-center justify-center rounded-lg {CATEGORY_ICON_BG[cat]}">
                  {#if cat === 'image'}
                    <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
                    </svg>
                  {:else if cat === 'video'}
                    <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 10l4.553-2.069A1 1 0 0121 8.87v6.26a1 1 0 01-1.447.894L15 14M4 8h11a1 1 0 011 1v6a1 1 0 01-1 1H4a1 1 0 01-1-1V9a1 1 0 011-1z" />
                    </svg>
                  {:else if cat === 'audio'}
                    <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19V6l12-3v13M9 19c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zm12-3c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zM9 10l12-3" />
                    </svg>
                  {:else}
                    <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
                    </svg>
                  {/if}
                </div>
                <h2 class="text-sm font-semibold text-gray-900">{CATEGORY_LABELS[cat]}</h2>
                <span class="text-sm text-gray-400">{group.length}</span>
              </div>

              <!-- Cards with left accent border -->
              <div class="border-l-2 {CATEGORY_BORDER[cat]} pl-4">
                <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-5">
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
                      <AssetCard {asset} onclick={(e) => handleCardClick(asset, globalIndex, e)} />
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
              <svg class="h-6 w-6 animate-spin text-gray-400" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
              </svg>
            {/if}
          </div>
        {:else}
          <div bind:this={sentinel}></div>
        {/if}
      {/if}
    </main>

    <!-- status bar -->
    <div class="absolute z-10 bottom-0 right-0 left-0 flex items-center justify-between border-t border-gray-200 bg-white px-4 py-3">
      <p class="text-sm text-gray-500">
        {assetsStore.assets.length} assets
      </p>
      <p class="text-sm text-gray-500">
        Zoom
      </p>
    </div>

  </div>
</div>

{#if toastStore.current}
  <div class="fixed bottom-4 right-4 z-50 rounded-lg px-4 py-3 text-sm font-medium shadow-lg transition-all {toastStore.current.type === 'error' ? 'bg-red-600 text-white' : 'bg-gray-900 text-white'}">
    {toastStore.current.msg}
  </div>
{/if}

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
