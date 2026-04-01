<script lang="ts">
  import { authStore } from '$lib/stores/auth.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import { foldersStore } from '$lib/stores/folders.svelte'
  import { navigationStore } from '$lib/stores/navigation.svelte'
  import FolderTree from './FolderTree.svelte'
  import { EllipsisVertical, Plus } from '@lucide/svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import InlineEditForm from '$lib/components/ui/InlineEditForm.svelte'
  import ContextMenu from '$lib/components/ui/ContextMenu.svelte'

  interface Props {
    selectedAssetIds: Set<string>
    creating: boolean
    oncreatingchange: (v: boolean) => void
    onselect: (id: string | null) => void
    onfolderselect: (projectId: string, folderId: string | null) => void
    onassetsDropped: (assetIds: string[], folderId: string | null, projectId: string) => void
  }

  let { selectedAssetIds, creating, oncreatingchange, onselect, onfolderselect, onassetsDropped }: Props = $props()

  let dropTargetProjectId = $state<string | null>(null)
  let creatingRootFolderForProject = $state<string | null>(null)
  let newRootFolderName = $state('')
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
      await projectsStore.create({ name, color: newColor })
      newName = ''
      newColor = '#6366f1'
      oncreatingchange(false)
    } catch {
      error = 'Could not create folder'
    }
  }

  async function submitEdit(id: string, name: string) {
    if (!name) return
    try {
      await projectsStore.update(id, { name })
      editingId = null
    } catch {
      error = 'Could not rename folder'
    }
  }

  async function deleteProject(id: string) {
    menuOpenId = null
    try {
      await projectsStore.delete(id)
    } catch {
      error = 'Could not delete folder'
    }
  }

  async function submitCreateRootFolder(projectId: string) {
    const name = newRootFolderName.trim()
    if (!name) { creatingRootFolderForProject = null; return }
    try {
      await foldersStore.create(projectId, { name })
      newRootFolderName = ''
      creatingRootFolderForProject = null
    } catch {
      error = 'Could not create folder'
    }
  }

  function startEdit(id: string, name: string) {
    editingId = id
    editName = name
    menuOpenId = null
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      oncreatingchange(false)
      editingId = null
    }
  }

  function projectColor(color: { Valid: boolean; String: string }): string {
    return color.Valid ? color.String : '#9ca3af'
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="flex flex-col gap-0.5">
  {#each projectsStore.projects as project (project.id)}
    <div class="group relative">
      {#if editingId === project.id}
        <div class="px-2 py-1">
          <InlineEditForm
            bind:value={editName}
            onsubmit={(v) => submitEdit(project.id, v)}
            oncancel={() => { editingId = null }}
            size="sm"
          />
        </div>
      {:else}
        <button
          class="flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-sm transition-colors
            {navigationStore.activeProjectId === project.id ? 'bg-gray-100 font-medium text-gray-900 dark:bg-gray-800 dark:text-gray-50' : 'text-gray-600 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800'}
            {dropTargetProjectId === project.id ? 'bg-green-50 ring-1 ring-green-400 dark:bg-green-900/30' : ''}"
          onclick={() => onselect(project.id)}
          ondragover={(e) => { e.preventDefault(); dropTargetProjectId = project.id }}
          ondragleave={() => { dropTargetProjectId = null }}
          ondrop={(e) => {
            e.preventDefault()
            dropTargetProjectId = null
            const assetId = e.dataTransfer?.getData('text/plain')
            if (!assetId) return
            const assetIds = selectedAssetIds.has(assetId) && selectedAssetIds.size > 1
              ? [...selectedAssetIds]
              : [assetId]
            onassetsDropped(assetIds, null, project.id)
          }}
        >
          <svg
            class="h-4 w-4 shrink-0"
            viewBox="0 0 20 20"
            fill="currentColor"
            style="color: {projectColor(project.color)}"
          >
            <path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z" />
          </svg>
          <span class="min-w-0 flex-1 truncate text-left">{project.name}</span>
          <span class="ml-auto shrink-0 text-xs text-gray-400">{project.asset_count || ''}</span>
        </button>

        {#if authStore.role !== 'viewer'}
          <button
            class="absolute right-7 top-1/2 -translate-y-1/2 rounded p-0.5 text-gray-300 opacity-0 hover:bg-gray-200 hover:text-gray-600 group-hover:opacity-100 dark:text-gray-600 dark:hover:bg-gray-700 dark:hover:text-gray-400"
            onclick={(e) => { e.stopPropagation(); menuOpenId = menuOpenId === project.id ? null : project.id }}
            aria-label="Folder menu"
          >
            <EllipsisVertical class="h-3.5 w-3.5" />
          </button>
        {/if}
      {/if}

      {#if menuOpenId === project.id}
        <div class="absolute right-2 top-full z-20 mt-0.5">
          <ContextMenu
            items={[
              { label: 'Rename', onclick: () => startEdit(project.id, project.name) },
              ...(authStore.role === 'owner' ? [{ label: 'Delete', onclick: () => deleteProject(project.id), danger: true }] : [])
            ]}
            onclose={() => { menuOpenId = null }}
          />
        </div>
      {/if}

      {#if navigationStore.activeProjectId === project.id && foldersStore.foldersByProject[project.id]}
        <div class="pl-5 pt-0.5">
          <FolderTree
            folders={foldersStore.foldersByProject[project.id]}
            activeFolderId={navigationStore.activeFolderId}
            projectId={project.id}
            {selectedAssetIds}
            onselect={(folderId) => onfolderselect(project.id, folderId)}
            onassetsDropped={(assetIds, folderId) => onassetsDropped(assetIds, folderId, project.id)}
          />
          {#if authStore.role !== 'viewer'}
            {#if creatingRootFolderForProject === project.id}
              <form
                class="mt-1 flex items-center gap-1 rounded-md border border-gray-200 bg-gray-50 px-2 py-1 dark:border-gray-700 dark:bg-gray-800"
                onsubmit={(e) => { e.preventDefault(); submitCreateRootFolder(project.id) }}
              >
                <input
                  bind:value={newRootFolderName}
                  placeholder="Folder name"
                  class="min-w-0 flex-1 bg-transparent text-xs text-gray-900 outline-none dark:text-gray-100"
                  onblur={() => { if (!newRootFolderName.trim()) creatingRootFolderForProject = null }}
                />
                <button type="submit" class="shrink-0 text-xs text-indigo-600 hover:text-indigo-800">Add</button>
              </form>
            {:else}
              <Button variant="ghost" size="sm" class="mt-0.5 w-full justify-start" onclick={() => { creatingRootFolderForProject = project.id; newRootFolderName = '' }}>
                {#snippet icon()}<Plus class="h-3 w-3" />{/snippet}
                New subfolder
              </Button>
            {/if}
          {/if}
        </div>
      {/if}
    </div>
  {/each}

  <!-- New folder -->
  {#if authStore.role !== 'viewer'}
    {#if creating}
      <form
        class="mt-1 flex flex-col gap-2 rounded-lg border border-gray-200 bg-gray-50 p-2 dark:border-gray-700 dark:bg-gray-800"
        onsubmit={(e) => { e.preventDefault(); submitCreate() }}
      >
        <input
          bind:value={newName}
          placeholder="Folder name"
          class="rounded border border-gray-300 bg-white px-2 py-1 text-sm text-gray-900 focus:border-indigo-400 focus:outline-none dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
        />
        <div class="flex flex-wrap gap-1">
          {#each COLORS as c}
            <button
              type="button"
              aria-label="Select color {c}"
              class="h-4 w-4 rounded-full transition-transform hover:scale-110 {newColor === c ? 'ring-2 ring-offset-1 ring-gray-500' : ''}"
              style="background-color: {c}"
              onclick={() => { newColor = c }}
            ></button>
          {/each}
        </div>
        <div class="flex gap-1">
          <Button type="submit" size="sm" class="flex-1">Create</Button>
          <Button type="button" variant="secondary" size="sm" class="flex-1" onclick={() => oncreatingchange(false)}>Cancel</Button>
        </div>
      </form>
    {:else}
      <Button variant="ghost" size="sm" class="mt-1 w-full justify-start" onclick={() => oncreatingchange(true)}>
        {#snippet icon()}<Plus class="h-3.5 w-3.5" />{/snippet}
        New folder
      </Button>
    {/if}
  {/if}

  {#if error}
    <p class="mt-1 px-2 text-xs text-red-500">{error}</p>
  {/if}
</div>
