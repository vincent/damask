<script lang="ts">
  import { onMount, type Snippet } from 'svelte'
  import { authStore } from '$lib/stores/auth.svelte'
  import { assetsStore } from '$lib/stores/assets.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import { foldersStore } from '$lib/stores/folders.svelte'
  import { navigationStore } from '$lib/stores/navigation.svelte'
  import { selectionStore } from '$lib/stores/selection.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { sharesStore } from '$lib/stores/shares.svelte'
  import ProjectSidebar from '$lib/components/ProjectSidebar.svelte'
  import { Activity, Book, LogOut, Plus, Share2, Rss, Settings2 } from '@lucide/svelte'
  import ThemeToggle from '$lib/components/ThemeToggle.svelte'
  import WorkspaceSwitcher from '$lib/components/WorkspaceSwitcher.svelte'
  import { goto } from '$app/navigation'
  import { page } from '$app/state'
  import { ingressStore } from '$lib/stores/ingress.svelte'
  
  let { children }: { data: any, children: Snippet } = $props()

  let sidebarCreating = $state(false)

  async function handleProjectSelect(id: string | null) {
    navigationStore.selectProject(id)
    if (id) await foldersStore.loadForProject(id)
    await assetsStore.load(true)
    goto('/library')
  }

  async function handleFolderSelect(_projectId: string, folderId: string | null) {
    navigationStore.selectFolder(folderId)
    await assetsStore.load(true)
    goto('/library')
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


  onMount(() => {
    projectsStore.load()
    assetsStore.load(true)

    setTimeout(() => {
      sharesStore.load()
      ingressStore.loadSources()
    }, 1000)
  })
</script>

<svelte:head>
  <title>Workspace — Damask</title>
</svelte:head>

<div class="bg-[var(--bg-app)] flex h-screen bg-gray-50 dark:bg-gray-950">
  <!-- Sidebar -->
  <aside class="damask-texture relative flex w-64 shrink-0 flex-col border-r border-gray-100 bg-white dark:border-gray-800 dark:bg-gray-900">

    <!-- Workspace switcher -->
    <WorkspaceSwitcher class="px-3 py-3" />

    <!-- All Assets button -->
    <div class="px-3 pb-2">
      <button
        class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors
          {(!page.route.id?.match('shares') && navigationStore.activeProjectId === null) ? 'bg-gray-100 font-medium text-gray-900 dark:bg-gray-800 dark:text-gray-50' : 'text-gray-600 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800'}"
        onclick={() => handleProjectSelect(null)}
      >
        <Book class="h-4 w-4 shrink-0 text-gray-400" />
        <span class="flex-1 text-left">All Assets</span>
        {#if projectsStore.totalAssetCount > 0}
          <span class="shrink-0 text-xs text-gray-400">{projectsStore.totalAssetCount}</span>
        {/if}
      </button>
    </div>

    <!-- All Shares button -->
    <div class="px-3 pb-2">
      <button
        class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors
          {(page.route.id?.match('shares')) ? 'bg-gray-100 font-medium text-gray-900 dark:bg-gray-800 dark:text-gray-50' : 'text-gray-600 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800'}"
        onclick={() => goto('/library/shares')}
      >
        <Share2 class="h-4 w-4 shrink-0 text-gray-400" />
        <span class="flex-1 text-left">All Shares</span>
        {#if sharesStore.shares?.length > 0}
          <span class="shrink-0 text-xs text-gray-400">{sharesStore.shares?.length}</span>
        {/if}
      </button>
    </div>

    <!-- Ingress Sources button -->
    <div class="px-3 pb-2">
      <button
        class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors
          {(page.route.id?.match('ingress')) ? 'bg-gray-100 font-medium text-gray-900 dark:bg-gray-800 dark:text-gray-50' : 'text-gray-600 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800'}"
        onclick={() => goto('/library/ingress')}
      >
        <Rss class="h-4 w-4 shrink-0 text-gray-400" />
        <span class="flex-1 text-left">Sources</span>
        {#if ingressStore.sources?.length > 0}
          <span class="shrink-0 text-xs text-gray-400">{ingressStore.sources?.length}</span>
        {/if}
      </button>
    </div>

    <!-- Custom Fields settings button -->
    <div class="px-3 pb-2">
      <button
        class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors
          {(page.route.id?.match('custom-fields')) ? 'bg-gray-100 font-medium text-gray-900 dark:bg-gray-800 dark:text-gray-50' : 'text-gray-600 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800'}"
        onclick={() => goto('/library/settings/custom-fields')}
      >
        <Settings2 class="h-4 w-4 shrink-0 text-gray-400" />
        <span class="flex-1 text-left">Custom Fields</span>
      </button>
    </div>

    <!-- Versioning settings button -->
    <div class="px-3 pb-2">
      <button
        class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors
          {(page.route.id?.match('versioning')) ? 'bg-gray-100 font-medium text-gray-900 dark:bg-gray-800 dark:text-gray-50' : 'text-gray-600 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800'}"
        onclick={() => goto('/library/settings/versioning')}
      >
        <Settings2 class="h-4 w-4 shrink-0 text-gray-400" />
        <span class="flex-1 text-left">Version History</span>
      </button>
    </div>

    <!-- Activity feed button -->
    <div class="px-3 pb-2">
      <button
        class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors
          {(page.route.id?.match('activity')) ? 'bg-gray-100 font-medium text-gray-900 dark:bg-gray-800 dark:text-gray-50' : 'text-gray-600 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800'}"
        onclick={() => goto('/library/activity')}
      >
        <Activity class="h-4 w-4 shrink-0 text-gray-400" />
        <span class="flex-1 text-left">Activity</span>
      </button>
    </div>

    <!-- Projects section -->
    <div class="flex flex-1 flex-col overflow-hidden px-3">
      <div class="mb-2 flex items-center justify-between px-2">
        <span class="text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">Projects</span>
        {#if authStore.role !== 'viewer'}
          <button
            class="rounded p-0.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600"
            onclick={() => { sidebarCreating = true }}
            aria-label="New project"
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
    {@render children?.()}
  </div>
</div>
