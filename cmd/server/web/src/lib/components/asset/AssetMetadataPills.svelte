<script lang="ts">
  import { assetApi, formatBytes, type Asset } from '$lib/api'
  import { stackApi } from '$lib/api/client'
  import { Download, ChevronDown } from '@lucide/svelte'
  import Badge from '$lib/components/ui/Badge.svelte'
  import { DOWNLOAD_BUTTON_COLORS } from '$lib/stores/assetView'
  import { m } from '$lib/paraglide/messages'

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

  let details: HTMLDetailsElement | undefined = $state()

  function closeDropdown() {
    details?.removeAttribute('open')
  }

  async function downloadZip(variantMode: 'shared' | 'all') {
    closeDropdown()
    await stackApi.exportZip([asset.id], asset.original_filename, variantMode)
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

  <!-- Download dropdown -->
  <details bind:this={details} class="group relative shrink-0">
    <summary
      class="flex cursor-pointer list-none items-center gap-0.5 rounded-xl p-2.5 text-white transition-colors {DOWNLOAD_BUTTON_COLORS[
        category
      ] ?? 'bg-indigo-600 hover:bg-indigo-700'}"
      aria-label="Download options"
    >
      <Download class="h-4 w-4" />
    </summary>

    <div
      class="absolute right-0 top-full z-20 mt-1 w-52 overflow-hidden rounded-xl border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-900"
    >
      <a
        href={assetApi.fileUrl(asset.id)}
        download={asset.original_filename}
        class="flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-800"
        onclick={closeDropdown}
      >
        <Download class="h-3.5 w-3.5 shrink-0" />
        {m.download_original()}
      </a>
      <button
        type="button"
        class="flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-800"
        onclick={() => downloadZip('shared')}
      >
        <Download class="h-3.5 w-3.5 shrink-0" />
        {m.with_shared_variants()}
      </button>
      <button
        type="button"
        class="flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-800"
        onclick={() => downloadZip('all')}
      >
        <Download class="h-3.5 w-3.5 shrink-0" />
        {m.with_all_variants()}
      </button>
    </div>
  </details>
</div>
