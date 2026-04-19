<script lang="ts">
  import { stackStore } from '$lib/stores/stack.svelte'
  import { stackApi, collectionApi, shareApi, jobApi, projectApi } from '$lib/api'
  import type { Project } from '$lib/api'
  import { X, Layers, Trash2, Tag, Download, FolderInput, Save, Loader, Share2, Copy, Check } from '@lucide/svelte'
  import { fly } from 'svelte/transition'
  import { toastStore } from '$lib/stores/toast.svelte'

  let labelInput = $state(stackStore.label ?? '')
  $effect(() => { labelInput = stackStore.label ?? '' })
  let downloading = $state(false)

  // Save as collection
  let savingCollection = $state(false)
  let collectionName = $state('')
  let showCollectionForm = $state(false)

  // Share stack
  let sharing = $state(false)
  let shareUrl = $state<string | null>(null)
  let copied = $state(false)
  let showShareNameForm = $state(false)
  let shareNameInput = $state('')

  // Move to project
  let movingToProject = $state(false)
  let showProjectPicker = $state(false)
  let projects = $state<Project[]>([])
  let projectSearch = $state('')
  let loadingProjects = $state(false)

  let filteredProjects = $derived(
    projects.filter(p => p.name.toLowerCase().includes(projectSearch.toLowerCase()))
  )

  // Merge
  let merging = $state(false)
  let mergeType = $state<'gif' | 'pdf'>('gif')
  let mergeStatus = $state<string | null>(null)
  let showMergeForm = $state(false)
  let pollAborted = $state(false)

  $effect(() => {
    return () => { pollAborted = true }
  })

  function handleLabelBlur() {
    stackStore.setLabel(labelInput.trim())
  }

  async function handleDownloadZip() {
    downloading = true
    try {
      await stackApi.exportZip(stackStore.ids, stackStore.label ?? 'stack-export')
    } catch (e: any) {
      toastStore.show(e?.message ?? 'Download failed', 'error')
    } finally {
      downloading = false
    }
  }

  async function handleSaveCollection() {
    if (!collectionName.trim()) return
    savingCollection = true
    try {
      await collectionApi.create(collectionName.trim(), '', stackStore.ids)
      toastStore.show('Collection saved', 'success')
      showCollectionForm = false
      collectionName = ''
    } catch (e: any) {
      toastStore.show(e?.message ?? 'Save failed', 'error')
    } finally {
      savingCollection = false
    }
  }

  // Share: needs a saved collection first. If stack has a label, use it; otherwise prompt.
  function handleShareClick() {
    shareUrl = null
    copied = false
    if (stackStore.label?.trim()) {
      doShare(stackStore.label.trim())
    } else {
      shareNameInput = ''
      showShareNameForm = true
    }
  }

  async function doShare(name: string) {
    sharing = true
    showShareNameForm = false
    try {
      const col = await collectionApi.create(name, '', stackStore.ids)
      stackStore.setLabel(name)
      const share = await shareApi.create({
        target_type: 'collection',
        target_id: col.id,
        label: name,
        allow_download: true,
      })
      shareUrl = share.public_url
    } catch (e: any) {
      toastStore.show(e?.message ?? 'Share failed', 'error')
    } finally {
      sharing = false
    }
  }

  async function handleCopyLink() {
    if (!shareUrl) return
    await navigator.clipboard.writeText(shareUrl)
    copied = true
    setTimeout(() => { copied = false }, 2000)
  }

  async function handleOpenProjectPicker() {
    showProjectPicker = true
    loadingProjects = true
    projectSearch = ''
    try {
      projects = await projectApi.list()
    } catch {
      projects = []
    } finally {
      loadingProjects = false
    }
  }

  async function handleMoveToProject(project: Project) {
    showProjectPicker = false
    movingToProject = true
    try {
      await fetch(`/api/v1/assets/bulk/project`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ asset_ids: stackStore.ids, project_id: project.id }),
      })
      toastStore.show(`${stackStore.count} asset${stackStore.count === 1 ? '' : 's'} moved to ${project.name}`, 'success')
    } catch (e: any) {
      toastStore.show(e?.message ?? 'Move failed', 'error')
    } finally {
      movingToProject = false
    }
  }

  async function handleMerge() {
    merging = true
    mergeStatus = 'pending'
    try {
      const jobId = await stackApi.merge(stackStore.ids, mergeType, stackStore.label ?? 'stack-merge')
      showMergeForm = false
      pollMergeJob(jobId)
    } catch (e: any) {
      toastStore.show(e?.message ?? 'Merge failed', 'error')
      merging = false
      mergeStatus = null
    }
  }

  async function pollMergeJob(jobId: string) {
    let delay = 2000
    while (!pollAborted) {
      await new Promise(r => setTimeout(r, delay))
      if (pollAborted) break
      try {
        const job = await jobApi.get(jobId)
        mergeStatus = job.status
        if (job.status === 'done') {
          merging = false
          toastStore.show('Merge complete — asset created', 'success')
          return
        }
        if (job.status === 'failed') {
          merging = false
          toastStore.show('Merge failed: ' + (job.error ?? 'unknown error'), 'error')
          return
        }
      } catch {
        // Ignore poll errors, keep trying
      }
      delay = Math.min(delay * 2, 10000)
    }
  }
</script>

<!-- Backdrop -->
<button
  type="button"
  class="fixed inset-0 z-40 bg-black/20 dark:bg-black/40"
  onclick={() => stackStore.closePanel()}
  aria-label="Close stack panel"
></button>

<!-- Panel -->
<div
  transition:fly={{ x: 400, duration: 250 }}
  class="fixed right-0 top-0 bottom-0 z-50 flex w-96 flex-col border-l border-gray-200 bg-white shadow-2xl dark:border-gray-700 dark:bg-gray-900"
>
  <!-- Header -->
  <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-gray-800">
    <div class="flex items-center gap-2">
      <Layers class="h-5 w-5 text-amber-500" />
      <span class="text-base font-semibold text-gray-900 dark:text-gray-100">
        Working Stack
        <span class="ml-1 text-sm font-normal text-gray-400">({stackStore.count})</span>
      </span>
    </div>
    <button
      type="button"
      class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-gray-800"
      onclick={() => stackStore.closePanel()}
    >
      <X class="h-4 w-4" />
    </button>
  </div>

  <!-- Label input -->
  <div class="border-b border-gray-100 px-5 py-3 dark:border-gray-800">
    <input
      type="text"
      placeholder="Label this stack…"
      bind:value={labelInput}
      onblur={handleLabelBlur}
      class="w-full rounded-lg border border-gray-200 bg-gray-50 px-3 py-1.5 text-sm text-gray-800 placeholder-gray-400 focus:border-amber-400 focus:outline-none focus:ring-1 focus:ring-amber-400 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500"
    />
  </div>

  <!-- Asset list -->
  <div class="flex-1 overflow-y-auto px-3 py-3">
    {#if stackStore.count === 0}
      <div class="flex flex-col items-center justify-center gap-2 py-16 text-gray-400">
        <Layers class="h-10 w-10 opacity-30" />
        <p class="text-sm">Stack is empty</p>
      </div>
    {:else}
      <div class="flex flex-col gap-1">
        {#each stackStore.assets as asset (asset.id)}
          <div class="flex items-center gap-3 rounded-lg p-2 hover:bg-gray-50 dark:hover:bg-gray-800">
            <div class="h-10 w-10 shrink-0 overflow-hidden rounded-md bg-gray-100 dark:bg-gray-700">
              {#if asset.thumbnailUrl}
                <img src={asset.thumbnailUrl} alt={asset.name} class="h-full w-full object-cover" />
              {:else}
                <div class="flex h-full w-full items-center justify-center">
                  <Layers class="h-4 w-4 text-gray-400" />
                </div>
              {/if}
            </div>
            <p class="flex-1 truncate text-sm text-gray-800 dark:text-gray-200" title={asset.name}>
              {asset.name}
            </p>
            <button
              type="button"
              class="shrink-0 rounded p-1 text-gray-300 hover:bg-gray-100 hover:text-gray-500 dark:hover:bg-gray-700 dark:text-gray-600 dark:hover:text-gray-400"
              onclick={() => stackStore.remove(asset.id)}
              title="Remove from stack"
            >
              <X class="h-3.5 w-3.5" />
            </button>
          </div>
        {/each}
      </div>
    {/if}
  </div>

  <!-- Actions -->
  <div class="border-t border-gray-100 px-5 py-4 dark:border-gray-800">
    <div class="flex flex-col gap-2">
      <!-- Download ZIP -->
      <button
        type="button"
        disabled={downloading || stackStore.count === 0}
        onclick={handleDownloadZip}
        class="flex w-full items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 text-sm transition-colors
          {downloading || stackStore.count === 0
            ? 'cursor-not-allowed text-gray-400 opacity-50 dark:border-gray-700'
            : 'text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800'}"
      >
        {#if downloading}
          <Loader class="h-4 w-4 animate-spin" />
        {:else}
          <Download class="h-4 w-4" />
        {/if}
        {downloading ? 'Downloading…' : 'Download as ZIP'}
      </button>

      <!-- Save as Collection -->
      {#if showCollectionForm}
        <div class="flex flex-col gap-1.5 rounded-lg border border-amber-200 bg-amber-50 p-3 dark:border-amber-800 dark:bg-amber-950">
          <input
            type="text"
            placeholder="Collection name…"
            bind:value={collectionName}
            class="w-full rounded border border-gray-200 bg-white px-2.5 py-1.5 text-sm text-gray-800 placeholder-gray-400 focus:border-amber-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
          />
          <div class="flex gap-2">
            <button
              type="button"
              disabled={savingCollection || !collectionName.trim()}
              onclick={handleSaveCollection}
              class="flex-1 rounded-lg bg-amber-500 px-3 py-1.5 text-sm font-medium text-white hover:bg-amber-600 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {savingCollection ? 'Saving…' : 'Save'}
            </button>
            <button
              type="button"
              onclick={() => { showCollectionForm = false; collectionName = '' }}
              class="rounded-lg border border-gray-200 px-3 py-1.5 text-sm text-gray-600 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-400 dark:hover:bg-gray-800"
            >
              Cancel
            </button>
          </div>
        </div>
      {:else}
        <button
          type="button"
          disabled={stackStore.count === 0}
          onclick={() => showCollectionForm = true}
          class="flex w-full items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 text-sm transition-colors
            {stackStore.count === 0
              ? 'cursor-not-allowed text-gray-400 opacity-50 dark:border-gray-700'
              : 'text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800'}"
        >
          <Save class="h-4 w-4" />
          Save as Collection
        </button>
      {/if}

      <!-- Share stack — WS-5 -->
      {#if shareUrl}
        <div class="flex flex-col gap-1.5 rounded-lg border border-green-200 bg-green-50 p-3 dark:border-green-800 dark:bg-green-950">
          <p class="text-xs font-medium text-green-700 dark:text-green-300">Share link ready</p>
          <div class="flex gap-2">
            <input
              type="text"
              readonly
              value={shareUrl}
              class="min-w-0 flex-1 rounded border border-gray-200 bg-white px-2 py-1 text-xs text-gray-700 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200"
            />
            <button
              type="button"
              onclick={handleCopyLink}
              class="shrink-0 rounded-lg border border-gray-200 px-2.5 py-1 text-xs text-gray-600 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-400 dark:hover:bg-gray-800"
            >
              {#if copied}
                <Check class="h-3.5 w-3.5 text-green-500" />
              {:else}
                <Copy class="h-3.5 w-3.5" />
              {/if}
            </button>
          </div>
          <button
            type="button"
            onclick={() => { shareUrl = null }}
            class="text-xs text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 text-left"
          >Dismiss</button>
        </div>
      {:else if showShareNameForm}
        <div class="flex flex-col gap-1.5 rounded-lg border border-green-200 bg-green-50 p-3 dark:border-green-800 dark:bg-green-950">
          <p class="text-xs font-medium text-green-700 dark:text-green-300">Name this collection to share it</p>
          <input
            type="text"
            placeholder="Collection name…"
            bind:value={shareNameInput}
            class="w-full rounded border border-gray-200 bg-white px-2.5 py-1.5 text-sm text-gray-800 placeholder-gray-400 focus:border-green-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
          />
          <div class="flex gap-2">
            <button
              type="button"
              disabled={sharing || !shareNameInput.trim()}
              onclick={() => doShare(shareNameInput.trim())}
              class="flex-1 rounded-lg bg-green-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-green-700 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {sharing ? 'Sharing…' : 'Share'}
            </button>
            <button
              type="button"
              onclick={() => { showShareNameForm = false }}
              class="rounded-lg border border-gray-200 px-3 py-1.5 text-sm text-gray-600 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-400 dark:hover:bg-gray-800"
            >Cancel</button>
          </div>
        </div>
      {:else}
        <button
          type="button"
          disabled={sharing || stackStore.count === 0}
          onclick={handleShareClick}
          class="flex w-full items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 text-sm transition-colors
            {sharing || stackStore.count === 0
              ? 'cursor-not-allowed text-gray-400 opacity-50 dark:border-gray-700'
              : 'text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800'}"
        >
          {#if sharing}
            <Loader class="h-4 w-4 animate-spin" />
            Sharing…
          {:else}
            <Share2 class="h-4 w-4" />
            Share as Collection
          {/if}
        </button>
      {/if}

      <!-- Move to Project — WS-5 -->
      {#if showProjectPicker}
        <div class="flex flex-col gap-1.5 rounded-lg border border-blue-200 bg-blue-50 p-3 dark:border-blue-800 dark:bg-blue-950">
          <input
            type="text"
            placeholder="Search projects…"
            bind:value={projectSearch}
            class="w-full rounded border border-gray-200 bg-white px-2.5 py-1.5 text-sm text-gray-800 placeholder-gray-400 focus:border-blue-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
          />
          {#if loadingProjects}
            <div class="flex justify-center py-2">
              <Loader class="h-4 w-4 animate-spin text-gray-400" />
            </div>
          {:else if filteredProjects.length === 0}
            <p class="py-2 text-center text-xs text-gray-400">No projects found</p>
          {:else}
            <div class="max-h-40 overflow-y-auto">
              {#each filteredProjects as project (project.id)}
                <button
                  type="button"
                  onclick={() => handleMoveToProject(project)}
                  class="flex w-full items-center gap-2 rounded px-2 py-1.5 text-sm text-gray-700 hover:bg-white dark:text-gray-300 dark:hover:bg-gray-800"
                >
                  <FolderInput class="h-3.5 w-3.5 shrink-0 text-gray-400" />
                  {project.name}
                </button>
              {/each}
            </div>
          {/if}
          <button
            type="button"
            onclick={() => { showProjectPicker = false }}
            class="text-xs text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 text-left"
          >Cancel</button>
        </div>
      {:else}
        <button
          type="button"
          disabled={movingToProject || stackStore.count === 0}
          onclick={handleOpenProjectPicker}
          class="flex w-full items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 text-sm transition-colors
            {movingToProject || stackStore.count === 0
              ? 'cursor-not-allowed text-gray-400 opacity-50 dark:border-gray-700'
              : 'text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800'}"
        >
          {#if movingToProject}
            <Loader class="h-4 w-4 animate-spin" />
            Moving…
          {:else}
            <FolderInput class="h-4 w-4" />
            Move to Project
          {/if}
        </button>
      {/if}

      <!-- Merge as GIF / PDF -->
      {#if showMergeForm}
        <div class="flex flex-col gap-1.5 rounded-lg border border-indigo-200 bg-indigo-50 p-3 dark:border-indigo-800 dark:bg-indigo-950">
          <div class="flex gap-2">
            <button
              type="button"
              onclick={() => mergeType = 'gif'}
              class="flex-1 rounded-lg border px-3 py-1.5 text-sm font-medium transition-colors
                {mergeType === 'gif' ? 'border-indigo-500 bg-indigo-500 text-white' : 'border-gray-200 text-gray-600 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-400'}"
            >GIF</button>
            <button
              type="button"
              onclick={() => mergeType = 'pdf'}
              class="flex-1 rounded-lg border px-3 py-1.5 text-sm font-medium transition-colors
                {mergeType === 'pdf' ? 'border-indigo-500 bg-indigo-500 text-white' : 'border-gray-200 text-gray-600 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-400'}"
            >PDF</button>
          </div>
          <div class="flex gap-2">
            <button
              type="button"
              disabled={merging}
              onclick={handleMerge}
              class="flex-1 rounded-lg bg-indigo-500 px-3 py-1.5 text-sm font-medium text-white hover:bg-indigo-600 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {merging ? 'Merging…' : 'Merge'}
            </button>
            <button
              type="button"
              onclick={() => showMergeForm = false}
              class="rounded-lg border border-gray-200 px-3 py-1.5 text-sm text-gray-600 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-400 dark:hover:bg-gray-800"
            >
              Cancel
            </button>
          </div>
        </div>
      {:else}
        <button
          type="button"
          disabled={stackStore.count < 2}
          onclick={() => showMergeForm = true}
          class="flex w-full items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 text-sm transition-colors
            {stackStore.count < 2
              ? 'cursor-not-allowed text-gray-400 opacity-50 dark:border-gray-700'
              : 'text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800'}"
        >
          {#if merging}
            <Loader class="h-4 w-4 animate-spin" />
            Merging ({mergeStatus})…
          {:else}
            <Tag class="h-4 w-4" />
            Merge as GIF / PDF
          {/if}
        </button>
      {/if}
    </div>

    <!-- Clear -->
    <button
      type="button"
      class="mt-4 flex w-full items-center justify-center gap-2 rounded-lg px-3 py-2 text-sm text-red-500 hover:bg-red-50 dark:hover:bg-red-950"
      onclick={() => { stackStore.clear(); stackStore.closePanel() }}
    >
      <Trash2 class="h-4 w-4" />
      Clear stack
    </button>
  </div>
</div>
