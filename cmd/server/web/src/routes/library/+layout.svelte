<script lang="ts">
  import { onMount, type Snippet } from 'svelte'
  import { authStore } from '$lib/stores/auth.svelte'
  import { assetsStore } from '$lib/stores/assets.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import { foldersStore } from '$lib/stores/folders.svelte'
  import { navigationStore } from '$lib/stores/navigation.svelte'
  import { sharesStore } from '$lib/stores/shares.svelte'
  import { collectionsStore } from '$lib/stores/collections.svelte'
  import { ingressStore } from '$lib/stores/ingress.svelte'
  import AppShell from '$lib/components/AppShell.svelte'
  import BottomStatusBar from '$lib/components/BottomStatusBar.svelte'
  import LibrarySidebar from '$lib/components/LibrarySidebar.svelte'
  import StackTray from '$lib/components/StackTray.svelte'
  import { useShortcuts } from '$lib/shortcuts'
  import { clearGMode } from '$lib/shortcuts/sequence'
  import { browserDetectStore } from '$lib/stores/browserDetect.svelte'
  import { viewportStore } from '$lib/stores/viewport.svelte'
  import { goto } from '$app/navigation'

  let { children }: { data: any; children: Snippet } = $props()

  useShortcuts({
    'search.focus': () =>
      document.querySelector<HTMLInputElement>('[data-search]')?.focus(),
    'upload.open': () =>
      document
        .querySelector<HTMLInputElement>('[data-upload-trigger]')
        ?.click(),
    'sidebar.toggle': () => navigationStore.toggleSidebarVisible(),
    'navigate.library': () => {
      clearGMode()
      goto('/library')
    },
    'navigate.tags': () => {
      clearGMode()
      goto('/library/settings/tags')
    },
    'navigate.settings': () => {
      clearGMode()
      goto('/library/settings/members')
    },
    'navigate.shares': () => {
      clearGMode()
      goto('/library/shares')
    },
  })

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

<AppShell showSidebar={navigationStore.sidebarVisible}>
  {#snippet sidebar()}
    <LibrarySidebar />
  {/snippet}

  {#snippet children()}
    <div class="relative flex flex-1 flex-col overflow-hidden">
      {@render children?.()}

      {#if !viewportStore.isMobile}
        <BottomStatusBar />
      {/if}
      <StackTray />
    </div>
  {/snippet}
</AppShell>
