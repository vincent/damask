<script lang="ts">
  import { assetApi, formatBytes, mimeCategory, type Asset } from '$lib/api'
  import { File, Loader, Play } from '@lucide/svelte'

  interface Props {
    asset: Asset
    class?: string,
    zoom?: number,
    onclick: (e: MouseEvent) => void
  }

  let { asset, class: extraClass = '', zoom = 5, onclick }: Props = $props()
  const category = $derived(mimeCategory(asset.mime_type))
  const isProcessing = $derived(!asset.thumbnail_key)

  const cardBg: Record<string, string> = {
    image: 'bg-violet-300 dark:bg-violet-700',
    video: 'bg-red-300 dark:bg-red-700',
    audio: 'bg-emerald-300 dark:bg-emerald-700',
    document: 'bg-blue-200 dark:bg-blue-700',
  }

  const dotBg: Record<string, string> = {
    image: 'bg-violet-200 dark:bg-violet-600',
    video: 'bg-red-200 dark:bg-red-600',
    audio: 'bg-emerald-200 dark:bg-emerald-600',
    document: 'bg-blue-100 dark:bg-blue-600',
  }

  function formatDate(iso: string) {
    return new Date(iso).toLocaleDateString('en-CA')
  }
</script>

<button
  type="button"
  draggable="true"
  class="asset-card group flex w-full flex-col overflow-hidden rounded-xl bg-white text-left shadow-sm transition-shadow hover:shadow-md focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-400 dark:bg-gray-800 dark:shadow-none dark:hover:shadow-none dark:ring-inset dark:focus-visible:ring-indigo-500"
  onclick={(e) => onclick(e)}
  ondragstart={(e) => {
    e.dataTransfer?.setData('text/plain', asset.id)
    e.dataTransfer?.setData('application/json', JSON.stringify({ assetId: asset.id }))
  }}
>
  <!-- Thumbnail area -->
  <div
    class="relative w-full overflow-hidden rounded-t-xl {cardBg[category]}"
    style="aspect-ratio: 4/3"
  >
    <!-- Status dot -->
    <div class="absolute left-2.5 top-2.5 h-3 w-3 rounded-full {dotBg[category]}"></div>

    {#if (category === 'image' || category === 'video') && !isProcessing}
      <img
        src={assetApi.thumbUrl(asset.id)}
        alt={asset.original_filename}
        class="h-full w-full object-cover"
        loading="lazy"
        onerror={(e) => {
          ;(e.currentTarget as HTMLImageElement).style.display = 'none'
        }}
      />
    {:else}
      <!-- Centered icon -->
      <div class="flex h-full items-center justify-center">
        <div class="flex h-12 w-12 items-center justify-center rounded-xl bg-white/25">
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
          <span class="text-xs font-medium">Processing</span>
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
