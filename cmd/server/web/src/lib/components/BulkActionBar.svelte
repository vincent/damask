<script lang="ts">
  import { tagApi, assetApi, type Project } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { SquareArrowRightExit, Tag, Trash } from '@lucide/svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import ButtonDelete from './ui/ButtonDelete.svelte'

  interface Props {
    selectedIds: Set<string>
    projects: Project[]
    ondone: () => void
    onclear: () => void
  }

  let { selectedIds, projects, ondone, onclear }: Props = $props()

  let tagInput = $state('')
  let busy = $state(false)
  let activePanel = $state<'tags' | 'projects' | null>(null)

  async function bulkTag() {
    const name = tagInput.trim().toLowerCase()
    if (!name || selectedIds.size === 0) return
    busy = true
    try {
      await tagApi.bulkTag([...selectedIds], name)
      tagInput = ''
      ondone()
    } finally {
      activePanel = null
      busy = false
    }
  }

  async function bulkProject(projectId: string | null) {
    if (selectedIds.size === 0) return
    busy = true
    try {
      await assetApi.bulkProject([...selectedIds], projectId)
      ondone()
    } finally {
      activePanel = null
      busy = false
    }
  }

  async function bulkDelete() {
    if (selectedIds.size === 0) return
    if (!confirm(`Delete ${selectedIds.size} asset${selectedIds.size > 1 ? 's' : ''}? This cannot be undone.`)) return
    busy = true
    try {
      await assetApi.bulkDelete([...selectedIds])
      ondone()
    } finally {
      busy = false
    }
  }
</script>

{#if selectedIds.size > 0}
  <div class="fixed bottom-6 left-1/2 z-30 -translate-x-1/2">
    <div class="flex items-center gap-3 rounded-xl border border-gray-200 bg-white px-4 py-2.5 shadow-xl dark:border-gray-700 dark:bg-gray-900">
      <span class="text-md font-medium text-gray-700 dark:text-gray-300">{selectedIds.size} selected</span>
      <div class="h-5 w-px bg-gray-200 dark:bg-gray-700"></div>

      <!-- Tag -->
      <div class="relative">
        <Button
          variant="ghost"
          size="sm"
          disabled={busy}
          onclick={() => { activePanel = activePanel === 'tags' ? null : 'tags' }}
        >
          {#snippet icon()}<Tag class="h-4 w-4" />{/snippet}
          Tag
        </Button>
        {#if activePanel === 'tags'}
          <form
            class="absolute bottom-full mb-2 flex gap-1"
            onsubmit={(e) => { e.preventDefault(); bulkTag() }}
          >
            <input
              bind:value={tagInput}
              placeholder="Tag name"
              class="rounded-lg border border-gray-300 bg-white px-2.5 py-1.5 text-md shadow-lg focus:border-blue-500 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
            />
            <Button type="submit" variant="primary" size="sm" disabled={busy}>Add</Button>
          </form>
        {/if}
      </div>

      <!-- Assign project -->
      <div class="relative">
        <Button
          variant="ghost"
          size="sm"
          disabled={busy}
          onclick={() => { activePanel = activePanel === 'projects' ? null : 'projects' }}
        >
          {#snippet icon()}<SquareArrowRightExit class="h-4 w-4" />{/snippet}
          Project
        </Button>
        {#if activePanel === 'projects'}
          <div class="absolute bottom-full mb-2 min-w-[160px] rounded-xl border border-gray-200 bg-white py-1 shadow-lg dark:border-gray-700 dark:bg-gray-900">
            <button
              class="flex w-full items-center gap-2 px-3 py-1.5 text-md text-gray-500 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800"
              onclick={() => bulkProject(null)}
            >
              <span class="h-2.5 w-2.5 rounded-full border border-gray-300"></span>
              None
            </button>
            {#each projects as p}
              <button
                class="flex w-full items-center gap-2 px-3 py-1.5 text-md text-gray-700 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-gray-800"
                onclick={() => bulkProject(p.id)}
              >
                <span
                  class="h-2.5 w-2.5 shrink-0 rounded-full"
                  style="background-color: {p.color ?? '#9ca3af'}"
                ></span>
                <span class="truncate">{p.name}</span>
              </button>
            {/each}
          </div>
        {/if}
      </div>

      <div class="h-5 w-px bg-gray-200 dark:bg-gray-700"></div>

      <!-- Delete (owner only) -->
      {#if authStore.role === 'owner'}
        <ButtonDelete title="Bulk Delete variants" onclick={bulkDelete} />
      {/if}

      <div class="h-5 w-px bg-gray-200 dark:bg-gray-700"></div>

      <Button variant="ghost" size="sm" onclick={onclear}>Cancel</Button>
    </div>
  </div>
{/if}
