<script lang="ts">
  import type { VisualSimilarResult } from '$lib/api'
  import { assetApi } from '$lib/api'
  import { m } from '$lib/paraglide/messages'
  import { ArrowLeft, Images } from '@lucide/svelte'
  import { fly } from 'svelte/transition'
  import { cubicOut } from 'svelte/easing'

  interface Props {
    results: VisualSimilarResult[]
    onBack: () => void
    onSelect: (result: VisualSimilarResult) => void
  }

  let { results, onBack, onSelect }: Props = $props()

  const count = $derived(results.length)
  const countLabel = $derived(m.result_count({ count }))
</script>

<div
  class="flex h-full flex-col overflow-hidden"
  in:fly={{ x: 24, duration: 200, easing: cubicOut }}
>
  <!-- Header -->
  <div
    class="flex items-center gap-3 border-b border-gray-200 px-4 py-3 dark:border-gray-700"
  >
    <button
      type="button"
      class="flex items-center gap-1.5 text-sm text-gray-500 transition-colors hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-100"
      onclick={onBack}
    >
      <ArrowLeft class="h-4 w-4" />
      {m.back()}
    </button>
    <div class="flex-1">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
        {m.results_title()}
      </h3>
      <p class="text-xs text-gray-500 dark:text-gray-400">{countLabel}</p>
    </div>
  </div>

  <!-- Results list -->
  <div class="flex-1 overflow-y-auto">
    {#each results as result (result.asset_version_id)}
      <button
        type="button"
        class="flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-gray-50 dark:hover:bg-gray-800"
        onclick={() => onSelect(result)}
      >
        <!-- Thumbnail -->
        <div
          class="h-12 w-12 shrink-0 overflow-hidden rounded-lg bg-gray-100 dark:bg-gray-800"
        >
          {#if result.thumbnail_url}
            <img
              src={result.thumbnail_url}
              alt={result.original_filename}
              class="h-full w-full object-cover"
            />
          {:else}
            <div class="flex h-full w-full items-center justify-center">
              <Images class="h-5 w-5 text-gray-400" />
            </div>
          {/if}
        </div>

        <!-- Info -->
        <div class="min-w-0 flex-1">
          <p
            class="truncate text-sm font-medium text-gray-900 dark:text-gray-100"
          >
            {result.original_filename}
          </p>
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {#if result.width && result.height}
              {result.width} × {result.height} ·
            {/if}
            {result.mime_type.split('/')[1]?.toUpperCase() ?? result.mime_type}
          </p>
        </div>
      </button>
    {/each}
  </div>
</div>
