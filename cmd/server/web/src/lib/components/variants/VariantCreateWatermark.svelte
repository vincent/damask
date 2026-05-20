<script lang="ts">
  import { authStore } from '$lib/stores/auth.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import {
    assetApi,
    variantApi,
    type Asset,
    type WatermarkAsset,
  } from '$lib/api'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    asset: Asset
    creating: boolean
    handleCreate: (kind: string, params: Record<string, unknown>) => void
  }

  let { asset, creating, handleCreate }: Props = $props()

  const kind = 'image_watermark'
  let opacity = $state(50)
  let watermarkAsset = $state<WatermarkAsset | null>(null)
  let resolveError = $state('')
  let resolving = $state(false)
  let resolveToken = 0
  let watermarkTileWidth = $state<number | null>(null)
  let watermarkTileHeight = $state<number | null>(null)

  $effect(() => {
    const assetId = asset.id
    resolveToken += 1
    const token = resolveToken
    watermarkAsset = null
    resolveError = ''
    resolving = true

    void variantApi
      .resolveWatermark(assetId)
      .then((result) => {
        if (token !== resolveToken) return
        watermarkAsset = result
      })
      .catch((error: unknown) => {
        if (token !== resolveToken) return
        resolveError =
          error instanceof Error ? error.message : m.watermark_missing()
      })
      .finally(() => {
        if (token === resolveToken) resolving = false
      })
  })

  $effect(() => {
    const currentWatermark = watermarkAsset
    const assetWidth = asset.width
    const assetHeight = asset.height
    watermarkTileWidth = null
    watermarkTileHeight = null

    if (!currentWatermark || !assetWidth || !assetHeight) return
    if (typeof Image === 'undefined') return

    const image = new Image()
    image.src = assetApi.fileUrl(currentWatermark.id)
    image.onload = () => {
      if (currentWatermark !== watermarkAsset) return
      if (!image.naturalWidth || !image.naturalHeight) return

      watermarkTileWidth = (image.naturalWidth / assetWidth) * 100
      watermarkTileHeight = (image.naturalHeight / assetHeight) * 100
    }
  })
</script>

<div class="space-y-5">
  <div class="preview-card">
    {#if watermarkAsset}
      <div class="preview-head">
        <div>
          <div class="preview-title">{watermarkAsset.name}</div>
          <div class="preview-scope">
            {m.watermark_scope_label()}: {watermarkAsset.scope}
          </div>
        </div>
      </div>
      <div class="preview-stage">
        <div class="preview-frame">
          <img
            src={assetApi.thumbUrl(asset.id)}
            alt={asset.original_filename}
            class="preview-image"
          />
          <div
            aria-hidden="true"
            class="watermark-overlay"
            style={`opacity: ${opacity / 100}; background-image: url('${assetApi.fileUrl(watermarkAsset.id)}'); ${watermarkTileWidth && watermarkTileHeight ? `background-size: ${watermarkTileWidth}% ${watermarkTileHeight}%;` : ''}`}
          ></div>
        </div>
      </div>
    {:else if resolving}
      <div class="preview-empty">{m.loading()}</div>
    {:else}
      <div class="preview-empty error-text">
        {resolveError || m.watermark_missing()}
      </div>
    {/if}
  </div>

  <div>
    <label for="variant-{kind}-opacity" class="field-label"
      >{m.opacity()}:
      <span class="quality-val">{opacity}%</span></label
    >
    <input
      id="variant-{kind}-opacity"
      type="range"
      min="1"
      max="100"
      bind:value={opacity}
      class="range-input"
    />
  </div>

  <Button
    disabled={creating || authStore.role === 'viewer' || !watermarkAsset}
    onclick={() =>
      handleCreate(kind, {
        opacity: opacity / 100,
      })}
    class="w-full"
  >
    {creating ? m.queuing_() : m.variant_create_watermark()}
  </Button>
</div>

<style>
  .preview-card {
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 12px;
    background: var(--bg-surface);
  }
  .preview-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
    margin-bottom: 12px;
  }
  .preview-title {
    font-size: 0.9rem;
    font-weight: 600;
    color: var(--text-primary);
  }
  .preview-scope {
    font-size: 0.75rem;
    color: var(--text-secondary);
    text-transform: capitalize;
  }
  .scope-pill {
    border-radius: 999px;
    padding: 4px 8px;
    background: var(--bg-subtle);
    color: var(--text-secondary);
    font-size: 0.7rem;
    text-transform: capitalize;
  }
  .preview-image {
    display: block;
    width: 100%;
    height: auto;
    max-height: 180px;
    object-fit: contain;
  }
  .preview-stage {
    display: flex;
    justify-content: center;
    padding: 8px;
    background: var(--bg-base);
    border-radius: 8px;
  }
  .preview-frame {
    position: relative;
    display: inline-block;
    max-width: 100%;
  }
  .watermark-overlay {
    position: absolute;
    inset: 0;
    background-position: top left;
    background-repeat: repeat;
    pointer-events: none;
  }
  .preview-empty {
    display: grid;
    place-items: center;
    min-height: 120px;
    text-align: center;
    color: var(--text-secondary);
    font-size: 0.875rem;
  }
  .field-label {
    display: block;
    margin-bottom: 4px;
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--text-secondary);
  }
  .range-input {
    width: 100%;
    accent-color: var(--accent-cta);
  }
  .quality-val {
    font-weight: 600;
    color: var(--accent-cta);
  }
  .error-text {
    color: var(--danger, #b42318);
  }
</style>
