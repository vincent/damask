<script lang="ts">
  import { tagApi, type Asset, type Project } from '$lib/api/client'
  import { authStore } from '$lib/stores/auth.svelte'
  import Chip from '$lib/components/ui/Chip.svelte'
  import ColorDot from './ui/ColorDot.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  
  interface Props {
    asset: Asset
    activeProject: Project | null
    onprojectchanged: () => void
  }

  let { asset, activeProject, onprojectchanged }: Props = $props()

  let showProjectPicker = $state(false)

  async function assignProject(projectId: string | null) {
    if (!asset) return
    try {
      await tagApi.bulkProject([asset.id], projectId)
      showProjectPicker = false
      onprojectchanged()
    } catch { /* silently ignore */ }
  }

</script>

<div>
    <div class="mb-3 flex items-center justify-between">
    <p class="text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">Folder</p>
    {#if authStore.role !== 'viewer'}
        <button
        class="text-xs text-indigo-600 hover:underline dark:text-indigo-400"
        onclick={() => { showProjectPicker = !showProjectPicker }}
        >
        {activeProject ? 'Change' : 'Assign'}
        </button>
    {/if}
    </div>
    <div class="relative">
        {#if activeProject}
            <span
                class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium"
                style="background-color: {activeProject.color.Valid ? activeProject.color.String + '22' : '#f3f4f6'}; color: {activeProject.color.Valid ? activeProject.color.String : '#6b7280'}"
            >
                <ColorDot color={activeProject.color.Valid ? activeProject.color.String : '#9ca3af'} size="sm" />
                {activeProject.name}
            </span>
        {:else}
            <span class="text-xs text-gray-400 dark:text-gray-500">Not assigned</span>
        {/if}

        {#if showProjectPicker}
            <div class="absolute left-0 top-full z-20 mt-1 min-w-[180px] rounded-xl border border-gray-200 bg-white py-1 shadow-lg dark:border-gray-700 dark:bg-gray-900">
                <button
                    class="flex w-full items-center gap-2 px-3 py-1.5 text-sm text-gray-500 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800"
                    onclick={() => assignProject(null)}
                >
                    <span class="h-2.5 w-2.5 rounded-full border border-gray-300"></span>
                    None
                </button>
                {#each projectsStore.projects as p}
                    <button
                        class="flex w-full items-center gap-2 px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-gray-800"
                        onclick={() => assignProject(p.id)}
                    >
                        <ColorDot color={p.color.Valid ? p.color.String : '#9ca3af'} size="sm" />
                        {p.name}
                    </button>
                {/each}
            </div>
        {/if}
    </div>
</div>