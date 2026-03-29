<script lang="ts">
  import { assetApi, tagApi, variantApi, formatBytes, mimeCategory, type Asset, type Variant } from '$lib/api/client'
  import { authStore } from '$lib/stores/auth'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import { assetsStore } from '$lib/stores/assets.svelte'
  import VariantPanel from './VariantPanel.svelte'
  import { Cross, Download, Image, Loader, Mail, Play, Share, Trash } from '@lucide/svelte'

  interface Props {
    asset: Asset | null
    onclose: () => void
    ondeleted: (id: string) => void
    ontagschanged: () => void
    onprojectchanged: () => void
  }

  let { asset, onclose, ondeleted, ontagschanged, onprojectchanged }: Props = $props()

  let deleting = $state(false)
  let showVariantPanel = $state(false)
  let tags = $state<string[]>([])
  let tagInput = $state('')
  let tagSuggestions = $state<string[]>([])
  let showTagInput = $state(false)
  let showProjectPicker = $state(false)
  let allTags = $state<{ id: string; name: string; asset_count: number }[]>([])
  let variants = $state<Variant[]>([])

  const category = $derived(asset ? mimeCategory(asset.mime_type) : 'document')

  const previewBg: Record<string, string> = {
    image: '#E8C05A',
    video: '#E88A8A',
    audio: '#7CC89A',
    document: '#B8C8E8',
  }

  const typeBadge: Record<string, string> = {
    image: 'bg-violet-100 text-violet-700',
    video: 'bg-red-100 text-red-700',
    audio: 'bg-emerald-100 text-emerald-700',
    document: 'bg-gray-100 text-gray-600',
  }

  const typeLabel: Record<string, string> = {
    image: 'IMAGE',
    video: 'VIDEO',
    audio: 'AUDIO',
    document: 'DOCUMENT',
  }

  const TAG_COLORS = [
    'bg-violet-100 text-violet-700',
    'bg-blue-100 text-blue-700',
    'bg-gray-100 text-gray-600',
    'bg-emerald-100 text-emerald-700',
    'bg-amber-100 text-amber-700',
    'bg-rose-100 text-rose-700',
  ]

  $effect(() => {
    if (!asset) { tags = []; variants = []; return }
    tagApi.getForAsset(asset.id).then((t) => { tags = t }).catch(() => {})
    tagApi.list().then((t) => { allTags = t }).catch(() => {})
    variantApi.list(asset.id).then((v) => { variants = v }).catch(() => {})
  })

  function formatDate(iso: string) {
    return new Date(iso).toLocaleDateString('en-CA')
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
    if (!q) { tagSuggestions = []; return }
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
      ? projectsStore.projects.find((p) => p.id === asset.project_id.String) ?? null
      : null,
  )

  function copyShareLink() {
    if (!asset) return
    navigator.clipboard.writeText(assetApi.fileUrl(asset.id)).catch(() => {})
  }
</script>

<svelte:window onkeydown={handleKeydown} />

{#if asset}
  <!-- Backdrop -->
  <div
    class="fixed inset-0 z-40 bg-black/40 backdrop-blur-sm"
    role="button"
    tabindex="-1"
    onclick={onclose}
    onkeydown={(e) => e.key === 'Enter' && onclose()}
    aria-label="Close lightbox"
  ></div>

  <!-- Panel -->
  <div
    class="fixed inset-y-0 right-0 z-50 flex w-full max-w-xl flex-col bg-white shadow-2xl"
    role="dialog"
    aria-modal="true"
    aria-label={asset.original_filename}
  >
    <!-- Preview area with colored background -->
    <div
      class="relative flex min-h-52 items-center justify-center"
      style="background-color: {previewBg[category]}; background-image: url('{assetApi.thumbUrl(asset.id)}'); background-size: cover; background-position: center"
    >
      <div class="flex h-16 w-16 items-center justify-center rounded-2xl bg-white/90 shadow-sm">
        {#if category === 'video' || category === 'audio'}
          <Play class="h-8 w-8 text-gray-700" />
        {:else if category === 'image'}
          <Image class="h-8 w-8 text-gray-700" />
        {/if}
      </div>

      <div class="absolute right-3 top-3 flex items-center gap-1.5">
        {#if $authStore.role !== 'viewer'}
          <button
            type="button"
            class="flex h-8 w-8 items-center justify-center rounded-lg bg-white/20 text-white backdrop-blur-sm hover:bg-white/30"
            onclick={handleDelete}
            disabled={deleting}
            aria-label="Delete asset"
            title="Delete"
          >
            {#if deleting}
              <Loader class="h-4 w-4 animate-spin" />
            {:else}
              <Trash class="h-4 w-4" />
            {/if}
          </button>
        {/if}
        <button
          type="button"
          class="flex h-8 w-8 items-center justify-center rounded-lg bg-white/20 text-white backdrop-blur-sm hover:bg-white/30"
          onclick={onclose}
          aria-label="Close"
        >
          <Cross class="h-4 w-4" />
        </button>
      </div>
    </div>

    <!-- Scrollable content -->
    <div class="flex-1 overflow-y-auto">
      <!-- Section 1: type + size + filename + download -->
      <div class="px-5 pb-3 pt-4">
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0">
            <div class="mb-1.5 flex items-center gap-2">
              <span class="rounded-md px-2 py-0.5 text-xs font-semibold {typeBadge[category]}">
                {typeLabel[category]}
              </span>
              <span class="text-sm text-gray-500">{formatBytes(asset.size)}</span>
            </div>
            <h2 class="text-lg font-bold leading-tight text-gray-900" title={asset.original_filename}>
              {asset.original_filename}
            </h2>
          </div>
          <a
            href={assetApi.fileUrl(asset.id)}
            download={asset.original_filename}
            class="flex shrink-0 items-center justify-center rounded-lg bg-indigo-600 p-2.5 text-white hover:bg-indigo-700"
            aria-label="Download"
          >
            <Download class="h-4 w-4" />
          </a>
        </div>
      </div>

      <!-- Section 2: 3-column metadata -->
      <div class="grid grid-cols-3 gap-4 border-b border-t border-gray-100 px-5 py-4">
        <div>
          <p class="mb-1 text-[10px] font-semibold uppercase tracking-widest text-gray-400">Created By</p>
          <p class="text-sm font-medium text-gray-900">—</p>
        </div>
        <div>
          <p class="mb-1 text-[10px] font-semibold uppercase tracking-widest text-gray-400">Date Added</p>
          <p class="text-sm font-medium text-gray-900">{formatDate(asset.created_at)}</p>
        </div>
        <div>
          <p class="mb-1 text-[10px] font-semibold uppercase tracking-widest text-gray-400">Dimensions</p>
          <p class="text-sm font-medium text-gray-900">
            {#if asset.width.Valid && asset.height.Valid}
              {asset.width.Int64} × {asset.height.Int64}
            {:else}
              —
            {/if}
          </p>
        </div>
      </div>

      <!-- Section 3: Tags -->
      <div class="border-b border-gray-100 px-5 py-4">
        <h3 class="mb-2.5 text-sm font-semibold text-gray-900">Tags</h3>
        <div class="flex flex-wrap gap-1.5">
          {#each tags as tag, i}
            <span class="flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-medium {TAG_COLORS[i % TAG_COLORS.length]}">
              {tag}
              {#if $authStore.role !== 'viewer'}
                <button
                  class="ml-0.5 rounded-full p-0.5 opacity-60 hover:opacity-100"
                  onclick={() => removeTag(tag)}
                  aria-label="Remove tag {tag}"
                >
                  <Cross class="h-3 w-3" />
                </button>
              {/if}
            </span>
          {/each}

          {#if $authStore.role !== 'viewer'}
            {#if showTagInput}
              <div class="relative">
                <form onsubmit={(e) => { e.preventDefault(); addTag(tagInput) }}>
                  <input
                    bind:value={tagInput}
                    oninput={updateSuggestions}
                    placeholder="Add tag…"
                    class="w-28 rounded-full border border-indigo-400 px-2.5 py-0.5 text-xs outline-none"
                    onblur={() => { setTimeout(() => { showTagInput = false; tagSuggestions = [] }, 150) }}
                  />
                </form>
                {#if tagSuggestions.length > 0}
                  <ul class="absolute left-0 top-full z-20 mt-0.5 w-36 rounded-lg border border-gray-200 bg-white py-1 shadow-md">
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
                class="rounded-full border border-dashed border-gray-300 px-2.5 py-0.5 text-xs text-gray-400 hover:border-indigo-400 hover:text-indigo-600"
                onclick={() => { showTagInput = true }}
              >+ Add Tag</button>
            {/if}
          {/if}
        </div>
      </div>

      <!-- Section 4: Project -->
      {#if $authStore.role !== 'viewer' || activeProject}
        <div class="border-b border-gray-100 px-5 py-4">
          <div class="flex items-center justify-between">
            <h3 class="text-sm font-semibold text-gray-900">Folder</h3>
            {#if $authStore.role !== 'viewer'}
              <button
                class="text-xs text-indigo-600 hover:underline"
                onclick={() => { showProjectPicker = !showProjectPicker }}
              >
                {activeProject ? 'Change' : 'Assign'}
              </button>
            {/if}
          </div>
          <div class="relative mt-2">
            {#if activeProject}
              <span
                class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium"
                style="background-color: {activeProject.color.Valid ? activeProject.color.String + '22' : '#f3f4f6'}; color: {activeProject.color.Valid ? activeProject.color.String : '#6b7280'}"
              >
                <span class="h-2 w-2 rounded-full" style="background-color: {activeProject.color.Valid ? activeProject.color.String : '#9ca3af'}"></span>
                {activeProject.name}
              </span>
            {:else}
              <span class="text-xs text-gray-400">Not assigned</span>
            {/if}

            {#if showProjectPicker}
              <div class="absolute left-0 top-full z-20 mt-1 min-w-[180px] rounded-xl border border-gray-200 bg-white py-1 shadow-lg">
                <button
                  class="flex w-full items-center gap-2 px-3 py-1.5 text-sm text-gray-500 hover:bg-gray-50"
                  onclick={() => assignProject(null)}
                >
                  <span class="h-2.5 w-2.5 rounded-full border border-gray-300"></span>
                  None
                </button>
                {#each projectsStore.projects as p}
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
      {/if}

      <!-- Section 5: Variants -->
      <div class="border-b border-gray-100 px-5 py-4">
        <div class="mb-3 flex items-center justify-between">
          <h3 class="text-sm font-semibold text-gray-900">Variants</h3>
          <button
            class="text-xs text-indigo-600 hover:underline"
            onclick={() => { showVariantPanel = true }}
          >View All</button>
        </div>
        {#if variants.length > 0}
          <div class="flex gap-2">
            {#each variants.slice(0, 2) as variant, i}
              <a
                href={variantApi.fileUrl(asset.id, variant.id)}
                download
                class="group flex flex-col overflow-hidden rounded-lg border border-gray-200 hover:border-indigo-300"
                style="width: 100px"
              >
                <div class="flex h-16 items-center justify-center bg-gray-100 text-xs text-gray-400">
                  v{i + 1}.0
                </div>
                <div class="px-2 py-1">
                  <p class="truncate text-[10px] text-gray-500">{variant.type}</p>
                  <p class="text-[10px] text-gray-400">{formatDate(variant.created_at)}</p>
                </div>
              </a>
            {/each}
          </div>
        {:else}
          <p class="text-xs text-gray-400">No variants yet. <button class="text-indigo-600 hover:underline" onclick={() => { showVariantPanel = true }}>Create one</button></p>
        {/if}
      </div>

      <!-- Section 6: Quick Actions -->
      <div class="border-b border-gray-100 px-5 py-4">
        <h3 class="mb-2.5 text-sm font-semibold text-gray-900">Quick Actions</h3>
        <div class="space-y-2">
          <button
            class="flex w-full items-center gap-2.5 rounded-lg border border-gray-200 px-3 py-2.5 text-sm text-gray-700 hover:bg-gray-50"
            onclick={copyShareLink}
          >
            <Share class="h-4 w-4 shrink-0 text-gray-400" />
            Copy Share Link
          </button>
          <button
            class="flex w-full items-center gap-2.5 rounded-lg border border-gray-200 px-3 py-2.5 text-sm text-gray-700 hover:bg-gray-50"
            onclick={() => { window.location.href = `mailto:?subject=Shared asset&body=${assetApi.fileUrl(asset!.id)}` }}
          >
            <Mail class="h-4 w-4 shrink-0 text-gray-400" />
             Send via Email
          </button>
        </div>
      </div>

      <!-- Section 7: Export Format -->
      <div class="px-5 py-4">
        <p class="mb-2 text-[10px] font-semibold uppercase tracking-widest text-gray-400">Export Format</p>
        <div class="flex gap-2">
          <select class="flex-1 rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-gray-700 focus:border-indigo-400 focus:outline-none">
            <option>Original</option>
            <option>JPEG</option>
            <option>PNG</option>
            <option>WebP</option>
            <option>PDF</option>
          </select>
          <a
            href={assetApi.fileUrl(asset.id)}
            download={asset.original_filename}
            class="flex items-center justify-center rounded-lg bg-gray-900 px-3 py-2 text-white hover:bg-gray-800"
            aria-label="Download"
          >
            <Download class="h-4 w-4" />
          </a>
        </div>
      </div>
    </div>
  </div>

  {#if showVariantPanel}
    <VariantPanel {asset} onclose={() => { showVariantPanel = false }} />
  {/if}
{/if}
