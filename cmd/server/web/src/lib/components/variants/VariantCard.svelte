<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import { Download, ImageOff, MoreHorizontal } from '@lucide/svelte'
  import { variantApi, type Variant } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { formatBytes } from '$lib/api'
  import { m } from '$lib/paraglide/messages'
  import { toastStore } from '$lib/stores/toast.svelte'
  import AssetThumbnail from '$lib/components/AssetThumbnail.svelte'
  import ConfirmModal from '$lib/components/ui/ConfirmModal.svelte'

  interface Props {
    variant: Variant
    assetId: string
    isSelected: boolean
    onSelect: () => void
    onDelete: () => void
    onPromote: () => void
    onThumbnailUpdated: () => void
    onRerun: () => void
  }

  let {
    variant,
    assetId,
    isSelected,
    onSelect,
    onDelete,
    onPromote,
    onThumbnailUpdated,
    onRerun,
  }: Props = $props()

  let thumbUrl = $state<string | null>(null)
  let thumbContentType = $state('image/jpeg')
  let noPreview = $state(false)
  let menuOpen = $state(false)

  const MAX_POLL = 30
  let pollCount = 0
  let pollTimer: ReturnType<typeof setTimeout> | null = null

  $effect(() => {
    thumbUrl = variant.thumbnail_url
    thumbContentType = variant.thumbnail_content_type
      ? variant.thumbnail_content_type
      : variant.thumbnail_url?.includes('.mp4')
        ? 'video/mp4'
        : 'image/jpeg'
    noPreview = false
    pollCount = 0
    if (pollTimer) clearTimeout(pollTimer)
    startPolling()
  })

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

  async function setAsThumbnail() {
    try {
      await variantApi.setThumbnail(assetId, variant.id)
      toastStore.show('Thumbnail updated', 'success')
      menuOpen = false
      onThumbnailUpdated()
    } catch (e: unknown) {
      toastStore.show(
        e instanceof Error ? e.message : 'Failed to update thumbnail.',
        'error'
      )
    }
  }

  async function rerunVariant() {
    let params: Record<string, unknown> | undefined
    if (variant.transform_params) {
      try {
        params = JSON.parse(variant.transform_params) as Record<string, unknown>
      } catch {
        params = undefined
      }
    }
    try {
      await variantApi.rerun(assetId, variant.id, params)
      toastStore.show('Variant queued for re-run', 'success')
      menuOpen = false
      onRerun()
    } catch (e: unknown) {
      toastStore.show(
        e instanceof Error ? e.message : 'Failed to re-run variant.',
        'error'
      )
    }
  }
</script>

<div
  class="variant-card group"
  class:selected={isSelected}
  role="button"
  tabindex="0"
  onclick={onSelect}
  onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && onSelect()}
  aria-pressed={isSelected}
>
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

    <div class="menu-wrap">
      <a
        href={variantApi.fileUrl(assetId, variant.id)}
        download
        class="thumb-action top-2 right-10"
        aria-label={m.download()}
        onclick={(e) => e.stopPropagation()}
      >
        <Download class="h-3.5 w-3.5" />
      </a>

      {#if authStore.role !== 'viewer'}
        <button
          type="button"
          class="thumb-action top-2 right-2"
          aria-label="Variant actions"
          onclick={(e) => {
            e.stopPropagation()
            menuOpen = !menuOpen
          }}
        >
          <MoreHorizontal class="h-3.5 w-3.5" />
        </button>
      {/if}
    </div>

    {#if menuOpen && authStore.role !== 'viewer'}
      <div class="variant-menu z-10">
        <button type="button" onclick={setAsThumbnail}>Set as thumbnail</button>
        <button
          type="button"
          onclick={() => {
            menuOpen = false
            onPromote()
          }}
        >
          Create new asset
        </button>
        {#if variant.type === 'image_bg_remove' || variant.type === 'image_with_prompt'}
          <button type="button" onclick={rerunVariant}>Re-run</button>
        {/if}
        <button
          type="button"
          class="danger"
          onclick={() => {
            menuOpen = false
            showDeleteConfirm = true
          }}
        >
          {m.kb_delete()}
        </button>
      </div>
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
  .variant-card.selected {
    border-color: var(--accent-cta);
    box-shadow: 0 0 0 2px var(--accent-cta);
  }
  .variant-card {
    cursor: pointer;
  }
  .thumb-area {
    position: relative;
    display: flex;
    height: 160px;
    align-items: center;
    justify-content: center;
    overflow: visible;
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
  .menu-wrap {
    position: absolute;
    inset: 0;
  }
  .variant-menu {
    position: absolute;
    top: 42px;
    right: 8px;
    display: grid;
    min-width: 170px;
    border: 1px solid var(--border);
    border-radius: 10px;
    background: var(--bg-surface);
    box-shadow: 0 16px 40px rgb(0 0 0 / 0.14);
    overflow: hidden;
    z-index: 2;
  }
  .variant-menu button {
    border: 0;
    background: transparent;
    padding: 10px 12px;
    text-align: left;
    color: var(--text-primary);
    cursor: pointer;
  }
  .variant-menu button:hover {
    background: var(--bg-app);
  }
  .variant-menu button.danger {
    color: var(--accent-danger);
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
