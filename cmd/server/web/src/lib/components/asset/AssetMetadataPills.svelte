<script lang="ts">
  import { assetApi, formatBytes, type Asset } from '$lib/api'
  import { Download } from '@lucide/svelte'
  import Badge from '$lib/components/ui/Badge.svelte'
  import { DOWNLOAD_BUTTON_COLORS } from '$lib/stores/assetView'

  const typeLabel: Record<string, string> = {
    image: 'IMAGE',
    video: 'VIDEO',
    audio: 'AUDIO',
    document: 'DOCUMENT',
  }

  type Props = {
    asset: Asset
    category: string
  }

  let { asset, category }: Props = $props()

  function formatDate(iso: string) {
    return new Date(iso).toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    })
  }
</script>

<div class="flex items-start justify-between gap-3">
  <div class="min-w-0">
    <!-- Pills row -->
    <div class="mb-1.5 flex flex-wrap items-center gap-1.5">
      <Badge
        variant={category as
          | 'image'
          | 'video'
          | 'audio'
          | 'document'
          | 'neutral'}
        size="md"
      >
        {typeLabel[category]} ({asset.mime_type})
      </Badge>
      <span
        class="text-md rounded bg-gray-100 px-1.5 py-0.5 font-medium text-gray-500 dark:bg-gray-800 dark:text-gray-400"
      >
        {formatBytes(asset.size)}
      </span>
      {#if asset.width != null && asset.height != null}
        <span
          class="text-md rounded bg-gray-100 px-1.5 py-0.5 font-medium text-gray-500 dark:bg-gray-800 dark:text-gray-400"
        >
          {asset.width} × {asset.height}
        </span>
      {/if}
    </div>
    <!-- Author + date -->
    <p class="text-md mt-0.5 text-[#8ba4c4] dark:text-[#5f7590]">
      {formatDate(asset.created_at)}
    </p>
  </div>
  <!-- Download button -->
  <a
    href={assetApi.fileUrl(asset.id)}
    download={asset.original_filename}
    class="flex shrink-0 items-center justify-center rounded-xl p-2.5 text-white transition-colors {DOWNLOAD_BUTTON_COLORS[
      category
    ] ?? 'bg-indigo-600 hover:bg-indigo-700'}"
    aria-label="Download original"
  >
    <Download class="h-4 w-4" />
  </a>
</div>
