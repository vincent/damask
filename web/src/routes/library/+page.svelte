<script lang="ts">
  import { onMount } from 'svelte'
  import { assetApi, projectApi, folderApi, mimeCategory, type Asset, type Project, type Folder } from '$lib/api/client'
  import { authStore } from '$lib/stores/auth'
  import { uploadsStore } from '$lib/stores/uploads.svelte'
  import AssetCard from '$lib/components/AssetCard.svelte'
  import Lightbox from '$lib/components/Lightbox.svelte'
  import ProjectSidebar from '$lib/components/ProjectSidebar.svelte'
  import BulkActionBar from '$lib/components/BulkActionBar.svelte'
  import CommandPalette from '$lib/components/CommandPalette.svelte'

  type CategoryKey = 'image' | 'video' | 'audio' | 'document'

  const CATEGORY_ORDER: CategoryKey[] = ['image', 'video', 'audio', 'document']

  const CATEGORY_LABELS: Record<CategoryKey, string> = {
    image: 'Images & Graphics',
    video: 'Video Production',
    audio: 'Audio & Music',
    document: 'Documents',
  }

  const CATEGORY_ICON_BG: Record<CategoryKey, string> = {
    image: 'bg-violet-100 text-violet-600',
    video: 'bg-red-100 text-red-600',
    audio: 'bg-emerald-100 text-emerald-600',
    document: 'bg-blue-100 text-blue-600',
  }

  const CATEGORY_BORDER: Record<CategoryKey, string> = {
    image: 'border-violet-200',
    video: 'border-red-200',
    audio: 'border-emerald-200',
    document: 'border-blue-200',
  }

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

  // Grouped assets derived
  const assetsByCategory = $derived({
    image: assets.filter((a) => mimeCategory(a.mime_type) === 'image'),
    video: assets.filter((a) => mimeCategory(a.mime_type) === 'video'),
    audio: assets.filter((a) => mimeCategory(a.mime_type) === 'audio'),
    document: assets.filter((a) => mimeCategory(a.mime_type) === 'document'),
  })

  const activeProjectName = $derived(
    activeProjectId ? (projects.find((p) => p.id === activeProjectId)?.name ?? 'Folder') : null
  )

  const totalAssetCount = $derived(projects.reduce((s, p) => s + p.asset_count, 0))

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

  // Sidebar new folder creation shortcut
  let sidebarCreating = $state(false)

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
          {activeProjectId === null ? 'bg-gray-100 font-medium text-gray-900' : 'text-gray-600 hover:bg-gray-50'}"
        onclick={() => handleProjectSelect(null)}
      >
        <svg class="h-4 w-4 shrink-0 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
            d="M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z" />
        </svg>
        <span class="flex-1 text-left">All Assets</span>
        {#if totalAssetCount > 0}
          <span class="shrink-0 text-xs text-gray-400">{totalAssetCount}</span>
        {/if}
      </button>
    </div>

    <!-- Folders section -->
    <div class="flex-1 overflow-hidden flex flex-col px-3">
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
  <div class="flex flex-1 flex-col overflow-hidden">
    <!-- Top bar -->
    <header class="flex items-center justify-between border-b border-gray-100 bg-white px-6 py-4">
      <div>
        <h1 class="text-xl font-bold text-gray-900">
          {activeProjectName ?? 'Library'}
        </h1>
        <p class="mt-0.5 text-xs text-gray-400">
          All Assets{#if activeProjectName} / {activeProjectName}{/if}
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
            bind:value={query}
            oninput={handleSearch}
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
            <input type="file" multiple class="hidden" onchange={handleHeaderUpload} />
          </label>
        {/if}
      </div>
    </header>

    <!-- Content -->
    <main class="flex-1 overflow-y-auto px-6 py-6">
      {#if initialLoad}
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
      {:else if assets.length === 0}
        <div class="flex flex-col items-center justify-center py-24 text-center">
          <svg class="mb-4 h-16 w-16 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
          </svg>
          {#if query}
            <p class="text-sm font-medium text-gray-600">No results for "{query}"</p>
            <p class="mt-1 text-xs text-gray-400">Try a different search term</p>
          {:else}
            <p class="text-sm font-medium text-gray-600">No assets yet</p>
            <p class="mt-1 text-xs text-gray-400">Upload files to get started</p>
          {/if}
        </div>
      {:else}
        <!-- Grouped by category -->
        {#each CATEGORY_ORDER as cat}
          {@const group = assetsByCategory[cat]}
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
                  {#each group as asset, i (asset.id)}
                    {@const globalIndex = assets.indexOf(asset)}
                    <div class="relative">
                      {#if selectedIds.has(asset.id)}
                        <div class="pointer-events-none absolute inset-0 z-10 rounded-xl ring-2 ring-indigo-500">
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
