<script lang="ts">
  import { assetApi, formatBytes, mimeCategory, type Asset } from '$lib/api/client'
  import { File, Loader, Play } from '@lucide/svelte'

  interface Props {
    asset: Asset
    onclick: (e: MouseEvent) => void
  }

  let { asset, onclick }: Props = $props()

  const category = $derived(mimeCategory(asset.mime_type))
  const isProcessing = $derived(!asset.thumbnail_key.Valid)

  const cardBg: Record<string, string> = {
    image: '#E8C05A',
    video: '#E88A8A',
    audio: '#7CC89A',
    document: '#B8C8E8',
  }

  const dotBg: Record<string, string> = {
    image: '#F0D080',
    video: '#F0A8A8',
    audio: '#A0DCBA',
    document: '#D0DCEE',
  }

  function formatDate(iso: string) {
    return new Date(iso).toLocaleDateString('en-CA')
  }
</script>

<button
  type="button"
  draggable="true"
  class="group flex w-full flex-col overflow-hidden rounded-xl bg-white text-left shadow-sm transition-shadow hover:shadow-md focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-400"
  onclick={(e) => onclick(e)}
  ondragstart={(e) => {
    e.dataTransfer?.setData('text/plain', asset.id)
    e.dataTransfer?.setData('application/json', JSON.stringify({ assetId: asset.id }))
  }}
>
  <!-- Thumbnail area -->
  <div
    class="relative w-full overflow-hidden rounded-t-xl"
    style="aspect-ratio: 4/3; background-color: {cardBg[category]}"
  >
    <!-- Status dot -->
    <div
      class="absolute left-2.5 top-2.5 h-3 w-3 rounded-full"
      style="background-color: {dotBg[category]}"
    ></div>

    {#if category === 'image' && !isProcessing}
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
  <div class="flex flex-col gap-1.5 px-3 py-2.5">
    <p class="truncate text-sm font-medium text-gray-900" title={asset.original_filename}>
      {asset.original_filename}
    </p>
    <div class="flex items-center justify-between">
      <span class="text-xs text-gray-400">{formatBytes(asset.size)}</span>
      <span class="text-xs text-gray-400">{formatDate(asset.created_at)}</span>
    </div>
    {#if asset.tags && asset.tags.length > 0}
      <div class="flex flex-wrap gap-1">
        {#each asset.tags.slice(0, 3) as tag}
          <span class="rounded bg-gray-100 px-1.5 py-0.5 text-[11px] text-gray-500">{tag}</span>
        {/each}
      </div>
    {/if}
  </div>
</button>
