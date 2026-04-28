<script lang="ts">
  import { m } from '$lib/paraglide/messages'

  interface Props {
    src: string
    contentType?: string
    alt?: string
    class?: string
    hoverPlay?: boolean
    assetId?: string
  }

  let {
    src,
    contentType = 'image/jpeg',
    alt = '',
    class: className = '',
    hoverPlay = true,
    assetId,
  }: Props = $props()

  let videoEl = $state<HTMLVideoElement | undefined>()

  const isVideo = $derived(contentType.startsWith('video/'))

  function onMouseEnter() {
    if (hoverPlay && videoEl) {
      videoEl.play().catch(() => {})
    }
  }

  function onMouseLeave() {
    if (videoEl) {
      videoEl.pause()
      videoEl.currentTime = 0
    }
  }
</script>

{#if isVideo}
  <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
  <video
    bind:this={videoEl}
    muted
    loop
    playsinline
    aria-label={alt || m.thumbnail_video_preview()}
    class={className}
    onmouseenter={onMouseEnter}
    onmouseleave={onMouseLeave}
  >
    <source data-asset-dynamic-resource={assetId} {src} type={contentType} />
  </video>
{:else}
  <img {src} {alt} class={className} loading="lazy" data-asset-dynamic-resource={assetId} />
{/if}
