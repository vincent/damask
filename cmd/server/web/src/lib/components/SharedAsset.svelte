<script lang="ts">
  import type { Asset, PublicAsset } from '$lib/api'
  import AssetIcon from './AssetIcon.svelte'

  type Props = {
    asset: Asset | PublicAsset
    category: 'document' | 'audio' | 'video' | 'image'
    thumbUrl: string
    assetUrl: string
    zoomIn?: () => void
    zoomOut?: () => void
    zoomReset?: () => void
    onwheel?: (e: WheelEvent) => void
    rotateRight?: () => void
  }

  let {
    category,
    asset,
    thumbUrl,
    assetUrl,
    zoomIn = $bindable(),
    zoomOut = $bindable(),
    zoomReset = $bindable(),
    onwheel = $bindable(),
    rotateRight = $bindable(),
  }: Props = $props()
  let isPdf = $derived(asset.mime_type.includes('/pdf'))
  let haveSplashImage = $derived(
    thumbUrl && (category === 'audio' || (category === 'document' && !isPdf))
  )

  // Progressive image load: show thumb immediately, crossfade to full once loaded
  let fullLoaded = $state(false)
  let fullSrc = $state('')

  $effect(() => {
    // Reset on asset change
    fullLoaded = false
    fullSrc = ''
    if (category !== 'image') return
    const url = assetUrl
    const img = new Image()
    img.onload = () =>
      setTimeout(() => {
        fullSrc = url
        setTimeout(() => (fullLoaded = true), 10)
      }, 10)
    img.src = url
    return () => {
      img.onload = null
    }
  })

  const MIN_SCALE = 1
  const MAX_SCALE = 5
  const STEP = 0.05

  let scale = $state(1)
  let translateX = $state(0)
  let translateY = $state(0)
  let rotation = $state(0)
  let animated = $state(true)
  let container: HTMLDivElement

  // reset zoom and rotation on asset navigation (no animation)
  $effect(() => {
    asset.id
    animated = false
    scale = 1
    translateX = 0
    translateY = 0
    rotation = 0
    setTimeout(() => {
      animated = true
    }, 50)
  })

  function applyZoom(newScale: number, originX: number, originY: number) {
    newScale = Math.min(MAX_SCALE, Math.max(MIN_SCALE, newScale))
    const ratio = newScale / scale
    translateX = originX - ratio * (originX - translateX)
    translateY = originY - ratio * (originY - translateY)
    scale = newScale
  }

  let wheelTimer: ReturnType<typeof setTimeout> | null = null
  function onWheel(e: WheelEvent) {
    e.preventDefault()
    animated = false
    if (wheelTimer) clearTimeout(wheelTimer)
    wheelTimer = setTimeout(() => {
      animated = true
    }, 150)
    const rect = container.getBoundingClientRect()
    applyZoom(
      scale + (e.deltaY > 0 ? -STEP : STEP),
      e.clientX - rect.left,
      e.clientY - rect.top
    )
  }

  function centerZoom(delta: number) {
    if (!container) return
    const { width, height } = container.getBoundingClientRect()
    applyZoom(scale + delta, width / 2, height / 2)
  }

  // Expose center-zoom, rotate, and wheel handler to parent via bindable refs
  $effect(() => {
    zoomIn = () => centerZoom(STEP)
    zoomOut = () => centerZoom(-STEP)
    zoomReset = () => {
      scale = 1
      translateX = 0
      translateY = 0
    }
    onwheel = onWheel
    rotateRight = () => {
      rotation = (rotation + 90) % 360
    }
  })
</script>

<div
  bind:this={container}
  class="asset-preview-full max-h-[80vh] max-w-3xl min-w-xl"
  style="transform: scale({scale}) rotate({rotation}deg); transform-origin: center; transition: {animated
    ? 'transform 0.25s cubic-bezier(0.25, 0.46, 0.45, 0.94)'
    : 'none'}; cursor: {scale > 1 ? 'grab' : 'default'};"
>
  {#if category === 'image'}
    <div class="image-crossfade pointer-events-none h-full w-full">
      <img
        src={thumbUrl}
        alt={asset.original_filename}
        class="layer h-full w-full object-cover"
        style="opacity: 1;"
      />
      {#if fullSrc}
        <img
          src={fullSrc}
          alt={asset.original_filename}
          data-asset-dynamic-resource={asset.id}
          class="layer h-full w-full object-cover"
          style="opacity: {fullLoaded ? 1 : 0};"
          onerror={(e) => {
            ;(e.currentTarget as HTMLImageElement).style.display = 'none'
          }}
        />
      {/if}
    </div>
  {:else if haveSplashImage}
    <img
      src={thumbUrl}
      alt={asset.original_filename}
      data-asset-dynamic-resource={asset.id}
      class="min-w-xl object-cover {category === 'audio' ? 'invert' : ''}"
      loading="lazy"
      onerror={(e) => {
        ;(e.currentTarget as HTMLImageElement).style.display = 'none'
      }}
    />
  {/if}
  {#if isPdf}
    <iframe
      class="min-h-[80vh] w-full min-w-3xl"
      src={assetUrl}
      title={asset.original_filename}
    ></iframe>
  {:else if category === 'video'}
    <video class="w-full" controls>
      <source
        data-asset-dynamic-resource={asset.id}
        src={assetUrl}
        type={asset.mime_type}
      />
      Your browser does not support the video tag.
    </video>
  {:else if category === 'audio'}
    <audio class="w-full" controls>
      <source
        data-asset-dynamic-resource={asset.id}
        src={assetUrl}
        type={asset.mime_type}
      />
      Your browser does not support the audio element.
    </audio>
  {:else if !thumbUrl}
    <div class="flex h-full items-center justify-center">
      <div
        class="flex h-12 w-12 items-center justify-center rounded-xl bg-white/25"
      >
        <AssetIcon {category} class="h-7 w-7 text-white" />
      </div>
    </div>
  {/if}
</div>

<style>
  .image-crossfade {
    position: relative;
  }

  .image-crossfade .layer {
    position: absolute;
    inset: 0;
    transition: opacity 400ms cubic-bezier(0.16, 1, 0.3, 1);
  }

  /* first layer (thumb) is relative to give the container its size */
  .image-crossfade .layer:first-child {
    position: relative;
  }

  @media (prefers-reduced-motion: reduce) {
    .image-crossfade .layer {
      transition: none;
    }
  }
</style>
