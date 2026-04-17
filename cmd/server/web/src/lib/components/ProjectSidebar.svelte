<script lang="ts">
  import { authStore } from '$lib/stores/auth.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import { foldersStore } from '$lib/stores/folders.svelte'
  import { navigationStore } from '$lib/stores/navigation.svelte'
  import { ingressStore } from '$lib/stores/ingress.svelte'
  import { getProjectColor } from '$lib/stores/shared'
  import FolderTree from './FolderTree.svelte'
  import { Box, EllipsisVertical, Plus } from '@lucide/svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import InlineEditForm from '$lib/components/ui/InlineEditForm.svelte'
  import ContextMenu from '$lib/components/ui/ContextMenu.svelte'
  import Feedback from './ui/Feedback.svelte'
  import { assetApi } from '$lib/api'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    selectedAssetIds: Set<string>
    creating: boolean
    onCreatingChange: (v: boolean) => void
    onSelect: (id: string | null) => void
    onFolderSelect: (projectId: string, folderId: string | null) => void
    onAssetsFolderDropped: (assetIds: string[], folderId: string | null, projectId: string) => void
    onAssetsProjectDropped: (assetIds: string[], projectId: string) => void
  }

  let { selectedAssetIds, creating, onCreatingChange, onSelect, onFolderSelect, onAssetsFolderDropped, onAssetsProjectDropped }: Props = $props()

  const emailIngestToken = $derived(
    ingressStore.sources.find(s => s.type === 'email_api' && s.enabled)?.public_token ?? null
  )

  let dropTargetProjectId = $state<string | null>(null)
  let creatingFolderForProject = $state<string | null>(null)
  let newFolderName = $state('')
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
      onCreatingChange(false)
    } catch {
      error = m.project_create_failed()
    }
  }

  async function submitEdit(id: string, name: string) {
    if (!name) return
    try {
      await projectsStore.update(id, { name })
      editingId = null
    } catch {
      error = m.project_rename_failed()
    }
  }

  async function deleteProject(id: string) {
    menuOpenId = null
    try {
      await projectsStore.delete(id)
    } catch {
      error = m.delete_project_failed()
    }
  }

  async function submitCreateFolder(projectId: string) {
    const name = newFolderName.trim()
    if (!name) { creatingFolderForProject = null; return }
    try {
      await foldersStore.create(projectId, { name })
      newFolderName = ''
      creatingFolderForProject = null
    } catch {
      error = m.folder_create_failed()
    }
  }

  function startEdit(id: string, name: string) {
    editingId = id
    editName = name
    menuOpenId = null
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      onCreatingChange(false)
      editingId = null
    }
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
          class="flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-md transition-colors font-medium
            {navigationStore.activeProjectId === project.id ? 'bg-gray-100 focus-visible:ring-gray-300 dark:text-gray-300 dark:bg-gray-700' : 'hover:bg-gray-100 focus-visible:ring-gray-300 dark:text-gray-300 dark:hover:bg-gray-700'}
            {dropTargetProjectId === project.id ? 'bg-green-50 ring-1 ring-green-400 dark:bg-green-900/30' : ''}"
          onclick={() => onSelect(project.id)}
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
            onAssetsProjectDropped(assetIds, project.id)
          }}
        >
          {#if project.cover_asset_id}
            <img
              src={assetApi.thumbUrl(project.cover_asset_id)}
              class="h-4 w-4 rounded object-cover"
              alt="Project cover"
            />
          {:else}
            <Box
              class="h-4 w-4 shrink-0 text-gray-400"
              style="color: {getProjectColor(project)}"
            />
          {/if}
          <span class="min-w-0 flex-1 truncate text-left">{project.name}</span>
          <span class="ml-auto shrink-0 text-sm text-gray-400 p-1">{project.asset_count || ''}</span>
        </button>

        {#if authStore.role !== 'viewer'}
          <button
            class="absolute right-7 top-2.5 rounded p-0.5 text-gray-300 opacity-0 hover:bg-gray-200 hover:text-gray-600 group-hover:opacity-100 dark:text-gray-600 dark:hover:bg-gray-700 dark:hover:text-gray-400"
            onclick={(e) => { e.stopPropagation(); menuOpenId = menuOpenId === project.id ? null : project.id }}
            aria-label="Project menu"
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
              ...(authStore.role === 'owner' ? [{ label: m.delete(), onclick: () => deleteProject(project.id), danger: true }] : [])
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
            ingestToken={emailIngestToken}
            onselect={(folderId) => onFolderSelect(project.id, folderId)}
            onassetsDropped={(assetIds, folderId) => onAssetsFolderDropped(assetIds, folderId, project.id)}
          />
          {#if authStore.role !== 'viewer'}
            {#if creatingFolderForProject === project.id}
              <form
                class="mt-1 flex items-center gap-1 rounded-md border border-gray-200 bg-gray-50 px-2 py-1 dark:border-gray-700 dark:bg-gray-800"
                onsubmit={(e) => { e.preventDefault(); submitCreateFolder(project.id) }}
              >
                <input
                  bind:value={newFolderName}
                  placeholder="Folder name"
                  class="min-w-0 flex-1 bg-transparent text-sm text-gray-900 outline-none dark:text-gray-100"
                  onblur={() => { if (!newFolderName.trim()) creatingFolderForProject = null }}
                />
                <button type="submit" class="shrink-0 text-sm text-indigo-600 hover:text-indigo-800">{m.add()}</button>
              </form>
            {:else}
              <Button variant="ghost" size="sm" class="mt-0.5 w-full justify-start" onclick={() => { creatingFolderForProject = project.id; newFolderName = '' }}>
                {#snippet icon()}<Plus class="h-3 w-3" />{/snippet}
                {m.new_folder()}
              </Button>
            {/if}
          {/if}
        </div>
      {/if}
    </div>
  {/each}

  <!-- New project -->
  {#if authStore.role !== 'viewer'}
    {#if creating}
      <form
        class="mt-1 flex flex-col gap-2 rounded-lg border border-gray-200 bg-gray-50 p-2 dark:border-gray-700 dark:bg-gray-800"
        onsubmit={(e) => { e.preventDefault(); submitCreate() }}
      >
        <input
          bind:value={newName}
          placeholder={m.project_name()}
          class="rounded border border-gray-300 bg-white px-2 py-1 text-md text-gray-900 focus:border-indigo-400 focus:outline-none dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100"
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
          <Button type="button" variant="secondary" size="sm" class="flex-1" onclick={() => onCreatingChange(false)}>{m.cancel()}</Button>
        </div>
      </form>
    {:else}
      <Button variant="ghost" size="sm" class="mt-1 w-full justify-start" onclick={() => onCreatingChange(true)}>
        {#snippet icon()}<Plus class="h-3.5 w-3.5" />{/snippet}
        {m.new_project()}
      </Button>
    {/if}
  {/if}

  <Feedback {error} />
</div>
