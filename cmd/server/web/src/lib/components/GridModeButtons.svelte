<script lang="ts">
  import type { GridMode } from '$lib/stores/bottomStatusBar.svelte'
  import { Grid3X3, LayoutGrid, Rows3 } from '@lucide/svelte'

  let { mode, onchange }: { mode: GridMode; onchange: (m: GridMode) => void } =
    $props()
</script>

<div class="ml-auto flex gap-2">
  <LayoutGrid
    onclick={() => onchange('spaced')}
    class="h-6 w-6 transition-colors hover:text-indigo-700 dark:hover:text-indigo-400 {mode ===
    'spaced'
      ? 'text-indigo-600 dark:text-indigo-400'
      : 'text-gray-400 dark:text-gray-600'}"
  />
  <Grid3X3
    onclick={() => onchange('compact')}
    class="h-6 w-6 transition-colors hover:text-indigo-700 dark:hover:text-indigo-400 {mode ===
    'compact'
      ? 'text-indigo-600 dark:text-indigo-400'
      : 'text-gray-400 dark:text-gray-600'}"
  />
  <Rows3
    onclick={() => onchange('table')}
    class="h-6 w-6 transition-colors hover:text-indigo-700 dark:hover:text-indigo-400 {mode ===
    'table'
      ? 'text-indigo-600 dark:text-indigo-400'
      : 'text-gray-400 dark:text-gray-600'}"
  />
</div>
