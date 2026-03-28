<script lang="ts">
  import { assetApi, tagApi, formatBytes, mimeCategory, type Asset, type Project } from '$lib/api/client'
  import { authStore } from '$lib/stores/auth'

  interface Props {
    asset: Asset | null
    projects: Project[]
    onclose: () => void
    ondeleted: (id: string) => void
    ontagschanged: () => void
    onprojectchanged: () => void
  }

  let { asset, projects, onclose, ondeleted, ontagschanged, onprojectchanged }: Props = $props()

  let deleting = $state(false)
  let tags = $state<string[]>([])
  let tagInput = $state('')
  let tagSuggestions = $state<string[]>([])
  let showTagInput = $state(false)
  let showProjectPicker = $state(false)
  let allTags = $state<{ id: string; name: string; asset_count: number }[]>([])

  const category = $derived(asset ? mimeCategory(asset.mime_type) : 'document')

  // Load tags whenever the asset changes
  $effect(() => {
    if (!asset) { tags = []; return }
    tagApi.getForAsset(asset.id).then((t) => { tags = t }).catch(() => {})
    tagApi.list().then((t) => { allTags = t }).catch(() => {})
  })

  function formatDate(iso: string) {
    return new Date(iso).toLocaleDateString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    })
  }

  async function handleDelete() {
    if (!asset) return
    deleting = true
    try {
      await assetApi.delete(asset.id)
      ondeleted(asset.id)
    } finally {
      deleting = false
    }
  }

  async function addTag(name: string) {
    if (!asset || !name.trim()) return
    const n = name.trim().toLowerCase()
    if (tags.includes(n)) return
    try {
      await tagApi.addToAsset(asset.id, n)
      tags = [...tags, n]
      tagInput = ''
      showTagInput = false
      ontagschanged()
    } catch {
      // silently ignore
    }
  }

  async function removeTag(name: string) {
    if (!asset) return
    try {
      await tagApi.removeFromAsset(asset.id, name)
      tags = tags.filter((t) => t !== name)
      ontagschanged()
    } catch {
      // silently ignore
    }
  }

  async function assignProject(projectId: string | null) {
    if (!asset) return
    try {
      await tagApi.bulkProject([asset.id], projectId)
      showProjectPicker = false
      onprojectchanged()
    } catch {
      // silently ignore
    }
  }

  function updateSuggestions() {
    const q = tagInput.trim().toLowerCase()
    if (!q) {
      tagSuggestions = []
      return
    }
    tagSuggestions = allTags
      .map((t) => t.name)
      .filter((n) => n.includes(q) && !tags.includes(n))
      .slice(0, 5)
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') onclose()
  }

  const activeProject = $derived(
    asset?.project_id.Valid
      ? projects.find((p) => p.id === asset.project_id.String) ?? null
      : null
  )
</script>

<svelte:window onkeydown={handleKeydown} />

{#if asset}
  <!-- Backdrop -->
  <div
    class="fixed inset-0 z-40 bg-black/50 backdrop-blur-sm"
    role="button"
    tabindex="-1"
    onclick={onclose}
    onkeydown={(e) => e.key === 'Enter' && onclose()}
    aria-label="Close lightbox"
  ></div>

  <!-- Panel -->
  <div
    class="fixed inset-y-0 right-0 z-50 flex w-full max-w-lg flex-col bg-white shadow-2xl"
    role="dialog"
    aria-modal="true"
    aria-label={asset.original_filename}
  >
    <!-- Header -->
    <div class="flex items-center justify-between border-b border-gray-200 px-5 py-4">
      <h2 class="truncate text-base font-semibold text-gray-900" title={asset.original_filename}>
        {asset.original_filename}
      </h2>
      <button
        type="button"
        class="ml-3 shrink-0 rounded-md p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600"
        onclick={onclose}
        aria-label="Close"
      >
        <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>

    <!-- Preview -->
    <div class="flex items-center justify-center border-b border-gray-200 bg-gray-50 p-4" style="min-height: 280px">
      {#if category === 'image'}
        <img
          src={assetApi.fileUrl(asset.id)}
          alt={asset.original_filename}
          class="max-h-64 max-w-full rounded object-contain shadow"
        />
      {:else if category === 'video'}
        <video
          src={assetApi.fileUrl(asset.id)}
          controls
          class="max-h-64 max-w-full rounded shadow"
        >
          <track kind="captions" />
        </video>
      {:else}
        <div class="flex flex-col items-center gap-3 text-gray-400">
          <svg class="h-16 w-16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="1.5"
              d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z"
            />
          </svg>
          <span class="text-sm">{asset.mime_type}</span>
        </div>
      {/if}
    </div>

    <!-- Details scroll area -->
    <div class="flex-1 overflow-y-auto px-5 py-4 space-y-5">
      <!-- Project badge + picker -->
      <div>
        <h3 class="mb-2 text-xs font-semibold uppercase tracking-wide text-gray-500">Project</h3>
        <div class="relative flex items-center gap-2">
          {#if activeProject}
            <span
              class="flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium"
              style="background-color: {activeProject.color.Valid ? activeProject.color.String + '22' : '#f3f4f6'}; color: {activeProject.color.Valid ? activeProject.color.String : '#6b7280'}"
            >
              <span class="h-2 w-2 rounded-full" style="background-color: {activeProject.color.Valid ? activeProject.color.String : '#9ca3af'}"></span>
              {activeProject.name}
            </span>
          {:else}
            <span class="text-xs text-gray-400">None</span>
          {/if}
          {#if $authStore.role !== 'viewer'}
            <button
              class="ml-auto text-xs text-gray-400 hover:text-gray-600"
              onclick={() => { showProjectPicker = !showProjectPicker }}
            >
              {activeProject ? 'Change' : 'Assign'}
            </button>
          {/if}

          {#if showProjectPicker}
            <div class="absolute right-0 top-full mt-1 z-10 min-w-[180px] rounded-xl border border-gray-200 bg-white py-1 shadow-lg">
              <button
                class="flex w-full items-center gap-2 px-3 py-1.5 text-sm text-gray-500 hover:bg-gray-50"
                onclick={() => assignProject(null)}
              >
                <span class="h-2.5 w-2.5 rounded-full border border-gray-300"></span>
                None
              </button>
              {#each projects as p}
                <button
                  class="flex w-full items-center gap-2 px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-50"
                  onclick={() => assignProject(p.id)}
                >
                  <span
                    class="h-2.5 w-2.5 shrink-0 rounded-full"
                    style="background-color: {p.color.Valid ? p.color.String : '#9ca3af'}"
                  ></span>
                  {p.name}
                </button>
              {/each}
            </div>
          {/if}
        </div>
      </div>

      <!-- Tags -->
      <div>
        <h3 class="mb-2 text-xs font-semibold uppercase tracking-wide text-gray-500">Tags</h3>
        <div class="flex flex-wrap gap-1.5">
          {#each tags as tag}
            <span class="flex items-center gap-1 rounded-full bg-gray-100 px-2.5 py-0.5 text-xs font-medium text-gray-700">
              {tag}
              {#if $authStore.role !== 'viewer'}
                <button
                  class="ml-0.5 rounded-full p-0.5 hover:bg-gray-200"
                  onclick={() => removeTag(tag)}
                  aria-label="Remove tag {tag}"
                >
                  <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              {/if}
            </span>
          {/each}

          {#if $authStore.role !== 'viewer'}
            {#if showTagInput}
              <div class="relative">
                <form onsubmit={(e) => { e.preventDefault(); addTag(tagInput) }}>
                  <input
                    autofocus
                    bind:value={tagInput}
                    oninput={updateSuggestions}
                    placeholder="Add tag…"
                    class="w-28 rounded-full border border-blue-400 px-2.5 py-0.5 text-xs outline-none"
                    onblur={() => { setTimeout(() => { showTagInput = false; tagSuggestions = [] }, 150) }}
                  />
                </form>
                {#if tagSuggestions.length > 0}
                  <ul class="absolute left-0 top-full z-10 mt-0.5 w-36 rounded-lg border border-gray-200 bg-white py-1 shadow-md">
                    {#each tagSuggestions as s}
                      <li>
                        <button
                          class="w-full px-3 py-1 text-left text-xs text-gray-700 hover:bg-gray-50"
                          onmousedown={() => addTag(s)}
                        >{s}</button>
                      </li>
                    {/each}
                  </ul>
                {/if}
              </div>
            {:else}
              <button
                class="rounded-full border border-dashed border-gray-300 px-2.5 py-0.5 text-xs text-gray-400 hover:border-blue-400 hover:text-blue-600"
                onclick={() => { showTagInput = true }}
              >+ Tag</button>
            {/if}
          {/if}
        </div>
      </div>

      <!-- Metadata -->
      <div>
        <h3 class="mb-2 text-xs font-semibold uppercase tracking-wide text-gray-500">Details</h3>
        <dl class="space-y-2 text-sm">
          <div class="flex justify-between">
            <dt class="text-gray-500">Type</dt>
            <dd class="font-medium text-gray-900">{asset.mime_type}</dd>
          </div>
          <div class="flex justify-between">
            <dt class="text-gray-500">Size</dt>
            <dd class="font-medium text-gray-900">{formatBytes(asset.size)}</dd>
          </div>
          {#if asset.width.Valid && asset.height.Valid}
            <div class="flex justify-between">
              <dt class="text-gray-500">Dimensions</dt>
              <dd class="font-medium text-gray-900">{asset.width.Int64} × {asset.height.Int64}</dd>
            </div>
          {/if}
          <div class="flex justify-between">
            <dt class="text-gray-500">Uploaded</dt>
            <dd class="font-medium text-gray-900">{formatDate(asset.created_at)}</dd>
          </div>
        </dl>
      </div>
    </div>

    <!-- Actions -->
    <div class="flex gap-2 border-t border-gray-200 px-5 py-4">
      <a
        href={assetApi.fileUrl(asset.id)}
        download={asset.original_filename}
        class="flex flex-1 items-center justify-center gap-1.5 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
      >
        <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
        </svg>
        Download
      </a>
      {#if $authStore.role !== 'viewer'}
        <button
          type="button"
          class="flex flex-1 items-center justify-center gap-1.5 rounded-lg bg-red-50 px-4 py-2 text-sm font-medium text-red-600 hover:bg-red-100 disabled:opacity-50"
          onclick={handleDelete}
          disabled={deleting}
        >
          {#if deleting}
            <svg class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
            </svg>
          {:else}
            <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
            </svg>
          {/if}
          Delete
        </button>
      {/if}
    </div>
  </div>
{/if}
