<script lang="ts">
  import type { Snippet } from 'svelte'

  interface Props {
    title: string
    count?: number
    accent?: string
    icon?: Snippet
  }

  let { title, count, accent, icon }: Props = $props()
</script>

<div class="mb-4 flex items-center gap-3">
  {#if icon}
    <div
      class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg"
      style={accent ? `background-color: ${accent}20; color: ${accent}` : ''}
    >
      {@render icon()}
    </div>
  {/if}
  <h2 class="text-sm font-semibold text-gray-900 dark:text-gray-100">{title}</h2>
  {#if count !== undefined}
    <span class="text-sm text-gray-400 dark:text-gray-500">{count}</span>
  {/if}
</div>
