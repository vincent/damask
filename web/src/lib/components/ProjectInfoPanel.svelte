<script lang="ts">
  import type { Project } from '$lib/api/models'
  import { authStore } from '$lib/stores/auth.svelte'
  import { ChevronDown, ChevronRight, Box } from '@lucide/svelte'
  import ProjectCustomFields from './ProjectCustomFields.svelte'

  interface Props {
    project: Project
  }

  let { project }: Props = $props()

  let expanded = $state(false)
</script>

<div class="border-b border-gray-100 bg-white dark:border-gray-800 dark:bg-gray-900">
  <button
    type="button"
    class="flex w-full items-center gap-2 px-6 py-2 text-left text-sm hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors"
    onclick={() => { expanded = !expanded }}
    aria-expanded={expanded}
  >
    <Box class="h-3.5 w-3.5 shrink-0 text-gray-400" style="color: {project.color ?? '#9ca3af'}" />
    <span class="flex-1 font-medium text-gray-700 dark:text-gray-300">{project.name}</span>
    {#if project.description}
      <span class="max-w-xs truncate text-xs text-gray-400 dark:text-gray-500">{project.description}</span>
    {/if}
    {#if expanded}
      <ChevronDown class="h-3.5 w-3.5 shrink-0 text-gray-400" />
    {:else}
      <ChevronRight class="h-3.5 w-3.5 shrink-0 text-gray-400" />
    {/if}
  </button>

  {#if expanded}
    <div class="border-t border-gray-100 px-6 py-4 dark:border-gray-800">
      {#if authStore.role !== 'viewer'}
        <ProjectCustomFields projectId={project.id} />
      {:else}
        <p class="text-xs text-gray-400 dark:text-gray-500">Project details</p>
      {/if}
    </div>
  {/if}
</div>
