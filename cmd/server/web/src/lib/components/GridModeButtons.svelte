<script lang="ts">
  import type { GridMode } from '$lib/stores/bottomStatusBar.svelte'
  import { Grid3X3, LayoutGrid, Rows3 } from '@lucide/svelte'

  let { mode, onchange }: { mode: GridMode; onchange: (m: GridMode) => void } =
    $props()
</script>

<div class="ml-auto flex items-center gap-0.5">
  <button
    type="button"
    aria-label="Spaced grid"
    aria-pressed={mode === 'spaced'}
    onclick={() => onchange('spaced')}
    class="rounded p-1 transition-colors {mode === 'spaced'
      ? 'bg-[var(--accent-soft)] text-[var(--accent)]'
      : 'text-[var(--text-muted)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-secondary)]'}"
  >
    <LayoutGrid class="h-4 w-4" />
  </button>
  <button
    type="button"
    aria-label="Compact grid"
    aria-pressed={mode === 'compact'}
    onclick={() => onchange('compact')}
    class="rounded p-1 transition-colors {mode === 'compact'
      ? 'bg-[var(--accent-soft)] text-[var(--accent)]'
      : 'text-[var(--text-muted)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-secondary)]'}"
  >
    <Grid3X3 class="h-4 w-4" />
  </button>
  <button
    type="button"
    aria-label="Table view"
    aria-pressed={mode === 'table'}
    onclick={() => onchange('table')}
    class="rounded p-1 transition-colors {mode === 'table'
      ? 'bg-[var(--accent-soft)] text-[var(--accent)]'
      : 'text-[var(--text-muted)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-secondary)]'}"
  >
    <Rows3 class="h-4 w-4" />
  </button>
</div>
