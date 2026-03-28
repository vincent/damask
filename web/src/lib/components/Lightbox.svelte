<script lang="ts">
  import { assetApi, formatBytes, mimeCategory, type Asset } from '$lib/api/client'

  interface Props {
    asset: Asset | null
    onclose: () => void
    ondeleted: (id: string) => void
  }

  let { asset, onclose, ondeleted }: Props = $props()

  let deleting = $state(false)

  const category = $derived(asset ? mimeCategory(asset.mime_type) : 'document')

  function formatDate(iso: string) {
    return new Date(iso).toLocaleDateString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    })
  }

  async function handleDelete() {
    if (!asset) return
    deleting = true
    try {
      await assetApi.delete(asset.id)
      ondeleted(asset.id)
    } finally {
      deleting = false
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') onclose()
  }
</script>

<svelte:window onkeydown={handleKeydown} />

{#if asset}
  <!-- Backdrop -->
  <div
    class="fixed inset-0 z-40 bg-black/50 backdrop-blur-sm"
    role="button"
    tabindex="-1"
    onclick={onclose}
    onkeydown={(e) => e.key === 'Enter' && onclose()}
    aria-label="Close lightbox"
  ></div>

  <!-- Panel -->
  <div
    class="fixed inset-y-0 right-0 z-50 flex w-full max-w-lg flex-col bg-white shadow-2xl"
    role="dialog"
    aria-modal="true"
    aria-label={asset.original_filename}
  >
    <!-- Header -->
    <div class="flex items-center justify-between border-b border-gray-200 px-5 py-4">
      <h2 class="truncate text-base font-semibold text-gray-900" title={asset.original_filename}>
        {asset.original_filename}
      </h2>
      <button
        type="button"
        class="ml-3 shrink-0 rounded-md p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600"
        onclick={onclose}
        aria-label="Close"
      >
        <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>

    <!-- Preview -->
    <div class="flex items-center justify-center border-b border-gray-200 bg-gray-50 p-4" style="min-height: 280px">
      {#if category === 'image'}
        <img
          src={assetApi.fileUrl(asset.id)}
          alt={asset.original_filename}
          class="max-h-64 max-w-full rounded object-contain shadow"
        />
      {:else if category === 'video'}
        <video
          src={assetApi.fileUrl(asset.id)}
          controls
          class="max-h-64 max-w-full rounded shadow"
        >
          <track kind="captions" />
        </video>
      {:else}
        <div class="flex flex-col items-center gap-3 text-gray-400">
          <svg class="h-16 w-16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="1.5"
              d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z"
            />
          </svg>
          <span class="text-sm">{asset.mime_type}</span>
        </div>
      {/if}
    </div>

    <!-- Metadata -->
    <div class="flex-1 overflow-y-auto px-5 py-4">
      <h3 class="mb-3 text-xs font-semibold uppercase tracking-wide text-gray-500">Details</h3>
      <dl class="space-y-2 text-sm">
        <div class="flex justify-between">
          <dt class="text-gray-500">Type</dt>
          <dd class="font-medium text-gray-900">{asset.mime_type}</dd>
        </div>
        <div class="flex justify-between">
          <dt class="text-gray-500">Size</dt>
          <dd class="font-medium text-gray-900">{formatBytes(asset.size)}</dd>
        </div>
        {#if asset.width.Valid && asset.height.Valid}
          <div class="flex justify-between">
            <dt class="text-gray-500">Dimensions</dt>
            <dd class="font-medium text-gray-900">{asset.width.Int64} × {asset.height.Int64}</dd>
          </div>
        {/if}
        <div class="flex justify-between">
          <dt class="text-gray-500">Uploaded</dt>
          <dd class="font-medium text-gray-900">{formatDate(asset.created_at)}</dd>
        </div>
      </dl>
    </div>

    <!-- Actions -->
    <div class="flex gap-2 border-t border-gray-200 px-5 py-4">
      <a
        href={assetApi.fileUrl(asset.id)}
        download={asset.original_filename}
        class="flex flex-1 items-center justify-center gap-1.5 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
      >
        <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
        </svg>
        Download
      </a>
      <button
        type="button"
        class="flex flex-1 items-center justify-center gap-1.5 rounded-lg bg-red-50 px-4 py-2 text-sm font-medium text-red-600 hover:bg-red-100 disabled:opacity-50"
        onclick={handleDelete}
        disabled={deleting}
      >
        {#if deleting}
          <svg class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
          </svg>
        {:else}
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
          </svg>
        {/if}
        Delete
      </button>
    </div>
  </div>
{/if}
