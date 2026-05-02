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
  import CollectionsSidebar from '$lib/components/CollectionsSidebar.svelte'
  import { collectionsStore } from '$lib/stores/collections.svelte'
  import { Activity, LogOut, Plus, Settings2, Megaphone, Info, LibraryBig, Users, Download, Plug, History, Shield, User, ArrowLeft, Tags } from '@lucide/svelte'
  import WorkspaceSwitcher from '$lib/components/WorkspaceSwitcher.svelte'
  import { goto } from '$app/navigation'
  import { page } from '$app/state'
  import { ingressStore } from '$lib/stores/ingress.svelte'
  import { fly } from 'svelte/transition'
  import { m } from '$lib/paraglide/messages'
  import BottomStatusBar from '$lib/components/BottomStatusBar.svelte'
  import Hint from '$lib/components/ui/Hint.svelte'
  import StackTray from '$lib/components/StackTray.svelte'
  import { useShortcuts } from '$lib/shortcuts'
  import { clearGMode } from '$lib/shortcuts/sequence'
  import { undoStore } from '$lib/stores/undo.svelte'
  import { BulkAssignAssetToProject } from '$lib/commands/BulkAssignAssetToProject'
  import { BulkAssignAssetToFolder } from '$lib/commands/BulkAssignAssetToFolder'
  import { browserDetectStore } from '$lib/stores/browserDetect.svelte'

  let { children }: { data: any, children: Snippet } = $props()

  let sidebarCreating = $state(false)

  const profileSections = [
    { id: 'account',       label: () => m.settings_auth_title(), path: '/library/settings/account',     icon: User },
  ]

  const settingsSections = [
    { id: 'members',       label: () => m.tab_members(),                 path: '/library/settings/members',       icon: Users },
    { id: 'tags',          label: () => m.tags(),                           path: '/library/settings/tags',          icon: Tags },
    { id: 'custom-fields', label: () => m.custom_fields_title(),   path: '/library/settings/custom-fields', icon: Settings2 },
    { id: 'ingress',       label: () => m.tab_ingress(),                 path: '/library/settings/ingress',       icon: Download },
    { id: 'integrations',  label: () => m.integrations_title(),     path: '/library/settings/integrations',  icon: Plug },
    { id: 'versioning',    label: () => m.tab_history(),                 path: '/library/settings/versioning',    icon: History },
  ]
  
  const securitySections = [
    { id: 'privacy',       label: () => m.tab_exif_privacy(),     path: '/library/settings/privacy',       icon: Shield },
    { id: 'activity',      label: () => m.activity(),                   path: '/library/settings/activity',      icon: Activity },
  ]

  const isSettings = $derived(page.url.pathname.startsWith('/library/settings'))
  const activeSettingsSection = $derived(
    [...profileSections, ...settingsSections, ...securitySections].find(s => page.url.pathname.startsWith(s.path))?.id ?? null
  )

  useShortcuts({
    'search.focus':      () => document.querySelector<HTMLInputElement>('[data-search]')?.focus(),
    'upload.open':       () => document.querySelector<HTMLInputElement>('[data-upload-trigger]')?.click(),
    'sidebar.toggle':    () => navigationStore.toggleSidebarVisible(),
    'navigate.library':  () => { clearGMode(); goto('/library') },
    'navigate.tags':     () => { clearGMode(); goto('/library/settings/tags') },
    'navigate.settings': () => { clearGMode(); goto('/library/settings/members') },
    'navigate.shares':   () => { clearGMode(); goto('/library/shares') },
  })

  async function handleProjectSelect(id: string | null) {
    navigationStore.selectProject(id)
    if (id) await foldersStore.loadForProject(id)
    goto('/library')
  }

  async function handleFolderSelect(_projectId: string, folderId: string | null) {
    navigationStore.selectFolder(folderId)
    goto('/library')
  }

  async function handleAssetsDropped(assetIds: string[], folderId: string | null, projectId: string) {
    try {
      const folder = foldersStore.foldersForActiveProject.find(f => f.id === folderId) ?? null
      await undoStore.execute(new BulkAssignAssetToFolder(
        assetsStore.assets.filter(sa => assetIds.includes(sa.id)),
        folderId ?? null,
        folder?.name ?? null,
        projectId,
      ))
      selectionStore.clear()
    } catch {
      toastStore.show(m.cannot_move_assets(), 'error')
    }
  }

  async function handleAssetsProjectDropped(assetIds: string[], projectId: string) {
    const beforeProjectIds = new Map(
      assetIds.map(id => [id, assetsStore.assets.find(a => a.id === id)?.project_id ?? null])
    )
    const projectName = projectsStore.projects.find(p => p.id === projectId)?.name ?? null
    try {
      await undoStore.execute(new BulkAssignAssetToProject(assetIds, beforeProjectIds, projectId, projectName))
      selectionStore.clear()
    } catch {
      toastStore.show(m.cannot_move_assets(), 'error')
    }
  }

  function handleCollectionSelect(id: string) {
    navigationStore.selectCollection(id)
    goto('/library')
  }

  let prevNavKey: string | null = null

  $effect(() => {
    const key = [
      navigationStore.activeProjectId,
      navigationStore.activeFolderId,
      navigationStore.activeCollectionId,
    ].join('|')
    if (key === prevNavKey) return
    prevNavKey = key
    assetsStore.load(true)
  })

  $effect(() => {
    if (projectsStore.stale) projectsStore.load()
  })

  $effect(() => {
    const activeId = navigationStore.activeProjectId
    if (activeId && foldersStore.staleProjects.has(activeId)) {
      foldersStore.loadForProject(activeId)
    }
  })

  onMount(() => {
    browserDetectStore.detectSmallDevice()

    projectsStore.load()
    collectionsStore.load()

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
  {#if navigationStore.sidebarVisible && isSettings}
    <aside
      in:fly={{ x: -256, duration: 150, delay: 150 }}
      out:fly={{ x: -256, duration: 150 }}
      class="damask-texture flex w-64 shrink-0 flex-col border-r border-gray-100 bg-white dark:border-gray-800 dark:bg-gray-900"
    >
      <WorkspaceSwitcher class="px-3 py-3" />

      <div class="px-3 pb-3">
        <a
          href="/library"
          class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-md text-gray-500 transition-colors hover:bg-gray-50 hover:text-gray-700 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-gray-200"
        >
          <ArrowLeft class="h-4 w-4 shrink-0" />
          <span>{m.back_to_library()}</span>
        </a>
      </div>

      <div class="px-3 pb-2 pt-1 border-t border-gray-100 dark:border-gray-800">
        <span class="px-3 text-sm font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">
          {m.user_profile()}
        </span>
      </div>

      <nav class="flex flex-col gap-0.5 px-3 pb-4">
        {#each profileSections as section}
          {@const Icon = section.icon}
          <a
            href={section.path}
            class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-md transition-colors
              {activeSettingsSection === section.id
                ? 'bg-gray-100 font-medium text-gray-900 dark:bg-gray-800 dark:text-gray-50'
                : 'text-gray-600 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800'}"
          >
            <Icon class="h-4 w-4 shrink-0 text-gray-400" />
            <span class="flex-1 text-left">{section.label()}</span>
          </a>
        {/each}
      </nav>

      <div class="px-3 pb-2 pt-1 border-t border-gray-100 dark:border-gray-800">
        <span class="px-3 text-sm font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">
          {m.settings()}
        </span>
      </div>

      <nav class="flex flex-col gap-0.5 px-3 pb-4">
        {#each settingsSections as section}
          {@const Icon = section.icon}
          <a
            href={section.path}
            class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-md transition-colors
              {activeSettingsSection === section.id
                ? 'bg-gray-100 font-medium text-gray-900 dark:bg-gray-800 dark:text-gray-50'
                : 'text-gray-600 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800'}"
          >
            <Icon class="h-4 w-4 shrink-0 text-gray-400" />
            <span class="flex-1 text-left">{section.label()}</span>
          </a>
        {/each}
      </nav>


      <div class="px-3 pb-2 pt-1 border-t border-gray-100 dark:border-gray-800">
        <span class="px-3 text-sm font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">
          {m.privacy_audit_logs()}
        </span>
      </div>

      <nav class="flex flex-col gap-0.5 px-3 pb-4">
        {#each securitySections as section}
          {@const Icon = section.icon}
          <a
            href={section.path}
            class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-md transition-colors
              {activeSettingsSection === section.id
                ? 'bg-gray-100 font-medium text-gray-900 dark:bg-gray-800 dark:text-gray-50'
                : 'text-gray-600 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800'}"
          >
            <Icon class="h-4 w-4 shrink-0 text-gray-400" />
            <span class="flex-1 text-left">{section.label()}</span>
          </a>
        {/each}
      </nav>

    </aside>
  {:else if navigationStore.sidebarVisible}
    <aside
      in:fly={{ x: -256, duration: 150, delay: 150 }}
      out:fly={{ x: -256, duration: 150 }}
      class="damask-texture flex relative w-64 shrink-0 flex-col border-r border-gray-100 bg-white dark:border-gray-800 dark:bg-gray-900"
    >
      <!-- Workspace switcher -->
      <WorkspaceSwitcher class="px-3 py-3" />

      <!-- All Assets button -->
      <div class="px-3 pb-2">
        <button
          class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-md transition-colors
            {(page.route.id === '/library' && navigationStore.activeProjectId === null) ? 'bg-gray-100 font-medium text-gray-900 dark:bg-gray-800 dark:text-gray-50' : 'text-gray-600 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800'}"
          onclick={() => handleProjectSelect(null)}
        >
          <LibraryBig class="h-4 w-4 shrink-0 text-gray-400" />
          <span class="flex-1 text-left">{m.all_assets()}</span>
          {#if authStore.totalAssetCount > 0}
            <span class="shrink-0 text-sm text-gray-400">{authStore.totalAssetCount}</span>
          {/if}
        </button>
      </div>

      <!-- Projects section -->
      <div class="flex flex-1 flex-col overflow-hidden px-3 pt-4 border-t border-gray-100 dark:border-gray-800">
        <div class="mb-2 flex items-center justify-between px-2">
          <span class="text-sm font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">{m.projects()}</span>
          {#if authStore.role !== 'viewer'}
            <button
              class="rounded text-gray-400 hover:bg-gray-100 hover:text-gray-600"
              onclick={() => { sidebarCreating = true }}
              aria-label={m.new_project()}
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

      <!-- Collections section -->
      {#if collectionsStore.collections.length > 0}
        <div class="flex flex-col overflow-hidden px-3 pt-4 border-t-2 border-gray-100 dark:border-gray-800">
          <div class="mb-2 flex items-center gap-2 px-2">
            <span class="text-sm font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">{m.collections()}</span>
          </div>
          <CollectionsSidebar onSelect={handleCollectionSelect} />
        </div>
      {/if}

      <div class="flex items-center justify-start border-t-2 border-gray-200 px-4 py-3 dark:border-gray-800">
        <Info class="h-3.5 w-3.5 text-gray-400" />
        <a href="https://docs.damask.studio" target="_blank" class="flex items-center gap-2 rounded-lg px-2 text-gray-400 hover:text-gray-700 dark:hover:text-gray-200">
          <span class="text-sm">{m.help_docs()}</span>
        </a>
      </div>
      <div class="flex items-center justify-start px-4 py-3">
        <Megaphone class="h-3.5 w-3.5 text-gray-400" />
        <a href="https://github.com/vincent/damask/discussions" target="_blank" class="flex items-center gap-2 rounded-lg px-2 text-gray-400 hover:text-gray-700 dark:hover:text-gray-200">
          <span class="text-sm">{m.help_feedback()}</span>
        </a>
      </div>

      <!-- Bottom sign out + theme toggle -->
      <div class="flex items-center justify-between border-t border-gray-200 px-4 py-3 dark:border-gray-800">
        <a href="/logout" class="flex items-center gap-2 rounded-lg text-md text-gray-400 hover:text-gray-700 dark:hover:text-gray-200">
          <LogOut class="h-3.5 w-3.5" />
          <Hint class="text-sm">{m.logout()}</Hint>
        </a>
      </div>
    </aside>
  {/if}

  <!-- Main -->
  <div class="relative flex flex-1 flex-col overflow-hidden">
    {@render children?.()}

    <BottomStatusBar />
    <StackTray />
  </div>
</div>
