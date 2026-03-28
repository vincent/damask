<script lang="ts">
  import { assetApi, formatBytes, mimeCategory, type Asset } from '$lib/api/client'

  interface Props {
    asset: Asset
    onclick: () => void
  }

  let { asset, onclick }: Props = $props()

  const category = $derived(mimeCategory(asset.mime_type))
  const isProcessing = $derived(!asset.thumbnail_key.Valid)

  const categoryLabel: Record<ReturnType<typeof mimeCategory>, string> = {
    image: 'Image',
    video: 'Video',
    audio: 'Audio',
    document: 'Document',
  }

  const categoryColor: Record<ReturnType<typeof mimeCategory>, string> = {
    image: 'bg-blue-100 text-blue-700',
    video: 'bg-purple-100 text-purple-700',
    audio: 'bg-green-100 text-green-700',
    document: 'bg-gray-100 text-gray-700',
  }
</script>

<button
  type="button"
  class="group relative flex flex-col overflow-hidden rounded-lg border border-gray-200 bg-white text-left shadow-sm transition hover:border-blue-400 hover:shadow-md focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
  onclick={onclick}
>
  <!-- Thumbnail area -->
  <div class="relative aspect-square w-full overflow-hidden bg-gray-50">
    {#if category === 'image'}
      <img
        src={assetApi.thumbUrl(asset.id)}
        alt={asset.original_filename}
        class="h-full w-full object-cover transition group-hover:scale-105"
        loading="lazy"
        onerror={(e) => {
          ;(e.currentTarget as HTMLImageElement).style.display = 'none'
        }}
      />
    {:else if category === 'video'}
      <div class="flex h-full w-full items-center justify-center text-purple-400">
        <svg class="h-12 w-12" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="1.5"
            d="M15 10l4.553-2.069A1 1 0 0121 8.87v6.26a1 1 0 01-1.447.894L15 14M4 8h11a1 1 0 011 1v6a1 1 0 01-1 1H4a1 1 0 01-1-1V9a1 1 0 011-1z"
          />
        </svg>
      </div>
    {:else if category === 'audio'}
      <div class="flex h-full w-full items-center justify-center text-green-400">
        <svg class="h-12 w-12" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="1.5"
            d="M9 19V6l12-3v13M9 19c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zm12-3c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zM9 10l12-3"
          />
        </svg>
      </div>
    {:else}
      <div class="flex h-full w-full items-center justify-center text-gray-400">
        <svg class="h-12 w-12" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="1.5"
            d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z"
          />
        </svg>
      </div>
    {/if}

    {#if isProcessing}
      <div
        class="absolute inset-0 flex items-center justify-center bg-black/40 backdrop-blur-[2px]"
      >
        <div class="flex flex-col items-center gap-1.5 text-white">
          <svg class="h-5 w-5 animate-spin" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"
            ></circle>
            <path
              class="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
            ></path>
          </svg>
          <span class="text-xs font-medium">Processing</span>
        </div>
      </div>
    {/if}
  </div>

  <!-- Info -->
  <div class="flex flex-col gap-1 p-2.5">
    <p class="truncate text-sm font-medium text-gray-900" title={asset.original_filename}>
      {asset.original_filename}
    </p>
    <div class="flex items-center justify-between">
      <span class="rounded px-1.5 py-0.5 text-xs font-medium {categoryColor[category]}">
        {categoryLabel[category]}
      </span>
      <span class="text-xs text-gray-400">{formatBytes(asset.size)}</span>
    </div>
  </div>
</button>
