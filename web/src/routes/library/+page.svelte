<script lang="ts">
  import { onMount } from 'svelte'
  import { assetApi, projectApi, folderApi, type Asset, type Project, type Folder } from '$lib/api/client'
  import { authStore } from '$lib/stores/auth'
  import { uploadsStore } from '$lib/stores/uploads.svelte'
  import AssetCard from '$lib/components/AssetCard.svelte'
  import Lightbox from '$lib/components/Lightbox.svelte'
  import UploadZone from '$lib/components/UploadZone.svelte'
  import ProjectSidebar from '$lib/components/ProjectSidebar.svelte'
  import TagFilterBar from '$lib/components/TagFilterBar.svelte'
  import BulkActionBar from '$lib/components/BulkActionBar.svelte'
  import CommandPalette from '$lib/components/CommandPalette.svelte'

  let assets = $state<Asset[]>([])
  let nextCursor = $state<string | null>(null)
  let loading = $state(false)
  let initialLoad = $state(true)
  let query = $state('')
  let selectedAsset = $state<Asset | null>(null)
  let sentinel = $state<HTMLDivElement | undefined>(undefined)

  // Projects & filtering
  let projects = $state<Project[]>([])
  let activeProjectId = $state<string | null>(null)
  let activeTags = $state<string[]>([])

  // Folders
  let activeFolderId = $state<string | null>(null)
  let foldersByProject = $state<Record<string, Folder[]>>({})
  let toast = $state<{ msg: string; type: 'success' | 'error' } | null>(null)
  let toastTimer: ReturnType<typeof setTimeout>

  // Multi-select
  let selectedIds = $state(new Set<string>())
  let lastSelectedIndex = $state<number>(-1)

  // Command palette
  let showPalette = $state(false)

  let debounceTimer: ReturnType<typeof setTimeout>

  async function loadProjects() {
    try {
      projects = await projectApi.list()
    } catch {
      // silently ignore
    }
  }

  async function loadAssets(reset = false) {
    if (loading) return
    loading = true
    try {
      const result = await assetApi.list({
        cursor: reset ? undefined : (nextCursor ?? undefined),
        q: query || undefined,
        project_id: activeProjectId ?? undefined,
        tags: activeTags.length > 0 ? activeTags : undefined,
        folder_id: activeFolderId ?? undefined,
        limit: 48,
      })
      if (reset) {
        assets = result.assets
      } else {
        assets = [...assets, ...result.assets]
      }
      nextCursor = result.next_cursor
    } catch {
      // 401 redirect handled by api client
    } finally {
      loading = false
      initialLoad = false
    }
  }

  function handleSearch() {
    clearTimeout(debounceTimer)
    debounceTimer = setTimeout(() => {
      nextCursor = null
      loadAssets(true)
    }, 300)
  }

  function handleUploaded(asset: Asset) {
    assets = [asset, ...assets]
  }

  function handleDeleted(id: string) {
    assets = assets.filter((a) => a.id !== id)
    selectedAsset = null
    selectedIds.delete(id)
    selectedIds = new Set(selectedIds)
  }

  function handleHeaderUpload(e: Event) {
    const files = Array.from((e.target as HTMLInputElement).files ?? [])
    for (const file of files) {
      const id = crypto.randomUUID()
      uploadsStore.add({ id, file, progress: 0, status: 'uploading' })
      assetApi
        .upload(file, (pct) => uploadsStore.update(id, { progress: pct }))
        .then((asset) => {
          handleUploaded(asset)
          uploadsStore.update(id, { status: 'done', asset })
        })
        .catch((err: Error) => uploadsStore.update(id, { status: 'error', error: err.message }))
    }
    ;(e.target as HTMLInputElement).value = ''
  }

  function handleCardClick(asset: Asset, index: number, event: MouseEvent) {
    if (event.shiftKey && $authStore.role !== 'viewer') {
      const newSet = new Set(selectedIds)
      if (lastSelectedIndex >= 0 && lastSelectedIndex !== index) {
        const lo = Math.min(lastSelectedIndex, index)
        const hi = Math.max(lastSelectedIndex, index)
        for (let i = lo; i <= hi; i++) {
          newSet.add(assets[i].id)
        }
      } else {
        if (newSet.has(asset.id)) {
          newSet.delete(asset.id)
        } else {
          newSet.add(asset.id)
          lastSelectedIndex = index
        }
      }
      selectedIds = newSet
    } else if (selectedIds.size > 0 && $authStore.role !== 'viewer') {
      const newSet = new Set(selectedIds)
      if (newSet.has(asset.id)) {
        newSet.delete(asset.id)
      } else {
        newSet.add(asset.id)
        lastSelectedIndex = index
      }
      selectedIds = newSet
    } else {
      selectedAsset = asset
    }
  }

  async function loadFolders(projectId: string) {
    try {
      const data = await folderApi.list(projectId)
      foldersByProject = { ...foldersByProject, [projectId]: data }
    } catch {
      // silently ignore
    }
  }

  function handleProjectSelect(id: string | null) {
    activeProjectId = id
    activeFolderId = null
    nextCursor = null
    if (id) loadFolders(id)
    loadAssets(true)
  }

  function handleFolderSelect(projectId: string, folderId: string | null) {
    activeFolderId = folderId
    nextCursor = null
    loadAssets(true)
  }

  function showToast(msg: string, type: 'success' | 'error' = 'success') {
    clearTimeout(toastTimer)
    toast = { msg, type }
    toastTimer = setTimeout(() => { toast = null }, 3000)
  }

  async function handleAssetsDropped(assetIds: string[], folderId: string | null, projectId: string) {
    try {
      await Promise.all(assetIds.map((id) => assetApi.updateFolder(id, folderId)))
      showToast(`Moved ${assetIds.length} asset${assetIds.length > 1 ? 's' : ''}`)
      loadFolders(projectId)
      loadAssets(true)
    } catch {
      showToast('Could not move assets', 'error')
    }
  }

  function handleTagChange(tags: string[]) {
    activeTags = tags
    nextCursor = null
    loadAssets(true)
  }

  function handleBulkDone() {
    selectedIds = new Set()
    lastSelectedIndex = -1
    loadProjects()
    loadAssets(true)
  }

  function handleWindowKeydown(e: KeyboardEvent) {
    if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
      e.preventDefault()
      showPalette = !showPalette
    }
    if (e.key === 'Escape' && !showPalette) {
      selectedIds = new Set()
    }
  }

  onMount(() => {
    loadProjects()
    loadAssets(true)

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && nextCursor && !loading) {
          loadAssets()
        }
      },
      { rootMargin: '200px' }
    )

    if (sentinel) observer.observe(sentinel)
    return () => observer.disconnect()
  })
</script>

<svelte:head>
  <title>Library — Creativo DAM</title>
</svelte:head>

<svelte:window onkeydown={handleWindowKeydown} />

<div class="flex h-screen bg-gray-100">
  <!-- Sidebar -->
  <aside class="flex w-64 shrink-0 flex-col border-r border-gray-200 bg-white">
    <div class="flex items-center justify-between px-4 py-4">
      <span class="text-sm font-semibold text-gray-900">Creativo</span>
      <a href="/logout" class="text-xs text-gray-400 hover:text-gray-600">Sign out</a>
    </div>

    {#if $authStore.workspace}
      <div class="border-b border-gray-100 px-4 pb-3">
        <p class="truncate text-xs text-gray-500">{$authStore.workspace.name}</p>
      </div>
    {/if}

    <nav class="flex-1 overflow-y-auto px-2 py-3">
      <ProjectSidebar
        {projects}
        {activeProjectId}
        folders={foldersByProject}
        {activeFolderId}
        selectedAssetIds={selectedIds}
        onselect={handleProjectSelect}
        onchange={() => { loadProjects(); loadAssets(true) }}
        onfolderschange={(projectId) => loadFolders(projectId)}
        onfolderselect={handleFolderSelect}
        onassetsDropped={handleAssetsDropped}
      />
    </nav>

    <div class="border-t border-gray-100 px-4 py-3">
      <button
        class="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-xs text-gray-400 hover:bg-gray-100"
        onclick={() => { showPalette = true }}
      >
        <span>Jump to project</span>
        <kbd class="ml-auto rounded border border-gray-200 px-1.5 py-0.5 text-xs">⌘K</kbd>
      </button>
    </div>
  </aside>

  <!-- Main -->
  <div class="flex flex-1 flex-col overflow-hidden">
    <!-- Top bar -->
    <header class="flex flex-col border-b border-gray-200 bg-white">
      <div class="flex items-center gap-3 px-6 py-3">
        <h1 class="text-base font-semibold text-gray-900">
          {#if activeProjectId}
            {projects.find((p) => p.id === activeProjectId)?.name ?? 'Project'}
          {:else}
            Library
          {/if}
        </h1>
        <div class="relative ml-auto w-72">
          <svg
            class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          <input
            type="search"
            placeholder="Search assets…"
            bind:value={query}
            oninput={handleSearch}
            class="w-full rounded-lg border border-gray-300 bg-gray-50 py-1.5 pl-9 pr-3 text-sm placeholder-gray-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
        </div>
        {#if $authStore.role !== 'viewer'}
          <label class="cursor-pointer rounded-lg bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700">
            Upload
            <input type="file" multiple class="hidden" onchange={handleHeaderUpload} />
          </label>
        {/if}
      </div>

      <TagFilterBar {activeTags} onchange={handleTagChange} />
    </header>

    <!-- Content -->
    <main class="flex-1 overflow-y-auto px-6 py-5">
      {#if $authStore.role !== 'viewer'}
        <div class="mb-5">
          <UploadZone onuploaded={handleUploaded} />
        </div>
      {/if}

      {#if initialLoad}
        <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
          {#each { length: 12 } as _}
            <div class="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm">
              <div class="aspect-square animate-pulse bg-gray-200"></div>
              <div class="p-2.5">
                <div class="mb-1.5 h-3 w-3/4 animate-pulse rounded bg-gray-200"></div>
                <div class="h-3 w-1/2 animate-pulse rounded bg-gray-200"></div>
              </div>
            </div>
          {/each}
        </div>
      {:else if assets.length === 0}
        <div class="flex flex-col items-center justify-center py-24 text-center">
          <svg class="mb-4 h-16 w-16 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
          </svg>
          {#if query}
            <p class="text-sm font-medium text-gray-600">No results for "{query}"</p>
            <p class="mt-1 text-xs text-gray-400">Try a different search term</p>
          {:else if activeTags.length > 0}
            <p class="text-sm font-medium text-gray-600">No assets match the selected tags</p>
            <button class="mt-2 text-xs text-blue-600 hover:underline" onclick={() => handleTagChange([])}>Clear filters</button>
          {:else}
            <p class="text-sm font-medium text-gray-600">No assets yet</p>
            <p class="mt-1 text-xs text-gray-400">Upload files above to get started</p>
          {/if}
        </div>
      {:else}
        <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
          {#each assets as asset, i (asset.id)}
            <div class="relative">
              {#if selectedIds.has(asset.id)}
                <div class="pointer-events-none absolute inset-0 z-10 rounded-lg ring-2 ring-blue-500">
                  <div class="absolute right-1.5 top-1.5 flex h-5 w-5 items-center justify-center rounded-full bg-blue-600">
                    <svg class="h-3 w-3 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7" />
                    </svg>
                  </div>
                </div>
              {/if}
              <AssetCard {asset} onclick={(e) => handleCardClick(asset, i, e)} />
            </div>
          {/each}
        </div>

        {#if nextCursor}
          <div bind:this={sentinel} class="flex justify-center py-6">
            {#if loading}
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
  </div>
</div>

{#if toast}
  <div class="fixed bottom-4 right-4 z-50 rounded-lg px-4 py-3 text-sm font-medium shadow-lg transition-all {toast.type === 'error' ? 'bg-red-600 text-white' : 'bg-gray-900 text-white'}">
    {toast.msg}
  </div>
{/if}

<Lightbox
  asset={selectedAsset}
  {projects}
  onclose={() => (selectedAsset = null)}
  ondeleted={handleDeleted}
  ontagschanged={() => loadAssets(true)}
  onprojectchanged={() => { loadProjects(); loadAssets(true) }}
/>

<BulkActionBar
  {selectedIds}
  {projects}
  ondone={handleBulkDone}
  onclear={() => { selectedIds = new Set(); lastSelectedIndex = -1 }}
/>

{#if showPalette}
  <CommandPalette
    {projects}
    onselect={(id) => { handleProjectSelect(id); showPalette = false }}
    onclose={() => { showPalette = false }}
  />
{/if}
