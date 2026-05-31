<script lang="ts">
  import { tagApi, type Asset } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import Chip from '$lib/components/ui/Chip.svelte'
  import SubSectionTitle from '$lib/components/ui/SubSectionTitle.svelte'
  import { m } from '$lib/paraglide/messages'
  import { undoStore } from '$lib/stores/undo.svelte'
  import { TagAsset } from '$lib/commands/TagAsset'
  import { assetsStore } from '$lib/stores/assets.svelte'

  interface Props {
    asset: Asset | null
  }

  let { asset }: Props = $props()

  // --- Asset state ---
  let tagInput = $state('')
  let tagSuggestions = $state<string[]>([])
  let showTagInput = $state(false)
  let allTags = $state<{ id: string; name: string; asset_count: number }[]>([])
  let tagError = $state('')

  const tags = $derived(
    (asset ? assetsStore.assets.find((a) => a.id === asset.id) : null)?.tags ??
      asset?.tags ??
      []
  )

  const noMatch = $derived(
    tagInput.trim().length > 0 && tagSuggestions.length === 0
  )
  const showCreate = $derived(noMatch && authStore.canCreateTag)
  const showVocabHint = $derived(
    noMatch && authStore.taxonomyLocked && !authStore.canCreateTag
  )

  function updateSuggestions() {
    const q = tagInput.trim().toLowerCase()
    tagError = ''
    if (!q) {
      tagSuggestions = []
      return
    }
    tagSuggestions = allTags
      .map((t) => t.name)
      .filter((n) => n.includes(q) && !tags.includes(n))
      .slice(0, 5)
  }

  async function addTag(name: string) {
    if (!asset || !name.trim()) return
    const n = name.trim().toLowerCase()
    if (tags.includes(n)) return
    tagError = ''
    try {
      await undoStore.execute(new TagAsset(asset.id, n, 'add'))
      tagInput = ''
      showTagInput = false
    } catch (e) {
      if (e instanceof Error && e.message.includes('tag_not_in_vocabulary')) {
        tagError = m.tag_vocab_locked_api_error()
      }
    }
  }

  async function removeTag(name: string) {
    if (!asset) return
    try {
      await undoStore.execute(new TagAsset(asset.id, name, 'remove'))
    } catch {
      /* silently ignore */
    }
  }

  $effect(() => {
    if (!asset) return
    tagApi
      .list()
      .then((t) => {
        allTags = t
      })
      .catch(() => {})
  })
</script>

<div>
  <SubSectionTitle>{m.tags()}</SubSectionTitle>
  <div class="flex flex-wrap gap-1.5">
    {#each tags as tag}
      <Chip
        label={tag}
        onremove={authStore.role !== 'viewer'
          ? () => removeTag(tag)
          : undefined}
      />
    {/each}

    {#if authStore.role !== 'viewer'}
      {#if showTagInput}
        <div
          class="relative"
          onfocusout={(e) => {
            if (!e.currentTarget.contains(e.relatedTarget as Node)) {
              showTagInput = false
              tagSuggestions = []
            }
          }}
        >
          <form
            onsubmit={(e) => {
              e.preventDefault()
              addTag(tagInput)
            }}
          >
            <input
              bind:value={tagInput}
              oninput={updateSuggestions}
              placeholder="Add tag…"
              class="w-28 rounded-full border border-indigo-400 bg-white px-2.5 py-0.5 text-sm text-gray-900 outline-none dark:bg-gray-800 dark:text-gray-100"
            />
          </form>
          {#if tagSuggestions.length > 0 || showVocabHint}
            <ul
              class="absolute top-full left-0 z-20 mt-0.5 w-48 rounded-lg border border-gray-200 bg-white py-1 shadow-md dark:border-gray-700 dark:bg-gray-900"
            >
              {#each tagSuggestions as s}
                <li>
                  <button
                    class="w-full px-3 py-1 text-left text-sm text-gray-700 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-gray-800"
                    onmousedown={() => addTag(s)}>{s}</button
                  >
                </li>
              {/each}
              {#if showCreate}
                <li>
                  <button
                    class="w-full px-3 py-1 text-left text-sm text-indigo-600 hover:bg-gray-50 dark:text-indigo-400 dark:hover:bg-gray-800"
                    onmousedown={() => addTag(tagInput.trim())}
                    >+ {m.tag_new()} "{tagInput.trim()}"</button
                  >
                </li>
              {/if}
              {#if showVocabHint}
                <li
                  class="flex items-center gap-1.5 px-3 py-1 text-sm text-[var(--text-muted)]"
                >
                  🔒 {m.tag_autocomplete_not_in_vocab()}
                </li>
              {/if}
            </ul>
          {/if}
          {#if tagError}
            <p
              class="absolute top-full left-0 mt-1 w-56 rounded bg-red-50 px-2 py-1 text-xs text-red-600 dark:bg-red-900/20 dark:text-red-400"
            >
              {tagError}
            </p>
          {/if}
        </div>
      {:else}
        <button
          class="rounded-full border border-dashed border-gray-300 px-2.5 py-0.5 text-sm text-gray-400 hover:border-indigo-400 hover:text-indigo-600 dark:border-gray-600 dark:text-gray-500 dark:hover:border-indigo-500 dark:hover:text-indigo-400"
          onclick={() => {
            showTagInput = true
          }}>+ {m.tag_new()}</button
        >
      {/if}
    {/if}
  </div>
</div>
