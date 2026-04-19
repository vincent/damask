<script lang="ts">
  import { formatKey } from '$lib/shortcuts'
  import { DEFAULT_KEYMAP } from '$lib/shortcuts'
  import { keymap } from '$lib/stores/shortcuts'
  import type { ShortcutAction } from '$lib/shortcuts'

  interface Props {
    action: ShortcutAction
    index?: number
  }

  let { action, index = 0 }: Props = $props()

  const combos = $derived($keymap[action] ?? DEFAULT_KEYMAP[action] ?? [])
  const combo = $derived(combos[index] ?? '')
  const keys = $derived(combo.split('+').map(formatKey))
</script>

{#if combo}
  <span class="inline-flex items-center gap-0.5 opacity-60 text-xs font-mono" aria-label="Keyboard shortcut: {combo}">
    {#each keys as key, i}
      <kbd class="inline-block px-1 py-px border border-zinc-300 dark:border-zinc-600 rounded bg-zinc-100 dark:bg-zinc-800 text-zinc-700 dark:text-zinc-300 text-xs font-mono leading-tight">{key}</kbd>{#if i < keys.length - 1}<span class="text-zinc-400">+</span>{/if}
    {/each}
  </span>
{/if}
