<script lang="ts">
  import { authStore } from '$lib/stores/auth.svelte'
  import { assetsStore } from '$lib/stores/assets.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import SortButtons from '$lib/components/SortButtons.svelte'
  import SearchInput from '$lib/components/ui/SearchInput.svelte'
  import { Share2, Upload } from '@lucide/svelte'
  import { navigationStore } from '$lib/stores/navigation.svelte'

  type Props = {
    sort: 'mimetype' | 'created_at' | 'size'
    asc: boolean
    onShareProject?: () => void
    showShareButton?: boolean
  }

  let { sort = $bindable(), asc = $bindable(), onShareProject, showShareButton = false }: Props = $props()
</script>

<header class="flex items-center justify-between border-b border-gray-100 bg-white px-6 py-4 dark:border-gray-800 dark:bg-gray-900">
  <div class="flex items-center gap-3">
    <div>
      <h1 class="text-2xl font-bold text-gray-900 dark:text-gray-50">
        {projectsStore.activeProjectName ?? 'Library'}
      </h1>
      <p class="mt-0.5 text-md text-gray-400">
        All Assets{#if projectsStore.activeProjectName} / {projectsStore.activeProjectName}{/if}
      </p>
    </div>
    {#if showShareButton}
      <button
        type="button"
        class="flex items-center gap-1.5 rounded-lg border border-gray-200 px-3 py-1.5 text-md text-gray-600 transition-colors hover:border-indigo-300 hover:bg-indigo-50 hover:text-indigo-700 dark:border-gray-700 dark:text-gray-400 dark:hover:border-indigo-700 dark:hover:bg-indigo-900/20 dark:hover:text-indigo-400"
        onclick={onShareProject}
        title="Share this project"
      >
        <Share2 class="h-3.5 w-3.5" />
        Share
      </button>
    {/if}
  </div>

  <div class="flex items-center gap-2">
    <SortButtons
      bind:value={sort} bind:asc
      keys={{ created_at: 'date', mimetype: 'type', size: 'size' }}
      sort={(key, a) => assetsStore.sort(key, a)}
    />
  </div>

  <div class="flex items-center gap-2">
    <SearchInput
      class="w-64"
      value={assetsStore.query}
      placeholder="Search anything..."
      onchange={(v) => { assetsStore.query = v; assetsStore.search() }}
    />

    {#if authStore.role !== 'viewer'}
      <label class="flex items-center gap-2 cursor-pointer rounded-lg bg-indigo-600 px-3 py-2 text-md font-medium text-white hover:bg-indigo-700">
        Upload
        <Upload class="h-4 w-4" />
        <input
          type="file"
          multiple
          class="hidden"
          onchange={(e) => {
            const files = Array.from((e.target as HTMLInputElement).files ?? [])
            assetsStore.upload(files, navigationStore.activeProjectId, navigationStore.activeFolderId)
            ;(e.target as HTMLInputElement).value = ''
          }}
        />
      </label>
    {/if}
  </div>
</header>
