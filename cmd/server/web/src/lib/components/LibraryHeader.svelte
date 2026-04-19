<script lang="ts">
  import { authStore } from '$lib/stores/auth.svelte'
  import { assetsStore } from '$lib/stores/assets.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import SortButtons from '$lib/components/SortButtons.svelte'
  import SearchInput from '$lib/components/ui/SearchInput.svelte'
  import { navigationStore } from '$lib/stores/navigation.svelte'
  import { Share2, Upload } from '@lucide/svelte'
  import Button from './ui/Button.svelte'
  import Title from './ui/Title.svelte'
  import type { Snippet } from 'svelte'
  import Hint from './ui/Hint.svelte'
  import { m } from '$lib/paraglide/messages.js';

  type Props = {
    prefix?: Snippet
    sort: 'mimetype' | 'created_at' | 'size' | 'taken_at'
    asc: boolean
    onShareProject?: () => void
    showShareButton?: boolean
  }

  let { sort = $bindable(), asc = $bindable(), onShareProject, showShareButton = false, prefix }: Props = $props()
</script>

<header class="flex items-center justify-between border-b border-gray-100 bg-white px-6 py-4 dark:border-gray-800 dark:bg-gray-900">
  <div class="flex items-center gap-3">
    {@render prefix?.()}
    <div>
      <Title>{projectsStore.activeProjectName ?? m.library()}</Title>
      <Hint>
        {m.all_assets()}{#if projectsStore.activeProjectName}&nbsp;/&nbsp;{projectsStore.activeProjectName}{/if}
      </Hint>
    </div>
    {#if showShareButton}
      <Button
        size="md"
        variant="outline"
        class="ms-3 flex items-center gap-1.5"
        onclick={onShareProject}
        title="Share this project"
      >
        <Share2 class="h-3.5 w-3.5" />
        {m.share()}
      </Button>
    {/if}
  </div>

  <div class="flex items-center gap-2">
    <SortButtons
      bind:value={sort} bind:asc
      keys={{ created_at: 'date', mimetype: 'type', size: 'size', taken_at: 'taken date' }}
      sort={(key, a) => assetsStore.sort(key, a)}
    />
  </div>

  <div class="flex items-center gap-2">
    <SearchInput
      class="w-64"
      value={assetsStore.query}
      placeholder={m.search_anything()}
      onchange={(q) => { assetsStore.search(q) }}
    />

    {#if authStore.role !== 'viewer'}
      <label class="flex items-center gap-2 cursor-pointer rounded-lg bg-indigo-600 px-3 py-2 text-md font-medium text-white hover:bg-indigo-700">
        {m.upload()}
        <Upload class="h-4 w-4" />
        <input
          type="file"
          multiple
          data-upload-trigger
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
