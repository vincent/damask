<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import { Download, Trash2, ImageOff } from '@lucide/svelte'
  import { variantApi, type Variant } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { formatBytes } from '$lib/api'
  import { m } from '$lib/paraglide/messages'
  import AssetThumbnail from '$lib/components/AssetThumbnail.svelte'
  import ButtonDelete from '$lib/components/ui/ButtonDelete.svelte'
  import ConfirmModal from '$lib/components/ui/ConfirmModal.svelte'

  interface Props {
    variant: Variant
    assetId: string
    onDelete: () => void
  }

  let { variant, assetId, onDelete }: Props = $props()

  // Local reactive thumbnail state — updated by the polling loop.
  let thumbUrl = $state<string | null>(variant.thumbnail_url)
  let thumbContentType = $state(
    variant.thumbnail_content_type
      ? variant.thumbnail_content_type
      : variant.thumbnail_url?.includes('.mp4')
        ? 'video/mp4'
        : 'image/jpeg'
  )
  let noPreview = $state(false)

  const MAX_POLL = 30
  let pollCount = 0
  let pollTimer: ReturnType<typeof setTimeout> | null = null

  function startPolling() {
    if (thumbUrl || noPreview) return
    poll()
  }

  async function poll() {
    if (pollCount >= MAX_POLL) {
      noPreview = true
      return
    }
    pollCount++

    try {
      const res = await fetch(variantApi.thumbUrl(assetId, variant.id), {
        credentials: 'include',
      })
      if (res.status === 202) {
        pollTimer = setTimeout(poll, 2000)
      } else if (res.ok) {
        thumbUrl = variantApi.thumbUrl(assetId, variant.id)
        // Derive content-type from response header when available
        const ct = res.headers.get('Content-Type')
        if (ct) thumbContentType = ct.split(';')[0].trim()
      } else {
        noPreview = true
      }
    } catch {
      noPreview = true
    }
  }

  onMount(() => {
    startPolling()
  })

  onDestroy(() => {
    if (pollTimer) clearTimeout(pollTimer)
  })

  let showDeleteConfirm = $state(false)

  function handleDelete() {
    showDeleteConfirm = true
  }
</script>

<div
  class="group flex flex-col overflow-hidden rounded-xl border border-gray-200 dark:border-gray-700"
>
  <!-- Thumbnail -->
  <div
    class="relative flex h-40 items-center justify-center overflow-hidden bg-gray-100 dark:bg-gray-800"
  >
    {#if thumbUrl}
      <AssetThumbnail
        src={thumbUrl}
        contentType={thumbContentType}
        alt={variant.type}
        class="h-full w-full object-cover"
      />
    {:else if noPreview}
      <div
        class="flex flex-col items-center gap-1 text-gray-400 dark:text-gray-600"
      >
        <ImageOff class="h-5 w-5" />
        <span class="text-[11px]">{m.variant_no_preview()}</span>
      </div>
    {:else}
      <div
        class="flex flex-col items-center gap-1 text-gray-400 dark:text-gray-600"
      >
        <div
          class="h-5 w-5 animate-spin rounded-full border-2 border-gray-300 border-t-sky-500 dark:border-gray-600 dark:border-t-sky-400"
        ></div>
        <span class="text-[11px]">{m.variant_thumbnail_generating()}</span>
      </div>
    {/if}

    <!-- Download overlay -->
    <a
      href={variantApi.fileUrl(assetId, variant.id)}
      download
      class="absolute top-2 right-2 flex h-7 w-7 items-center justify-center rounded-lg bg-white/70 text-gray-600 opacity-0 transition-colors transition-opacity group-hover:opacity-100 hover:bg-white dark:bg-gray-800/70 dark:text-gray-300 dark:hover:bg-gray-500/20"
      aria-label={m.download()}
    >
      <Download class="h-3.5 w-3.5" />
    </a>

    {#if authStore.role !== 'viewer'}
      <button
        type="button"
        title={m.variant_delete_confirm()}
        class="absolute top-10 right-2 flex h-7 w-7 items-center justify-center rounded-lg bg-white/70 text-gray-600 opacity-0 transition-colors transition-opacity group-hover:opacity-100 hover:bg-red-50 hover:text-red-600 dark:bg-gray-800/70 dark:text-gray-300 dark:hover:bg-red-900/20 dark:hover:text-red-400"
        onclick={handleDelete}
      >
        <Trash2 class="h-3.5 w-3.5" />
      </button>
    {/if}
  </div>

  <!-- Info -->
  <div class="px-3 py-2.5">
    <p class="truncate text-sm font-semibold text-gray-800 dark:text-gray-200">
      {variant.type}
    </p>
    <p class="mt-0.5 text-xs text-gray-400 dark:text-gray-500">
      {variant.size != null ? formatBytes(variant.size) + ' · ' : ''}{new Date(
        variant.created_at
      ).toLocaleDateString('en-US', {
        month: 'short',
        day: 'numeric',
        year: 'numeric',
      })}
    </p>
  </div>
</div>

<ConfirmModal
  bind:open={showDeleteConfirm}
  title={m.variant_delete_confirm()}
  items={[variant.type]}
  onConfirm={onDelete}
  onCancel={() => {
    showDeleteConfirm = false
  }}
/>
