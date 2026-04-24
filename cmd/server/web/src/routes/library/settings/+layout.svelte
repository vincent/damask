<script lang="ts">
  import { page } from '$app/state'
  import type { Snippet } from 'svelte'
  import { Users, Download, Plug, History, Shield, User, Settings2, Tag, ArrowLeft } from '@lucide/svelte'
  import { m } from '$lib/paraglide/messages'
  import WorkspaceSwitcher from '$lib/components/WorkspaceSwitcher.svelte'

  let { children }: { children: Snippet } = $props()

  const sections = [
    { id: 'members',       label: m.tab_members(),        path: '/library/settings/members',      icon: Users },
    { id: 'ingress',       label: m.tab_ingress(),         path: '/library/settings/ingress',       icon: Download },
    { id: 'integrations',  label: m.integrations_title(),  path: '/library/settings/integrations',  icon: Plug },
    { id: 'versioning',    label: m.tab_history(),         path: '/library/settings/versioning',    icon: History },
    { id: 'privacy',       label: m.tab_exif_privacy(),    path: '/library/settings/privacy',       icon: Shield },
    { id: 'account',       label: m.settings_auth_title(), path: '/library/settings/account',       icon: User },
    { id: 'custom-fields', label: m.custom_fields_title(), path: '/library/settings/custom-fields', icon: Settings2 },
    { id: 'tags',          label: m.tags(),                path: '/library/settings/tags',          icon: Tag },
  ]

  const activeSection = $derived(
    sections.find(s => page.url.pathname.startsWith(s.path))?.id ?? null
  )
</script>

{#if activeSection !== null}
  <div class="flex flex-1 overflow-hidden">
    <!-- Settings sidebar (replaces library sidebar) -->
    <aside class="damask-texture flex w-64 shrink-0 flex-col border-r border-gray-100 bg-white dark:border-gray-800 dark:bg-gray-900">
      <WorkspaceSwitcher class="px-3 py-3" />

      <!-- Back to Library -->
      <div class="px-3 pb-3">
        <a
          href="/library"
          class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-md text-gray-500 transition-colors hover:bg-gray-50 hover:text-gray-700 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-gray-200"
        >
          <ArrowLeft class="h-4 w-4 shrink-0" />
          <span>{m.back_to_library()}</span>
        </a>
      </div>

      <!-- Section label -->
      <div class="px-3 pb-2 pt-1 border-t border-gray-100 dark:border-gray-800">
        <span class="px-3 text-sm font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">
          {m.settings()}
        </span>
      </div>

      <nav class="flex flex-col gap-0.5 px-3 pb-4">
        {#each sections as section}
          {@const Icon = section.icon}
          <a
            href={section.path}
            class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-md transition-colors
              {activeSection === section.id
                ? 'bg-gray-100 font-medium text-gray-900 dark:bg-gray-800 dark:text-gray-50'
                : 'text-gray-600 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800'}"
          >
            <Icon class="h-4 w-4 shrink-0 text-gray-400" />
            <span class="flex-1 text-left">{section.label}</span>
          </a>
        {/each}
      </nav>
    </aside>

    <!-- Page content -->
    <div class="flex flex-1 flex-col overflow-hidden bg-gray-50 dark:bg-gray-950">
      {@render children()}
    </div>
  </div>
{:else}
  {@render children()}
{/if}
