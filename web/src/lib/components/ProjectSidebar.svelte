<script lang="ts">
  import { projectApi, folderApi, type Project, type Folder } from '$lib/api/client'
  import { authStore } from '$lib/stores/auth'
  import FolderTree from './FolderTree.svelte'

  interface Props {
    projects: Project[]
    activeProjectId: string | null
    folders: Record<string, Folder[]>
    activeFolderId: string | null
    selectedAssetIds: Set<string>
    onselect: (id: string | null) => void
    onchange: () => void
    onfolderschange: (projectId: string) => void
    onfolderselect: (projectId: string, folderId: string | null) => void
    onassetsDropped: (assetIds: string[], folderId: string | null, projectId: string) => void
  }

  let { projects, activeProjectId, folders, activeFolderId, selectedAssetIds, onselect, onchange, onfolderschange, onfolderselect, onassetsDropped }: Props = $props()

  let dropTargetProjectId = $state<string | null>(null)
  let creatingRootFolderForProject = $state<string | null>(null)
  let newRootFolderName = $state('')

  async function submitCreateRootFolder(projectId: string) {
    const name = newRootFolderName.trim()
    if (!name) { creatingRootFolderForProject = null; return }
    try {
      await folderApi.create(projectId, { name })
      newRootFolderName = ''
      creatingRootFolderForProject = null
      onfolderschange(projectId)
    } catch {
      error = 'Could not create folder'
    }
  }

  let creating = $state(false)
  let newName = $state('')
  let newColor = $state('#6366f1')
  let editingId = $state<string | null>(null)
  let editName = $state('')
  let menuOpenId = $state<string | null>(null)
  let error = $state('')

  const COLORS = ['#6366f1', '#f59e0b', '#10b981', '#ef4444', '#3b82f6', '#8b5cf6', '#ec4899', '#14b8a6']

  async function submitCreate() {
    const name = newName.trim()
    if (!name) return
    try {
      await projectApi.create({ name, color: newColor })
      newName = ''
      newColor = '#6366f1'
      creating = false
      onchange()
    } catch {
      error = 'Could not create project'
    }
  }

  async function submitEdit(id: string) {
    const name = editName.trim()
    if (!name) return
    try {
      await projectApi.update(id, { name })
      editingId = null
      onchange()
    } catch {
      error = 'Could not rename project'
    }
  }

  async function deleteProject(id: string) {
    menuOpenId = null
    try {
      await projectApi.delete(id)
      if (activeProjectId === id) onselect(null)
      onchange()
    } catch {
      error = 'Could not delete project'
    }
  }

  function startEdit(p: Project) {
    editingId = p.id
    editName = p.name
    menuOpenId = null
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      creating = false
      editingId = null
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="flex flex-col">
  <!-- All assets link -->
  <button
    class="flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition-colors {activeProjectId === null ? 'bg-blue-50 text-blue-700' : 'text-gray-600 hover:bg-gray-100'}"
    onclick={() => onselect(null)}
  >
    <svg class="h-4 w-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
        d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
    </svg>
    All assets
  </button>

  {#if projects.length > 0}
    <p class="mt-3 mb-1 px-3 text-xs font-semibold uppercase tracking-wide text-gray-400">Projects</p>
  {/if}

  {#each projects as project (project.id)}
    <div class="group relative">
      {#if editingId === project.id}
        <form
          class="flex items-center gap-1 rounded-md px-2 py-1"
          onsubmit={(e) => { e.preventDefault(); submitEdit(project.id) }}
        >
          <input
            autofocus
            bind:value={editName}
            class="min-w-0 flex-1 rounded border border-blue-400 px-1.5 py-0.5 text-sm outline-none"
            onblur={() => submitEdit(project.id)}
          />
        </form>
      {:else}
        <button
          class="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm transition-colors {activeProjectId === project.id ? 'bg-blue-50 text-blue-700' : 'text-gray-600 hover:bg-gray-100'} {dropTargetProjectId === project.id ? 'bg-green-50 ring-1 ring-green-400' : ''}"
          onclick={() => onselect(project.id)}
          ondragover={(e) => { e.preventDefault(); dropTargetProjectId = project.id }}
          ondragleave={() => { dropTargetProjectId = null }}
          ondrop={(e) => {
            e.preventDefault()
            dropTargetProjectId = null
            const assetId = e.dataTransfer?.getData('text/plain')
            if (!assetId) return
            let assetIds: string[]
            if (selectedAssetIds.has(assetId) && selectedAssetIds.size > 1) {
              assetIds = [...selectedAssetIds]
            } else {
              assetIds = [assetId]
            }
            onassetsDropped(assetIds, null, project.id)
          }}
        >
          <span
            class="h-2.5 w-2.5 shrink-0 rounded-full"
            style="background-color: {project.color.Valid ? project.color.String : '#9ca3af'}"
          ></span>
          <span class="min-w-0 flex-1 truncate text-left">{project.name}</span>
          <span class="ml-auto shrink-0 text-xs text-gray-400">{project.asset_count}</span>
        </button>

        <!-- Context menu trigger -->
        {#if $authStore.role !== 'viewer'}
          <button
            class="absolute right-2 top-1/2 -translate-y-1/2 rounded p-0.5 text-gray-400 opacity-0 hover:bg-gray-200 hover:text-gray-700 group-hover:opacity-100"
            onclick={(e) => { e.stopPropagation(); menuOpenId = menuOpenId === project.id ? null : project.id }}
            aria-label="Project menu"
          >
            <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 5v.01M12 12v.01M12 19v.01M12 6a1 1 0 110-2 1 1 0 010 2zm0 7a1 1 0 110-2 1 1 0 010 2zm0 7a1 1 0 110-2 1 1 0 010 2z" />
            </svg>
          </button>
        {/if}
      {/if}

      {#if menuOpenId === project.id}
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div
          class="absolute right-2 top-full z-20 mt-0.5 w-36 rounded-lg border border-gray-200 bg-white py-1 shadow-lg"
          onmouseleave={() => { menuOpenId = null }}
        >
          <button
            class="flex w-full items-center gap-2 px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-50"
            onclick={() => startEdit(project)}
          >Rename</button>
          {#if $authStore.role === 'owner'}
            <button
              class="flex w-full items-center gap-2 px-3 py-1.5 text-sm text-red-600 hover:bg-red-50"
              onclick={() => deleteProject(project.id)}
            >Delete</button>
          {/if}
        </div>
      {/if}

      <!-- Folder tree for active project -->
      {#if activeProjectId === project.id && folders[project.id]}
        <div class="pl-2 pt-0.5">
          <FolderTree
            folders={folders[project.id]}
            {activeFolderId}
            projectId={project.id}
            {selectedAssetIds}
            onselect={(folderId) => onfolderselect(project.id, folderId)}
            onchange={() => onfolderschange(project.id)}
            onassetsDropped={(assetIds, folderId) => onassetsDropped(assetIds, folderId, project.id)}
          />
          {#if $authStore.role !== 'viewer'}
            {#if creatingRootFolderForProject === project.id}
              <form
                class="mt-1 flex items-center gap-1 rounded-md border border-gray-200 bg-gray-50 px-2 py-1"
                onsubmit={(e) => { e.preventDefault(); submitCreateRootFolder(project.id) }}
              >
                <input
                  autofocus
                  bind:value={newRootFolderName}
                  placeholder="Folder name"
                  class="min-w-0 flex-1 bg-transparent text-xs outline-none"
                  onblur={() => { if (!newRootFolderName.trim()) creatingRootFolderForProject = null }}
                />
                <button type="submit" class="shrink-0 text-xs text-blue-600 hover:text-blue-800">Add</button>
              </form>
            {:else}
              <button
                class="mt-1 flex w-full items-center gap-1 rounded px-2 py-1 text-xs text-gray-400 hover:bg-gray-100 hover:text-gray-600"
                onclick={() => { creatingRootFolderForProject = project.id; newRootFolderName = '' }}
              >
                <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
                </svg>
                New folder
              </button>
            {/if}
          {/if}
        </div>
      {/if}
    </div>
  {/each}

  <!-- New project -->
  {#if $authStore.role !== 'viewer'}
    {#if creating}
      <form
        class="mt-1 flex flex-col gap-2 rounded-lg border border-gray-200 bg-gray-50 p-2"
        onsubmit={(e) => { e.preventDefault(); submitCreate() }}
      >
        <input
          autofocus
          bind:value={newName}
          placeholder="Project name"
          class="rounded border border-gray-300 px-2 py-1 text-sm focus:border-blue-500 focus:outline-none"
        />
        <div class="flex flex-wrap gap-1">
          {#each COLORS as c}
            <button
              type="button"
              aria-label="Select color {c}"
              class="h-5 w-5 rounded-full transition-transform hover:scale-110 {newColor === c ? 'ring-2 ring-offset-1 ring-gray-600' : ''}"
              style="background-color: {c}"
              onclick={() => { newColor = c }}
            ></button>
          {/each}
        </div>
        <div class="flex gap-1">
          <button type="submit" class="flex-1 rounded bg-blue-600 py-1 text-xs font-medium text-white hover:bg-blue-700">Create</button>
          <button type="button" class="flex-1 rounded bg-gray-200 py-1 text-xs font-medium text-gray-600 hover:bg-gray-300" onclick={() => { creating = false }}>Cancel</button>
        </div>
      </form>
    {:else}
      <button
        class="mt-1 flex items-center gap-2 rounded-md px-3 py-2 text-sm text-gray-400 hover:bg-gray-100 hover:text-gray-600"
        onclick={() => { creating = true }}
      >
        <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
        </svg>
        New project
      </button>
    {/if}
  {/if}

  {#if error}
    <p class="mt-1 px-3 text-xs text-red-500">{error}</p>
  {/if}
</div>
