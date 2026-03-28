<script lang="ts">
  import { folderApi, type Folder } from '$lib/api/client'
  import { authStore } from '$lib/stores/auth'

  interface Props {
    folders: Folder[]
    activeFolderId: string | null
    projectId: string
    selectedAssetIds: Set<string>
    onselect: (folderId: string | null) => void
    onchange: () => void
    onassetsDropped: (assetIds: string[], folderId: string | null) => void
  }

  let { folders, activeFolderId, projectId, selectedAssetIds, onselect, onchange, onassetsDropped }: Props = $props()

  const STORAGE_KEY = 'folder-tree-open'

  function loadOpenIds(): Set<string> {
    try {
      const raw = localStorage.getItem(STORAGE_KEY)
      if (raw) return new Set(JSON.parse(raw))
    } catch {}
    return new Set()
  }

  function saveOpenIds(ids: Set<string>) {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify([...ids]))
    } catch {}
  }

  let openFolderIds = $state<Set<string>>(loadOpenIds())
  let contextMenuId = $state<string | null>(null)
  let editingId = $state<string | null>(null)
  let editName = $state('')
  let creatingUnder = $state<string | null>(null)
  let newFolderName = $state('')
  let dropTargetId = $state<string | null>(null)
  let error = $state('')

  function toggleOpen(id: string, e: MouseEvent) {
    e.stopPropagation()
    const next = new Set(openFolderIds)
    if (next.has(id)) {
      next.delete(id)
    } else {
      next.add(id)
    }
    openFolderIds = next
    saveOpenIds(next)
  }

  async function submitRename(id: string) {
    const name = editName.trim()
    if (!name) { editingId = null; return }
    try {
      await folderApi.update(id, { name })
      editingId = null
      onchange()
    } catch {
      error = 'Could not rename folder'
    }
  }

  async function deleteFolder(id: string) {
    contextMenuId = null
    try {
      await folderApi.delete(id)
      onchange()
    } catch {
      error = 'Could not delete folder'
    }
  }

  async function submitCreate(parentId: string | null) {
    const name = newFolderName.trim()
    if (!name) { creatingUnder = null; return }
    try {
      const data: { name: string; parent_id?: string } = { name }
      if (parentId) data.parent_id = parentId
      await folderApi.create(projectId, data)
      newFolderName = ''
      creatingUnder = null
      onchange()
    } catch {
      error = 'Could not create folder'
    }
  }

  function handleDragOver(e: DragEvent, folderId: string | null) {
    e.preventDefault()
    dropTargetId = folderId
  }

  function handleDragLeave() {
    dropTargetId = null
  }

  function handleDrop(e: DragEvent, folderId: string | null) {
    e.preventDefault()
    dropTargetId = null
    const assetId = e.dataTransfer?.getData('text/plain')
    if (!assetId) return
    let assetIds: string[]
    if (selectedAssetIds.has(assetId) && selectedAssetIds.size > 1) {
      assetIds = [...selectedAssetIds]
    } else {
      assetIds = [assetId]
    }
    onassetsDropped(assetIds, folderId)
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      editingId = null
      contextMenuId = null
      creatingUnder = null
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

{#each folders as folder (folder.id)}
  <div class="mt-0.5">
    <!-- Root folder row -->
    <div
      class="group relative flex items-center rounded-md transition-colors
        {activeFolderId === folder.id ? 'bg-blue-50' : ''}
        {dropTargetId === folder.id ? 'bg-blue-100 ring-1 ring-blue-400' : ''}
      "
      ondragover={(e) => handleDragOver(e, folder.id)}
      ondragleave={handleDragLeave}
      ondrop={(e) => handleDrop(e, folder.id)}
    >
      <!-- Chevron -->
      {#if folder.children && folder.children.length > 0}
        <button
          class="shrink-0 p-1 text-gray-400 hover:text-gray-600"
          onclick={(e) => toggleOpen(folder.id, e)}
          aria-label="Toggle folder"
        >
          <svg class="h-3 w-3 transition-transform {openFolderIds.has(folder.id) ? 'rotate-90' : ''}" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
          </svg>
        </button>
      {:else}
        <span class="w-5 shrink-0"></span>
      {/if}

      {#if editingId === folder.id}
        <form class="flex flex-1 items-center gap-1 pr-1" onsubmit={(e) => { e.preventDefault(); submitRename(folder.id) }}>
          <input
            autofocus
            bind:value={editName}
            class="min-w-0 flex-1 rounded border border-blue-400 px-1.5 py-0.5 text-xs outline-none"
            onblur={() => submitRename(folder.id)}
          />
        </form>
      {:else}
        <button
          class="flex min-w-0 flex-1 items-center gap-1.5 py-1.5 pr-1 text-left text-sm {activeFolderId === folder.id ? 'text-blue-700 font-medium' : 'text-gray-600 hover:text-gray-900'}"
          onclick={() => onselect(folder.id)}
        >
          <svg class="h-3.5 w-3.5 shrink-0 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V7z" />
          </svg>
          <span class="min-w-0 flex-1 truncate text-xs">{folder.name}</span>
          {#if folder.asset_count > 0}
            <span class="shrink-0 text-xs text-gray-400">{folder.asset_count}</span>
          {/if}
        </button>
      {/if}

      <!-- Context menu trigger -->
      {#if $authStore.role !== 'viewer'}
        <button
          class="absolute right-1 top-1/2 -translate-y-1/2 rounded p-0.5 text-gray-400 opacity-0 hover:bg-gray-200 hover:text-gray-700 group-hover:opacity-100"
          onclick={(e) => { e.stopPropagation(); contextMenuId = contextMenuId === folder.id ? null : folder.id }}
          aria-label="Folder menu"
        >
          <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 5v.01M12 12v.01M12 19v.01M12 6a1 1 0 110-2 1 1 0 010 2zm0 7a1 1 0 110-2 1 1 0 010 2zm0 7a1 1 0 110-2 1 1 0 010 2z" />
          </svg>
        </button>
      {/if}

      {#if contextMenuId === folder.id}
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div
          class="absolute right-0 top-full z-30 mt-0.5 w-40 rounded-lg border border-gray-200 bg-white py-1 shadow-lg"
          onmouseleave={() => { contextMenuId = null }}
        >
          <button
            class="flex w-full items-center gap-2 px-3 py-1.5 text-xs text-gray-700 hover:bg-gray-50"
            onclick={() => { editingId = folder.id; editName = folder.name; contextMenuId = null }}
          >Rename</button>
          {#if !folder.parent_id.Valid}
            <button
              class="flex w-full items-center gap-2 px-3 py-1.5 text-xs text-gray-700 hover:bg-gray-50"
              onclick={() => { creatingUnder = folder.id; contextMenuId = null; newFolderName = '' }}
            >Add subfolder</button>
          {/if}
          {#if $authStore.role === 'owner'}
            <button
              class="flex w-full items-center gap-2 px-3 py-1.5 text-xs text-red-600 hover:bg-red-50"
              onclick={() => deleteFolder(folder.id)}
            >Delete</button>
          {/if}
        </div>
      {/if}
    </div>

    <!-- Create subfolder form -->
    {#if creatingUnder === folder.id}
      <div class="ml-5">
        <form
          class="flex items-center gap-1 rounded-md border border-gray-200 bg-gray-50 px-2 py-1"
          onsubmit={(e) => { e.preventDefault(); submitCreate(folder.id) }}
        >
          <input
            autofocus
            bind:value={newFolderName}
            placeholder="Folder name"
            class="min-w-0 flex-1 bg-transparent text-xs outline-none"
            onblur={() => { if (!newFolderName.trim()) creatingUnder = null }}
          />
          <button type="submit" class="shrink-0 text-xs text-blue-600 hover:text-blue-800">Add</button>
        </form>
      </div>
    {/if}

    <!-- Children -->
    {#if openFolderIds.has(folder.id) && folder.children && folder.children.length > 0}
      <div class="ml-4 border-l border-gray-100 pl-1">
        {#each folder.children as child (child.id)}
          <div
            class="group relative flex items-center rounded-md transition-colors
              {activeFolderId === child.id ? 'bg-blue-50' : ''}
              {dropTargetId === child.id ? 'bg-blue-100 ring-1 ring-blue-400' : ''}
            "
            ondragover={(e) => handleDragOver(e, child.id)}
            ondragleave={handleDragLeave}
            ondrop={(e) => handleDrop(e, child.id)}
          >
            <span class="w-5 shrink-0"></span>
            {#if editingId === child.id}
              <form class="flex flex-1 items-center gap-1 pr-1" onsubmit={(e) => { e.preventDefault(); submitRename(child.id) }}>
                <input
                  autofocus
                  bind:value={editName}
                  class="min-w-0 flex-1 rounded border border-blue-400 px-1.5 py-0.5 text-xs outline-none"
                  onblur={() => submitRename(child.id)}
                />
              </form>
            {:else}
              <button
                class="flex min-w-0 flex-1 items-center gap-1.5 py-1.5 pr-1 text-left {activeFolderId === child.id ? 'text-blue-700 font-medium' : 'text-gray-600 hover:text-gray-900'}"
                onclick={() => onselect(child.id)}
              >
                <svg class="h-3.5 w-3.5 shrink-0 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V7z" />
                </svg>
                <span class="min-w-0 flex-1 truncate text-xs">{child.name}</span>
                {#if child.asset_count > 0}
                  <span class="shrink-0 text-xs text-gray-400">{child.asset_count}</span>
                {/if}
              </button>
            {/if}

            {#if $authStore.role !== 'viewer'}
              <button
                class="absolute right-1 top-1/2 -translate-y-1/2 rounded p-0.5 text-gray-400 opacity-0 hover:bg-gray-200 hover:text-gray-700 group-hover:opacity-100"
                onclick={(e) => { e.stopPropagation(); contextMenuId = contextMenuId === child.id ? null : child.id }}
                aria-label="Folder menu"
              >
                <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 5v.01M12 12v.01M12 19v.01M12 6a1 1 0 110-2 1 1 0 010 2zm0 7a1 1 0 110-2 1 1 0 010 2zm0 7a1 1 0 110-2 1 1 0 010 2z" />
                </svg>
              </button>
            {/if}

            {#if contextMenuId === child.id}
              <!-- svelte-ignore a11y_no_static_element_interactions -->
              <div
                class="absolute right-0 top-full z-30 mt-0.5 w-40 rounded-lg border border-gray-200 bg-white py-1 shadow-lg"
                onmouseleave={() => { contextMenuId = null }}
              >
                <button
                  class="flex w-full items-center gap-2 px-3 py-1.5 text-xs text-gray-700 hover:bg-gray-50"
                  onclick={() => { editingId = child.id; editName = child.name; contextMenuId = null }}
                >Rename</button>
                {#if $authStore.role === 'owner'}
                  <button
                    class="flex w-full items-center gap-2 px-3 py-1.5 text-xs text-red-600 hover:bg-red-50"
                    onclick={() => deleteFolder(child.id)}
                  >Delete</button>
                {/if}
              </div>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>
{/each}

{#if error}
  <p class="mt-1 px-2 text-xs text-red-500">{error}</p>
{/if}
