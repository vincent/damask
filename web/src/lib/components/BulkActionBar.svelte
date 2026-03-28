<script lang="ts">
  import { tagApi, type Project } from '$lib/api/client'
  import { authStore } from '$lib/stores/auth'

  interface Props {
    selectedIds: Set<string>
    projects: Project[]
    ondone: () => void
    onclear: () => void
  }

  let { selectedIds, projects, ondone, onclear }: Props = $props()

  let tagInput = $state('')
  let busy = $state(false)
  let showProjectPicker = $state(false)
  let showTagInput = $state(false)

  async function bulkTag() {
    const name = tagInput.trim().toLowerCase()
    if (!name || selectedIds.size === 0) return
    busy = true
    try {
      await tagApi.bulkTag([...selectedIds], name)
      tagInput = ''
      showTagInput = false
      ondone()
    } finally {
      busy = false
    }
  }

  async function bulkProject(projectId: string | null) {
    if (selectedIds.size === 0) return
    busy = true
    try {
      await tagApi.bulkProject([...selectedIds], projectId)
      showProjectPicker = false
      ondone()
    } finally {
      busy = false
    }
  }

  async function bulkDelete() {
    if (selectedIds.size === 0) return
    if (!confirm(`Delete ${selectedIds.size} asset${selectedIds.size > 1 ? 's' : ''}? This cannot be undone.`)) return
    busy = true
    try {
      await tagApi.bulkDelete([...selectedIds])
      ondone()
    } finally {
      busy = false
    }
  }
</script>

{#if selectedIds.size > 0}
  <div class="fixed bottom-6 left-1/2 z-30 -translate-x-1/2">
    <div class="flex items-center gap-3 rounded-xl border border-gray-200 bg-white px-4 py-2.5 shadow-xl">
      <span class="text-sm font-medium text-gray-700">{selectedIds.size} selected</span>
      <div class="h-5 w-px bg-gray-200"></div>

      <!-- Tag -->
      <div class="relative">
        <button
          class="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm font-medium text-gray-600 hover:bg-gray-100 disabled:opacity-50"
          disabled={busy}
          onclick={() => { showTagInput = !showTagInput; showProjectPicker = false }}
        >
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A2 2 0 013 12V7a4 4 0 014-4z" />
          </svg>
          Tag
        </button>
        {#if showTagInput}
          <form
            class="absolute bottom-full mb-2 flex gap-1"
            onsubmit={(e) => { e.preventDefault(); bulkTag() }}
          >
            <input
              autofocus
              bind:value={tagInput}
              placeholder="Tag name"
              class="rounded-lg border border-gray-300 px-2.5 py-1.5 text-sm shadow-lg focus:border-blue-500 focus:outline-none"
            />
            <button
              type="submit"
              class="rounded-lg bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
              disabled={busy}
            >Add</button>
          </form>
        {/if}
      </div>

      <!-- Assign project -->
      <div class="relative">
        <button
          class="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm font-medium text-gray-600 hover:bg-gray-100 disabled:opacity-50"
          disabled={busy}
          onclick={() => { showProjectPicker = !showProjectPicker; showTagInput = false }}
        >
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
          </svg>
          Project
        </button>
        {#if showProjectPicker}
          <div class="absolute bottom-full mb-2 min-w-[160px] rounded-xl border border-gray-200 bg-white py-1 shadow-lg">
            <button
              class="flex w-full items-center gap-2 px-3 py-1.5 text-sm text-gray-500 hover:bg-gray-50"
              onclick={() => bulkProject(null)}
            >
              <span class="h-2.5 w-2.5 rounded-full border border-gray-300"></span>
              None
            </button>
            {#each projects as p}
              <button
                class="flex w-full items-center gap-2 px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-50"
                onclick={() => bulkProject(p.id)}
              >
                <span
                  class="h-2.5 w-2.5 shrink-0 rounded-full"
                  style="background-color: {p.color.Valid ? p.color.String : '#9ca3af'}"
                ></span>
                <span class="truncate">{p.name}</span>
              </button>
            {/each}
          </div>
        {/if}
      </div>

      <div class="h-5 w-px bg-gray-200"></div>

      <!-- Delete (owner only) -->
      {#if $authStore.role === 'owner'}
        <button
          class="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm font-medium text-red-600 hover:bg-red-50 disabled:opacity-50"
          disabled={busy}
          onclick={bulkDelete}
        >
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
          </svg>
          Delete
        </button>
      {/if}

      <div class="h-5 w-px bg-gray-200"></div>

      <button
        class="text-sm text-gray-400 hover:text-gray-600"
        onclick={onclear}
      >Cancel</button>
    </div>
  </div>
{/if}
