<script lang="ts">
  import { goto } from '$app/navigation'
  import { page } from '$app/state'
  import { onMount } from 'svelte'
  import CollectionsSidebar from '$lib/components/CollectionsSidebar.svelte'
  import StorageBar from '$lib/components/storage/StorageBar.svelte'
  import {
    fetchWorkspaceStorage,
    type WorkspaceStorageUsage,
  } from '$lib/api/storage'
  import ProjectSidebar from '$lib/components/ProjectSidebar.svelte'
  import WorkspaceSwitcher from '$lib/components/WorkspaceSwitcher.svelte'
  import { BulkAssignAssetToFolder } from '$lib/commands/BulkAssignAssetToFolder'
  import { BulkAssignAssetToProject } from '$lib/commands/BulkAssignAssetToProject'
  import { m } from '$lib/paraglide/messages'
  import { authStore } from '$lib/stores/auth.svelte'
  import { assetsStore } from '$lib/stores/assets.svelte'
  import { collectionsStore } from '$lib/stores/collections.svelte'
  import { foldersStore } from '$lib/stores/folders.svelte'
  import { navigationStore } from '$lib/stores/navigation.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import { selectionStore } from '$lib/stores/selection.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { undoStore } from '$lib/stores/undo.svelte'
  import {
    Activity,
    ArrowLeft,
    Download,
    HardDrive,
    History,
    Info,
    LibraryBig,
    Megaphone,
    Plus,
    Plug,
    Settings2,
    Shield,
    Upload,
    Tags,
    User,
    Users,
    Workflow,
  } from '@lucide/svelte'

  interface Props {
    onNavigate?: () => void
  }

  let { onNavigate = () => {} }: Props = $props()

  let sidebarCreating = $state(false)
  let storageUsage = $state<WorkspaceStorageUsage | null>(null)

  async function loadStorageUsage() {
    storageUsage = await fetchWorkspaceStorage().catch(() => null)
  }

  onMount(() => {
    loadStorageUsage()
    const interval = setInterval(loadStorageUsage, 60_000)
    return () => clearInterval(interval)
  })

  const profileSections = [
    {
      id: 'account',
      label: () => m.settings_account_title(),
      path: '/library/settings/account',
      icon: User,
    },
  ]

  const settingsSections = [
    {
      id: 'members',
      label: () => m.tab_members(),
      path: '/library/settings/members',
      icon: Users,
    },
    {
      id: 'tags',
      label: () => m.tags(),
      path: '/library/settings/tags',
      icon: Tags,
    },
    {
      id: 'custom-fields',
      label: () => m.custom_fields_title(),
      path: '/library/settings/custom-fields',
      icon: Settings2,
    },
    {
      id: 'ingress',
      label: () => m.tab_ingress(),
      path: '/library/settings/ingress',
      icon: Download,
    },
    {
      id: 'exports',
      label: () => 'Exports',
      path: '/library/settings/exports',
      icon: Upload,
    },
    {
      id: 'integrations',
      label: () => m.integrations_title(),
      path: '/library/settings/integrations',
      icon: Plug,
    },
    {
      id: 'workflows',
      label: () => 'Workflows',
      path: '/library/settings/workflows',
      icon: Workflow,
    },
    {
      id: 'versioning',
      label: () => m.tab_history(),
      path: '/library/settings/versioning',
      icon: History,
    },
    {
      id: 'storage',
      label: () => m.storage_breakdown_title(),
      path: '/library/settings/storage',
      icon: HardDrive,
    },
  ]

  const securitySections = [
    {
      id: 'privacy',
      label: () => m.tab_exif_privacy(),
      path: '/library/settings/privacy',
      icon: Shield,
    },
    {
      id: 'activity',
      label: () => m.activity(),
      path: '/library/settings/activity',
      icon: Activity,
    },
  ]

  const isSettings = $derived(page.url.pathname.startsWith('/library/settings'))
  const activeSettingsSection = $derived(
    [...profileSections, ...settingsSections, ...securitySections].find((s) =>
      page.url.pathname.startsWith(s.path)
    )?.id ?? null
  )

  async function handleProjectSelect(id: string | null) {
    navigationStore.selectProject(id)
    if (id) await foldersStore.loadForProject(id)
    await goto('/library')
    onNavigate()
  }

  async function handleFolderSelect(
    _projectId: string,
    folderId: string | null
  ) {
    navigationStore.selectFolder(folderId)
    await goto('/library')
    onNavigate()
  }

  async function handleAssetsDropped(
    assetIds: string[],
    folderId: string | null,
    projectId: string
  ) {
    try {
      const folder =
        foldersStore.foldersForActiveProject.find((f) => f.id === folderId) ??
        null
      await undoStore.execute(
        new BulkAssignAssetToFolder(
          assetsStore.assets.filter((asset) => assetIds.includes(asset.id)),
          folderId ?? null,
          folder?.name ?? null,
          projectId
        )
      )
      selectionStore.clear()
    } catch {
      toastStore.show(m.cannot_move_assets(), 'error')
    }
  }

  async function handleAssetsProjectDropped(
    assetIds: string[],
    projectId: string
  ) {
    const beforeProjectIds = new Map(
      assetIds.map((id) => [
        id,
        assetsStore.assets.find((asset) => asset.id === id)?.project_id ?? null,
      ])
    )
    const projectName =
      projectsStore.projects.find((project) => project.id === projectId)
        ?.name ?? null

    try {
      await undoStore.execute(
        new BulkAssignAssetToProject(
          assetIds,
          beforeProjectIds,
          projectId,
          projectName
        )
      )
      selectionStore.clear()
    } catch {
      toastStore.show(m.cannot_move_assets(), 'error')
    }
  }

  async function handleCollectionSelect(id: string) {
    navigationStore.selectCollection(id)
    await goto('/library')
    onNavigate()
  }

  function handleAnchorNavigate() {
    onNavigate()
  }
</script>

<aside
  class="relative flex h-full w-full shrink-0 flex-col border-r border-[var(--border-subtle)] bg-[var(--bg-sidebar)] md:w-64"
>
  {#if isSettings}
    <WorkspaceSwitcher class="px-3 py-3" />

    <div class="px-3 pb-2">
      <a
        href="/library"
        class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-sm text-[var(--text-muted)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-secondary)]"
        onclick={handleAnchorNavigate}
      >
        <ArrowLeft class="h-4 w-4 shrink-0" />
        <span>{m.back_to_library()}</span>
      </a>
    </div>

    <div class="border-t border-[var(--border-subtle)] px-3 pt-3 pb-1.5">
      <span
        class="px-3 text-xs font-medium tracking-widest text-[var(--text-muted)] uppercase"
      >
        {m.user_profile()}
      </span>
    </div>

    <nav class="flex flex-col gap-0.5 px-3 pb-3">
      {#each profileSections as section}
        {@const Icon = section.icon}
        <a
          href={section.path}
          class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors
            {activeSettingsSection === section.id
            ? 'bg-[var(--accent-soft)] font-medium text-[var(--accent-text)]'
            : 'text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'}"
          onclick={handleAnchorNavigate}
        >
          <Icon
            class="h-4 w-4 shrink-0 {activeSettingsSection === section.id
              ? 'text-[var(--accent)]'
              : 'text-[var(--text-muted)]'}"
          />
          <span class="flex-1 text-left">{section.label()}</span>
        </a>
      {/each}
    </nav>

    <div class="border-t border-[var(--border-subtle)] px-3 pt-3 pb-1.5">
      <span
        class="px-3 text-xs font-medium tracking-widest text-[var(--text-muted)] uppercase"
      >
        {m.workspace()}
      </span>
    </div>

    <nav class="flex flex-col gap-0.5 px-3 pb-3">
      {#each settingsSections as section}
        {@const Icon = section.icon}
        <a
          href={section.path}
          class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors
            {activeSettingsSection === section.id
            ? 'bg-[var(--accent-soft)] font-medium text-[var(--accent-text)]'
            : 'text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'}"
          onclick={handleAnchorNavigate}
        >
          <Icon
            class="h-4 w-4 shrink-0 {activeSettingsSection === section.id
              ? 'text-[var(--accent)]'
              : 'text-[var(--text-muted)]'}"
          />
          <span class="flex-1 text-left">{section.label()}</span>
        </a>
      {/each}
    </nav>

    <div class="border-t border-[var(--border-subtle)] px-3 pt-3 pb-1.5">
      <span
        class="px-3 text-xs font-medium tracking-widest text-[var(--text-muted)] uppercase"
      >
        {m.privacy_audit_logs()}
      </span>
    </div>

    <nav class="flex flex-col gap-0.5 px-3 pb-3">
      {#each securitySections as section}
        {@const Icon = section.icon}
        <a
          href={section.path}
          class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors
            {activeSettingsSection === section.id
            ? 'bg-[var(--accent-soft)] font-medium text-[var(--accent-text)]'
            : 'text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'}"
          onclick={handleAnchorNavigate}
        >
          <Icon
            class="h-4 w-4 shrink-0 {activeSettingsSection === section.id
              ? 'text-[var(--accent)]'
              : 'text-[var(--text-muted)]'}"
          />
          <span class="flex-1 text-left">{section.label()}</span>
        </a>
      {/each}
    </nav>
  {:else}
    <WorkspaceSwitcher class="px-3 py-3" />

    <div class="px-3 pb-2">
      <button
        class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors
          {page.route.id === '/library' &&
        navigationStore.activeProjectId === null
          ? 'bg-[var(--accent-soft)] font-medium text-[var(--accent-text)]'
          : 'text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'}"
        onclick={() => handleProjectSelect(null)}
      >
        <LibraryBig
          class="h-4 w-4 shrink-0 {page.route.id === '/library' &&
          navigationStore.activeProjectId === null
            ? 'text-[var(--accent)]'
            : 'text-[var(--text-muted)]'}"
        />
        <span class="flex-1 text-left">{m.all_assets()}</span>
        {#if authStore.totalAssetCount > 0}
          <span class="shrink-0 text-xs text-[var(--text-muted)] tabular-nums"
            >{authStore.totalAssetCount}</span
          >
        {/if}
      </button>
    </div>

    <div
      class="mb-4 flex flex-col overflow-hidden border-t border-[var(--border-subtle)] px-3 pt-3"
    >
      <div class="mb-2 flex items-center justify-between px-2">
        <span
          class="text-xs font-medium tracking-widest text-[var(--text-muted)] uppercase"
          >{m.projects()}</span
        >
        {#if authStore.role !== 'viewer'}
          <button
            class="rounded p-0.5 text-[var(--text-muted)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-secondary)]"
            onclick={() => {
              sidebarCreating = true
            }}
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
          onCreatingChange={(value) => {
            sidebarCreating = value
          }}
          onSelect={handleProjectSelect}
          onFolderSelect={handleFolderSelect}
          onAssetsFolderDropped={handleAssetsDropped}
          onAssetsProjectDropped={handleAssetsProjectDropped}
        />
      </nav>
    </div>

    {#if collectionsStore.collections.length > 0}
      <div
        class="flex flex-col overflow-hidden border-t border-[var(--border-subtle)] px-3 pt-3"
      >
        <div class="mb-2 flex items-center px-2">
          <span
            class="text-xs font-medium tracking-widest text-[var(--text-muted)] uppercase"
            >{m.collections()}</span
          >
        </div>
        <CollectionsSidebar onSelect={handleCollectionSelect} />
      </div>
    {/if}

    {#if storageUsage}
      <a class="mt-auto" href="/library/settings/storage">
        <div class="border-t border-[var(--border-subtle)] px-4 py-2">
          <StorageBar
            used={storageUsage.total_bytes}
            limit={storageUsage.limit_bytes}
            compact
          />
        </div>
      </a>
    {/if}

    <div class="border-t border-[var(--border-subtle)] px-3 py-2">
      <a
        href="https://github.com/vincent/damask/discussions"
        target="_blank"
        class="flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm text-[var(--text-muted)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-secondary)]"
        onclick={handleAnchorNavigate}
      >
        <Megaphone class="h-3.5 w-3.5 shrink-0" />
        <span>{m.help_feedback()}</span>
      </a>
      <a
        href="/docs/"
        target="_blank"
        class="flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm text-[var(--text-muted)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-secondary)]"
        onclick={handleAnchorNavigate}
      >
        <Info class="h-3.5 w-3.5 shrink-0" />
        <span>{m.help_docs()}</span>
      </a>
    </div>
  {/if}
</aside>
