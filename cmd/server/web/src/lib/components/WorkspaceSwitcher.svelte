<script lang="ts">
  import { Check, ChevronDown, Plus, Settings, Share2, Users, Image } from '@lucide/svelte'
  import { workspaceApi, type WorkspaceWithRole } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { m } from '$lib/paraglide/messages'
  import { undoStore } from '$lib/stores/undo.svelte'

  let open = $state(false)
  let workspaces = $state<WorkspaceWithRole[]>([])
  let switching = $state(false)

  interface Props {
    class?: string,
  }

  let { class: extraClass = '' }: Props = $props()

  const activeWs = $derived(workspaces.find(w => w.id === authStore.workspace?.id))
  const otherWs  = $derived(workspaces.filter(w => w.id !== authStore.workspace?.id))

  async function loadWorkspaces() {
    try {
      workspaces = await workspaceApi.list()
    } catch {
      // keep empty — dropdown still shows current workspace
    }
  }

  function toggle() {
    if (!open) loadWorkspaces()
    open = !open
  }

  function handleOutsideClick(e: MouseEvent) {
    if (!(e.target as Element).closest('[data-workspace-switcher]')) {
      open = false
    }
  }

  async function switchTo(ws: WorkspaceWithRole) {
    if (ws.id === authStore.workspace?.id) {
      open = false
      return
    }
    switching = true
    try {
      const res = await workspaceApi.switch(ws.id)
      authStore.switchWorkspace(res.workspace, res.role)
      undoStore.clear()
      open = false
      window.location.reload()
    } catch {
      switching = false
    }
  }

  async function createNew() {
    const name = prompt(m.workspace_prompt_name())
    if (!name?.trim()) return
    try {
      await workspaceApi.createWorkspace(name.trim())
      loadWorkspaces()
    } catch {
      // silently ignore
    }
  }

  const initials = $derived(
    authStore.workspace?.name?.[0]?.toUpperCase() ?? 'W'
  )
</script>

<svelte:window onclick={handleOutsideClick} />

<div class="relative {extraClass}" data-workspace-switcher>
  <button
    onclick={toggle}
    class="flex w-full items-center gap-2.5 rounded-lg px-1 py-1 transition-colors hover:bg-gray-100 dark:hover:bg-gray-800"
    aria-expanded={open}
  >
    <div class="flex items-center gap-2">
      <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-gradient-to-br from-indigo-500 to-purple-600 text-md font-bold text-white">
        {initials}
      </div>
      <span class="flex-1 truncate text-left text-xl font-semibold text-gray-900 dark:text-gray-50">
        {authStore.workspace?.name ?? 'Workspace'}
        {#if activeWs}<span class="flex text-sm text-gray-400">{activeWs.member_count} {m.members()}</span>{/if}
      </span>
      <ChevronDown class="h-4 w-4 shrink-0 text-gray-400 transition-transform {open ? 'rotate-180' : ''}" />
    </div>
  </button>

  {#if open}
    <div
      class="absolute left-1 top-full z-50 w-80 rounded-xl border border-gray-200 bg-white py-2 shadow-lg dark:border-gray-700 dark:bg-gray-900"
      role="menu"
    >
      <!-- Active workspace — tall card -->
      {#if activeWs}
        <div class="mx-2 mb-2 rounded-lg bg-indigo-50 p-3 dark:bg-indigo-950">
          <div class="mb-2 flex items-center gap-2">
            <div class="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-gradient-to-br from-indigo-500 to-purple-600 text-sm font-bold text-white">
              {activeWs.name[0]?.toUpperCase()}
            </div>

            <div class="flex-1 my-1 truncate font-semibold text-indigo-900 dark:text-indigo-100">{activeWs.name}</div>

            <Check class="h-4 w-4 shrink-0 text-indigo-600 dark:text-indigo-400" />
          </div>

          <div class="my-3 flex gap-3 text-xs text-indigo-700 dark:text-indigo-300">
            <span class="flex items-center gap-1">
              <Users class="h-3.5 w-3.5" />
              {activeWs.member_count} {m.members()}
            </span>
            <span class="flex items-center gap-1">
              <Image class="h-3.5 w-3.5" />
              {m.assets_count({ count: activeWs.asset_count })}
            </span>
          </div>

          <div class="flex items-center gap-2 mt-2">
            <a
              href="/library/shares"
              onclick={() => open = false}
              class="flex items-center gap-1 rounded-md bg-white px-2 py-1 text-xs font-medium text-gray-700 shadow-sm hover:bg-gray-50 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
            >
              <Share2 class="h-3 w-3" />
              {m.shares()}
            </a>
            <a
              href="/library/settings/members"
              onclick={() => open = false}
              class="flex items-center gap-1 rounded-md bg-white px-2 py-1 text-xs font-medium text-gray-700 shadow-sm hover:bg-gray-50 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
            >
              <Settings class="h-3 w-3" />
              {m.settings()}
            </a>
          </div>
        </div>
      {/if}

      <!-- Other workspaces — compact rows -->
      {#if otherWs.length > 0}
        <p class="px-3 pb-1 pt-0.5 text-xs font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">
          {m.workspaces()}
        </p>
        {#each otherWs as ws (ws.id)}
          <button
            onclick={() => switchTo(ws)}
            disabled={switching}
            class="flex w-full items-center gap-3 px-5 py-3 text-left text-md text-gray-700 transition-colors hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-gray-800"
            role="menuitem"
          >
            <div class="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-gradient-to-br from-indigo-500 to-purple-600 text-sm font-bold text-white">
              {ws.name[0]?.toUpperCase()}
            </div>
            <span class="flex-1 truncate font-medium">{ws.name}</span>
            <span class="flex items-center gap-1 text-xs text-gray-400 dark:text-gray-500">
              <Users class="h-3 w-3" />
              {ws.member_count}
            </span>
          </button>
        {/each}
      {/if}

      <div class="my-1 border-t border-gray-100 dark:border-gray-800"></div>

      <button
        onclick={createNew}
        class="flex w-full items-center gap-3 px-5 py-2 text-md text-gray-500 transition-colors hover:bg-gray-50 hover:text-gray-700 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-gray-200"
        role="menuitem"
      >
        <div class="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg border-2 border-dashed border-gray-300 dark:border-gray-600">
          <Plus class="h-3.5 w-3.5" />
        </div>
        <span>{m.add_new_workspace()}</span>
      </button>
    </div>
  {/if}
</div>
