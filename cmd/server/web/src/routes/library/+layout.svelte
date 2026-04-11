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
  import { Activity, Book, LogOut, Plus, Share2, Rss, Settings2, ChevronDown, ChevronUp, GitBranch, Tag, Megaphone, Info, ScanLine } from '@lucide/svelte'
  import ThemeToggle from '$lib/components/ThemeToggle.svelte'
  import WorkspaceSwitcher from '$lib/components/WorkspaceSwitcher.svelte'
  import { goto } from '$app/navigation'
  import { page } from '$app/state'
  import { ingressStore } from '$lib/stores/ingress.svelte'
  import { fly } from 'svelte/transition'
  
  let { children }: { data: any, children: Snippet } = $props()

  let sidebarCreating = $state(false)
  let sidebarDetails = $state(false)

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
      selectionStore.clear()
    } catch {
      toastStore.show('Could not move assets', 'error')
    }
  }

  async function handleAssetsProjectDropped(assetIds: string[], projectId: string) {
    try {
      await assetsStore.bulkProject(assetIds, projectId)
      toastStore.show(`Moved ${assetIds.length} asset${assetIds.length > 1 ? 's' : ''}`)
      await assetsStore.load(true)
      await projectsStore.load()
      await foldersStore.loadForProject(projectId)
      selectionStore.clear()
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
        class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-md transition-colors
          {(page.route.id === '/library' && navigationStore.activeProjectId === null) ? 'bg-gray-100 font-medium text-gray-900 dark:bg-gray-800 dark:text-gray-50' : 'text-gray-600 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800'}"
        onclick={() => handleProjectSelect(null)}
      >
        <Book class="h-4 w-4 shrink-0 text-gray-400" />
        <span class="flex-1 text-left">All Assets</span>
        {#if projectsStore.totalAssetCount > 0}
          <span class="shrink-0 text-sm text-gray-400">{projectsStore.totalAssetCount}</span>
        {/if}
      </button>
    </div>

    {#if sidebarDetails}
      <div transition:fly={{ y: '-50%', duration: 50 }}>
        <!-- All Shares button -->
        <div class="px-3 pb-2">
          <button
            class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-md transition-colors
              {(page.route.id?.match('shares')) ? 'bg-gray-100 font-medium text-gray-900 dark:bg-gray-800 dark:text-gray-50' : 'text-gray-600 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800'}"
            onclick={() => goto('/library/shares')}
          >
            <Share2 class="h-4 w-4 shrink-0 text-gray-400" />
            <span class="flex-1 text-left">All Shares</span>
            {#if sharesStore.shares?.length > 0}
              <span class="shrink-0 text-sm text-gray-400">{sharesStore.shares?.length}</span>
            {/if}
          </button>
        </div>

        <!-- Ingress Sources button -->
        <div class="px-3 pb-2">
          <button
            class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-md transition-colors
              {(page.route.id?.match('ingress')) ? 'bg-gray-100 font-medium text-gray-900 dark:bg-gray-800 dark:text-gray-50' : 'text-gray-600 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800'}"
            onclick={() => goto('/library/ingress')}
          >
            <Rss class="h-4 w-4 shrink-0 text-gray-400" />
            <span class="flex-1 text-left">Sources</span>
            {#if ingressStore.sources?.length > 0}
              <span class="shrink-0 text-sm text-gray-400">{ingressStore.sources?.length}</span>
            {/if}
          </button>
        </div>

        <!-- Custom Fields settings button -->
        <div class="px-3 pb-2">
          <button
            class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-md transition-colors
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
            class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-md transition-colors
              {(page.route.id?.match('versioning')) ? 'bg-gray-100 font-medium text-gray-900 dark:bg-gray-800 dark:text-gray-50' : 'text-gray-600 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800'}"
            onclick={() => goto('/library/settings/versioning')}
          >
            <GitBranch class="h-4 w-4 shrink-0 text-gray-400" />
            <span class="flex-1 text-left">Version History</span>
          </button>
        </div>

        <!-- Tags settings button -->
        <div class="px-3 pb-2">
          <button
            class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-md transition-colors
              {(page.route.id?.match('settings/tags')) ? 'bg-gray-100 font-medium text-gray-900 dark:bg-gray-800 dark:text-gray-50' : 'text-gray-600 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800'}"
            onclick={() => goto('/library/settings/tags')}
          >
            <Tag class="h-4 w-4 shrink-0 text-gray-400" />
            <span class="flex-1 text-left">Tags</span>
          </button>
        </div>

        <!-- EXIF & Privacy settings button -->
        <div class="px-3 pb-2">
          <button
            class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-md transition-colors
              {(page.route.id?.match('settings/privacy')) ? 'bg-gray-100 font-medium text-gray-900 dark:bg-gray-800 dark:text-gray-50' : 'text-gray-600 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800'}"
            onclick={() => goto('/library/settings/privacy')}
          >
            <ScanLine class="h-4 w-4 shrink-0 text-gray-400" />
            <span class="flex-1 text-left">EXIF & Privacy</span>
          </button>
        </div>

        <!-- Activity feed button -->
        <div class="px-3 pb-2">
          <button
            class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-md transition-colors
              {(page.route.id?.match('activity')) ? 'bg-gray-100 font-medium text-gray-900 dark:bg-gray-800 dark:text-gray-50' : 'text-gray-600 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800'}"
            onclick={() => goto('/library/activity')}
          >
            <Activity class="h-4 w-4 shrink-0 text-gray-400" />
            <span class="flex-1 text-left">Activity</span>
          </button>
        </div>

        <!-- svelte-ignore a11y_unknown_role -->
        <!-- svelte-ignore a11y_click_events_have_key_events -->
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div role="expand" onclick={() => (sidebarDetails = false)} class="px-2 pe-5 py-1 flex justify-center cursor-pointer text-gray-400 dark:text-gray-500 hover:text-gray-800 hover:dark:text-gray-100">
          <button type="button"><ChevronUp class="h-4 w-4" /></button>
        </div>
      </div>
    {:else}
        <!-- svelte-ignore a11y_unknown_role -->
        <!-- svelte-ignore a11y_click_events_have_key_events -->
        <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div role="expand" onclick={() => (sidebarDetails = true)}  class="px-2 pe-5 py-1 flex justify-center cursor-pointer text-gray-400 dark:text-gray-500 hover:text-gray-800 hover:dark:text-gray-100">
        <button type="button"><ChevronDown class="h-4 w-4" /></button>
      </div>
    {/if}

    <!-- Projects section -->
    <div class="flex flex-1 flex-col overflow-hidden px-3 pt-4 border-t border-gray-100 dark:border-gray-800">
      <div class="mb-2 flex items-center justify-between px-2">
        <span class="text-sm font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">Projects</span>
        {#if authStore.role !== 'viewer'}
          <button
            class="rounded text-gray-400 hover:bg-gray-100 hover:text-gray-600"
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
          onCreatingChange={(v) => { sidebarCreating = v }}
          onSelect={handleProjectSelect}
          onFolderSelect={handleFolderSelect}
          onAssetsFolderDropped={handleAssetsDropped}
          onAssetsProjectDropped={handleAssetsProjectDropped}
        />
      </nav>
    </div>

    <div class="flex items-center justify-start border-t border-gray-200 px-4 py-3 dark:border-gray-800">
      <Info class="h-3.5 w-3.5 text-gray-400" />
      <a href="https://docs.damask.studio" target="_blank" class="flex items-center gap-2 rounded-lg px-2 text-gray-400 hover:text-gray-700 dark:hover:text-gray-200">
        <span class="text-sm">Docs</span>
      </a>
    </div>
    <div class="flex items-center justify-start px-4 py-3">
      <Megaphone class="h-3.5 w-3.5 text-gray-400" />
      <a href="https://github.com/vincent/damask/discussions" target="_blank" class="flex items-center gap-2 rounded-lg px-2 text-gray-400 hover:text-gray-700 dark:hover:text-gray-200">
        <span class="text-sm">Feedback</span>
      </a>
    </div>
    <!-- Bottom sign out + theme toggle -->
    <div class="flex items-center justify-between border-t border-gray-200 px-4 py-3 dark:border-gray-800">
      <ThemeToggle />
      <a href="/logout" class="flex items-center gap-2 rounded-lg px-2 text-md text-gray-400 hover:text-gray-700 dark:hover:text-gray-200">
        <LogOut class="h-3.5 w-3.5" />
      </a>
    </div>
  </aside>

  <!-- Main -->
  <div class="relative flex flex-1 flex-col overflow-hidden">
    {@render children?.()}
  </div>
</div>
