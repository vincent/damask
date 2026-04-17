<script lang="ts">
  import { page } from '$app/state'
  import { goto } from '$app/navigation'
  import type { Snippet } from 'svelte'
  import PageHeader from '$lib/components/ui/PageHeader.svelte'
  import { m } from '$lib/paraglide/messages'

  let { children }: { children: Snippet } = $props()

  const tabs = [
    { id: 'members',    label: m.tab_members(),           path: '/library/settings/members' },
    { id: 'ingress',    label: m.tab_ingress(),           path: '/library/settings/ingress' },
    { id: 'versioning', label: m.tab_history(),   path: '/library/settings/versioning' },
    { id: 'privacy',    label: m.tab_exif_privacy(),   path: '/library/settings/privacy' },
  ]

  const activeTab = $derived(
    tabs.find(t => page.url.pathname.startsWith(t.path))?.id ?? null
  )
</script>

{#if activeTab !== null}
  <div class="flex flex-1 flex-col overflow-hidden bg-gray-50 dark:bg-gray-950">
    <PageHeader
      title={m.settings()}
      description={m.workspace_settings_description()}
    />

    <!-- Tab bar -->
    <div class="flex border-b border-gray-100 px-6 dark:border-gray-800">
      {#each tabs as tab}
        <button
          type="button"
          class="relative py-3 pr-6 text-md font-medium transition-colors
            {activeTab === tab.id
              ? 'text-indigo-600 dark:text-indigo-400'
              : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'}"
          onclick={() => goto(tab.path)}
        >
          {tab.label}
          {#if activeTab === tab.id}
            <span class="absolute bottom-0 left-0 right-4 h-0.5 rounded-t bg-indigo-600 dark:bg-indigo-400"></span>
          {/if}
        </button>
      {/each}
    </div>

    {@render children()}
  </div>
{:else}
  {@render children()}
{/if}
