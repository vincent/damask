<script lang="ts">
  import { tagApi, type Tag } from '$lib/api'
  import Chip from '$lib/components/ui/Chip.svelte'

  interface Props {
    activeTags: string[]
    onchange: (tags: string[]) => void
  }

  let { activeTags, onchange }: Props = $props()

  let allTags = $state<Tag[]>([])

  async function loadTags() {
    try {
      allTags = await tagApi.list()
    } catch {
      // silently ignore
    }
  }

  $effect(() => {
    loadTags()
  })

  function toggle(name: string, event: MouseEvent) {
    if (event.shiftKey) {
      // Multi-select AND
      if (activeTags.includes(name)) {
        onchange(activeTags.filter((t) => t !== name))
      } else {
        onchange([...activeTags, name])
      }
    } else {
      // Single select — toggle off if already the only active
      if (activeTags.length === 1 && activeTags[0] === name) {
        onchange([])
      } else {
        onchange([name])
      }
    }
  }

  function dismiss(name: string) {
    onchange(activeTags.filter((t) => t !== name))
  }
</script>

{#if allTags.length > 0}
  <div class="flex flex-wrap items-center gap-1.5 border-t border-gray-100 bg-white dark:bg-gray-900 dark:border-gray-800 px-6 py-2">
    <!-- Active chips -->
    {#each activeTags as tag}
      <Chip label={tag} onremove={() => dismiss(tag)} color="#3b82f6" />
    {/each}

    {#if activeTags.length > 0}
      <span class="text-sm text-gray-300">|</span>
    {/if}

    <!-- All available tags -->
    {#each allTags as tag}
      {#if !activeTags.includes(tag.name)}
        <button
          class="rounded-full border px-3 py-1 text-sm transition-colors border-indigo-500 bg-indigo-50 text-indigo-700 dark:border-indigo-400 dark:bg-indigo-950/40 dark:text-indigo-300"
          onclick={(e) => toggle(tag.name, e)}
          title="Shift+click to add to filter"
        >
          {tag.name}
          <span class="ml-1 text-gray-400">{tag.asset_count}</span>
        </button>
      {/if}
    {/each}

    {#if activeTags.length > 0}
      <button
        class="ml-1 text-sm text-gray-400 hover:text-gray-600"
        onclick={() => onchange([])}
      >Clear</button>
    {/if}
  </div>
{/if}
