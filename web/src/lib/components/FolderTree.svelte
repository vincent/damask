<script lang="ts">
  import { authStore } from '$lib/stores/auth.svelte'
  import { foldersStore } from '$lib/stores/folders.svelte'
  import type { Folder } from '$lib/api'
  import { ChevronRight, EllipsisVertical, FolderClosed, FolderOpen } from '@lucide/svelte'
  import InlineEditForm from '$lib/components/ui/InlineEditForm.svelte'
  import ContextMenu from '$lib/components/ui/ContextMenu.svelte'
  import Button from '$lib/components/ui/Button.svelte'

  interface Props {
    folders: Folder[]
    activeFolderId: string | null
    projectId: string
    selectedAssetIds: Set<string>
    onselect: (folderId: string | null) => void
    onassetsDropped: (assetIds: string[], folderId: string | null) => void
  }

  let { folders, activeFolderId, projectId, selectedAssetIds, onselect, onassetsDropped }: Props = $props()

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
    if (next.has(id)) { next.delete(id) } else { next.add(id) }
    openFolderIds = next
    saveOpenIds(next)
  }

  async function submitRename(id: string, name: string) {
    if (!name) { editingId = null; return }
    try {
      await foldersStore.rename(id, projectId, name)
      editingId = null
    } catch {
      error = 'Could not rename folder'
    }
  }

  async function deleteFolder(id: string) {
    contextMenuId = null
    try {
      await foldersStore.delete(id, projectId)
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
      await foldersStore.create(projectId, data)
      newFolderName = ''
      creatingUnder = null
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
    const assetIds = selectedAssetIds.has(assetId) && selectedAssetIds.size > 1
      ? [...selectedAssetIds]
      : [assetId]
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
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
      class="group relative flex items-center rounded-md transition-colors
        {activeFolderId === folder.id ? 'bg-blue-50 dark:bg-blue-900/30' : ''}
        {dropTargetId === folder.id ? 'bg-blue-100 ring-1 ring-blue-400 dark:bg-blue-900/40' : ''}
      "
      ondragover={(e) => handleDragOver(e, folder.id)}
      ondragleave={handleDragLeave}
      ondrop={(e) => handleDrop(e, folder.id)}
    >
      {#if folder.children && folder.children.length > 0}
        <button
          class="shrink-0 p-1 text-gray-400 hover:text-gray-600"
          onclick={(e) => toggleOpen(folder.id, e)}
          aria-label="Toggle folder"
        >
          <ChevronRight class="h-3 w-3 transition-transform {openFolderIds.has(folder.id) ? 'rotate-90' : ''}" />
        </button>
      {:else}
        <span class="w-5 shrink-0"></span>
      {/if}

      {#if editingId === folder.id}
        <div class="flex-1 pr-1">
          <InlineEditForm
            bind:value={editName}
            onsubmit={(v) => submitRename(folder.id, v)}
            oncancel={() => { editingId = null }}
            size="sm"
          />
        </div>
      {:else}
        <button
          class="flex min-w-0 flex-1 items-center gap-1.5 py-1.5 pr-1 text-left text-sm {activeFolderId === folder.id ? 'text-blue-700 font-medium dark:text-blue-400' : 'text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-100'}"
          onclick={() => onselect(folder.id)}
        >
          <span class="min-w-0 flex-1 truncate text-xs">{folder.name}</span>
          {#if folder.asset_count > 0}
            <span class="shrink-0 text-xs text-gray-400">{folder.asset_count}</span>
          {/if}
        </button>
      {/if}

      {#if authStore.role !== 'viewer'}
        <button
          class="absolute right-1 top-1/2 -translate-y-1/2 rounded p-0.5 text-gray-400 opacity-0 hover:bg-gray-200 hover:text-gray-700 group-hover:opacity-100 dark:text-gray-500 dark:hover:bg-gray-700 dark:hover:text-gray-300"
          onclick={(e) => { e.stopPropagation(); contextMenuId = contextMenuId === folder.id ? null : folder.id }}
          aria-label="Folder menu"
        >
          <EllipsisVertical class="h-3.5 w-3.5" />
        </button>
      {/if}

      {#if contextMenuId === folder.id}
        <div class="absolute right-0 top-full z-30 mt-0.5">
          <ContextMenu
            items={[
              { label: 'Rename', onclick: () => { editingId = folder.id; editName = folder.name; contextMenuId = null } },
              ...(!folder.parent_id.Valid ? [{ label: 'Add subfolder', onclick: () => { creatingUnder = folder.id; contextMenuId = null; newFolderName = '' } }] : []),
              ...(authStore.role === 'owner' ? [{ label: 'Delete', onclick: () => deleteFolder(folder.id), danger: true }] : [])
            ]}
            onclose={() => { contextMenuId = null }}
          />
        </div>
      {/if}
    </div>

    {#if creatingUnder === folder.id}
      <div class="ml-5">
        <form
          class="flex items-center gap-1 rounded-md border border-gray-200 bg-gray-50 px-2 py-1 dark:border-gray-700 dark:bg-gray-800"
          onsubmit={(e) => { e.preventDefault(); submitCreate(folder.id) }}
        >
          <input
            bind:value={newFolderName}
            placeholder="Folder name"
            class="min-w-0 flex-1 bg-transparent text-xs text-gray-900 outline-none dark:text-gray-100"
            onblur={() => { if (!newFolderName.trim()) creatingUnder = null }}
          />
          <Button type="submit" variant="ghost" size="sm" class="shrink-0">Add</Button>
        </form>
      </div>
    {/if}

    {#if openFolderIds.has(folder.id) && folder.children && folder.children.length > 0}
      <div class="ml-4 border-l border-gray-100 pl-1">
        {#each folder.children as child (child.id)}
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div
            class="group relative flex items-center rounded-md transition-colors
              {activeFolderId === child.id ? 'bg-blue-50 dark:bg-blue-900/30' : ''}
              {dropTargetId === child.id ? 'bg-blue-100 ring-1 ring-blue-400 dark:bg-blue-900/40' : ''}
            "
            ondragover={(e) => handleDragOver(e, child.id)}
            ondragleave={handleDragLeave}
            ondrop={(e) => handleDrop(e, child.id)}
          >
            <span class="w-5 shrink-0"></span>
            {#if editingId === child.id}
              <div class="flex-1 pr-1">
                <InlineEditForm
                  bind:value={editName}
                  onsubmit={(v) => submitRename(child.id, v)}
                  oncancel={() => { editingId = null }}
                  size="sm"
                />
              </div>
            {:else}
              <button
                class="flex min-w-0 flex-1 items-center gap-1.5 py-1.5 pr-1 text-left {activeFolderId === child.id ? 'text-blue-700 font-medium dark:text-blue-400' : 'text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-100'}"
                onclick={() => onselect(child.id)}
              >
                <span class="min-w-0 flex-1 truncate text-xs">{child.name}</span>
                {#if child.asset_count > 0}
                  <span class="shrink-0 text-xs text-gray-400">{child.asset_count}</span>
                {/if}
              </button>
            {/if}

            {#if authStore.role !== 'viewer'}
              <button
                class="absolute right-1 top-1/2 -translate-y-1/2 rounded p-0.5 text-gray-400 opacity-0 hover:bg-gray-200 hover:text-gray-700 group-hover:opacity-100 dark:text-gray-500 dark:hover:bg-gray-700 dark:hover:text-gray-300"
                onclick={(e) => { e.stopPropagation(); contextMenuId = contextMenuId === child.id ? null : child.id }}
                aria-label="Folder menu"
              >
                <EllipsisVertical class="h-3.5 w-3.5" />
              </button>
            {/if}

            {#if contextMenuId === child.id}
              <div class="absolute right-0 top-full z-30 mt-0.5">
                <ContextMenu
                  items={[
                    { label: 'Rename', onclick: () => { editingId = child.id; editName = child.name; contextMenuId = null } },
                    ...(authStore.role === 'owner' ? [{ label: 'Delete', onclick: () => deleteFolder(child.id), danger: true }] : [])
                  ]}
                  onclose={() => { contextMenuId = null }}
                />
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
