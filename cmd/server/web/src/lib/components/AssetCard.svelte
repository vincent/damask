<script lang="ts">
  import { assetApi, formatBytes, mimeCategory, type Asset } from '$lib/api'
  import { customFieldsStore } from '$lib/stores/customFields.svelte'
  import { File, Loader, Play, TriangleAlert } from '@lucide/svelte'
  import { ASSET_BACKGROUND_COLORS, DOT_COLORS } from '$lib/stores/shared'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    asset: Asset
    class?: string,
    zoom?: number,
    onclick: (e: MouseEvent) => void
    /** Set to true when this asset was just uploaded and may be missing required fields */
    requiresFields?: boolean
  }

  let { asset, class: extraClass = '', zoom = 5, onclick, requiresFields = false }: Props = $props()

  const hasRequiredFields = $derived(customFieldsStore.assetFields.some((f) => f.required))
  const showRequiredNudge = $derived(requiresFields && hasRequiredFields)
  const category = $derived(mimeCategory(asset.mime_type))
  const isProcessing = $derived(!asset.thumbnail_key)

  function formatDate(iso: string) {
    return new Date(iso).toLocaleDateString()
  }
</script>

<button
  type="button"
  draggable="true"
  class="asset-card group flex w-full flex-col rounded-lg bg-white text-left shadow-sm transition-shadow hover:shadow-md focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-400 dark:bg-gray-800 dark:shadow-none dark:hover:shadow-none dark:ring-inset dark:focus-visible:ring-indigo-500"
  onclick={(e) => onclick(e)}
  ondragstart={(e) => {
    e.dataTransfer?.setData('text/plain', asset.id)
    e.dataTransfer?.setData('application/json', JSON.stringify({ assetId: asset.id }))
  }}
>
  <!-- stack effect -->
  <div class="card-shadow {asset.variant_count > 0 ? 'block' : 'hidden'} absolute right-[-3px] top-[-3px] z-0 h-full w-full rounded-lg border-r-2 border-t-2 border-gray-300 dark:border-gray-700 rounded-br-lg [transition:top_150ms_ease,right_150ms_ease]"></div>
  <div class="card-shadow {asset.variant_count > 2 ? 'block' : 'hidden'} absolute right-[-3px] top-[-3px] z-0 h-full w-full rounded-lg border-r-2 border-t-2 border-gray-300 dark:border-gray-700 rounded-br-lg [transition:top_150ms_ease,right_150ms_ease]"></div>

  <!-- Thumbnail area -->
  <div
    class="damask-texture relative w-full overflow-hidden rounded-t-lg {ASSET_BACKGROUND_COLORS[category]}"
    style="aspect-ratio: 4/3"
  >
    <!-- Status dot -->
    <div class="absolute left-2.5 top-2.5 h-3 w-3 rounded-full {DOT_COLORS[category]}"></div>

    <!-- Required fields nudge -->
    {#if showRequiredNudge}
      <div class="absolute right-2 top-2 flex items-center gap-1 rounded-full bg-orange-500 px-1.5 py-0.5 text-white shadow"
           title="Required fields missing">
        <TriangleAlert class="h-3 w-3" />
        <span class="text-xs font-semibold leading-none">{m.fields()}</span>
      </div>
    {:else if asset.version_count > 1}
      <div class="absolute right-2 top-2 rounded-full bg-black/50 px-1.5 py-0.5 text-white backdrop-blur-sm"
           title="{asset.version_count} versions">
        <span class="text-xs font-bold leading-none">v{asset.version_count}</span>
      </div>
    {/if}

    {#if (category === 'image' || category === 'video' || category === 'audio' || category === 'document') && !isProcessing}
      <img
        src={assetApi.thumbUrl(asset.id)}
        alt={asset.original_filename}
        data-asset-dynamic-resource={asset.id}
        class="h-full w-full object-cover"
        loading="lazy"
        onerror={(e) => {
          ;(e.currentTarget as HTMLImageElement).style.display = 'none'
        }}
      />
    {:else}
      <!-- Centered icon -->
      <div class="flex h-full items-center justify-center">
        <div class="flex h-12 w-12 items-center justify-center rounded-lg bg-white/25">
          {#if category === 'video' || category === 'audio'}
            <Play class="h-7 w-7 text-white" />
          {:else}
            <File class="h-7 w-7 text-white" />
          {/if}
        </div>
      </div>
    {/if}

    {#if isProcessing}
      <div class="absolute inset-0 flex items-center justify-center bg-black/25">
        <div class="flex flex-col items-center gap-1.5 text-white">
          <Loader class="h-5 w-5 animate-spin" />
          <span class="text-sm font-medium">{m.processing()}</span>
        </div>
      </div>
    {/if}
  </div>

  <!-- Info -->
  <div class="flex flex-col gap-1.5 px-3 py-2.5 {zoom < 6 ? 'hidden' : ''}">
    <p class="truncate text-sm font-medium text-gray-900 dark:text-gray-100" title={asset.original_filename}>
      {asset.original_filename}
    </p>
    <div class="flex items-center justify-between">
      <span class="text-xs text-gray-400">{formatBytes(asset.size)}</span>
      <span class="text-xs text-gray-400">{formatDate(asset.created_at)}</span>
    </div>
    {#if asset.tags && asset.tags.length > 0}
      <div class="flex flex-wrap gap-1">
        {#each asset.tags.slice(0, 3) as tag}
          <span class="rounded bg-gray-100 px-1.5 py-0.5 text-[11px] text-gray-500 dark:bg-gray-700 dark:text-gray-400">{tag}</span>
        {/each}
      </div>
    {/if}
  </div>
</button>


<style>
  .card-shadow + .card-shadow {
    top: -6px;
    right: -6px;
  }
  .asset-card:hover .card-shadow {
    top: -4px;
    right: -4px;
  }
  .asset-card:hover .card-shadow + .card-shadow {
    top: -8px;
    right: -8px;
  }
</style>