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
        const ct = res.headers.get('Content-Type')
        if (ct) thumbContentType = ct.split(';')[0].trim()
      } else {
        noPreview = true
      }
    } catch {
      noPreview = true
    }
  }

  onMount(() => startPolling())
  onDestroy(() => {
    if (pollTimer) clearTimeout(pollTimer)
  })

  let showDeleteConfirm = $state(false)
</script>

<div class="variant-card group">
  <!-- Thumbnail -->
  <div class="thumb-area">
    {#if thumbUrl}
      <AssetThumbnail
        src={thumbUrl}
        contentType={thumbContentType}
        alt={variant.type}
        class="h-full w-full object-cover"
      />
    {:else if noPreview}
      <div class="thumb-placeholder">
        <ImageOff class="h-5 w-5" />
        <span class="thumb-label">{m.variant_no_preview()}</span>
      </div>
    {:else}
      <div class="thumb-placeholder">
        <div class="spinner"></div>
        <span class="thumb-label">{m.variant_thumbnail_generating()}</span>
      </div>
    {/if}

    <!-- Download overlay -->
    <a
      href={variantApi.fileUrl(assetId, variant.id)}
      download
      class="thumb-action top-2 right-2"
      aria-label={m.download()}
    >
      <Download class="h-3.5 w-3.5" />
    </a>

    {#if authStore.role !== 'viewer'}
      <button
        type="button"
        title={m.variant_delete_confirm()}
        class="thumb-action danger top-10 right-2"
        onclick={() => {
          showDeleteConfirm = true
        }}
      >
        <Trash2 class="h-3.5 w-3.5" />
      </button>
    {/if}
  </div>

  <!-- Info -->
  <div class="card-info">
    <p class="card-name">{variant.type}</p>
    <p class="card-meta">
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

<style>
  .variant-card {
    display: flex;
    flex-direction: column;
    overflow: hidden;
    border-radius: 10px;
    border: 1px solid var(--border);
    background: var(--bg-surface);
    transition: box-shadow 0.15s ease;
  }
  .variant-card:hover {
    box-shadow: 0 4px 12px -2px rgb(0 0 0 / 0.1);
  }
  .thumb-area {
    position: relative;
    display: flex;
    height: 160px;
    align-items: center;
    justify-content: center;
    overflow: hidden;
    background: var(--bg-app);
  }
  .thumb-placeholder {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    color: var(--text-muted);
  }
  .thumb-label {
    font-size: 11px;
  }
  .spinner {
    width: 20px;
    height: 20px;
    border-radius: 50%;
    border: 2px solid var(--border);
    border-top-color: var(--accent-cta);
    animation: spin 0.8s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  .thumb-action {
    position: absolute;
    display: flex;
    height: 28px;
    width: 28px;
    align-items: center;
    justify-content: center;
    border-radius: 7px;
    background: rgb(255 255 255 / 0.75);
    color: var(--text-secondary);
    opacity: 0;
    transition:
      opacity 0.15s ease,
      background 0.1s ease;
  }
  :global(.dark) .thumb-action {
    background: rgb(37 45 62 / 0.8);
    color: var(--text-secondary);
  }
  :global(.group:hover) .thumb-action {
    opacity: 1;
  }
  .thumb-action:hover {
    background: white;
  }
  :global(.dark) .thumb-action:hover {
    background: var(--bg-elevated);
  }
  .thumb-action.danger:hover {
    background: oklch(97% 0.02 15);
    color: oklch(50% 0.2 15);
  }
  :global(.dark) .thumb-action.danger:hover {
    background: oklch(28% 0.06 15 / 0.4);
    color: oklch(68% 0.18 15);
  }
  .card-info {
    padding: 10px 12px;
  }
  .card-name {
    font-size: 0.8125rem;
    font-weight: 600;
    color: var(--text-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .card-meta {
    margin-top: 2px;
    font-size: 0.6875rem;
    color: var(--text-muted);
  }
</style>
