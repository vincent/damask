<script lang="ts">
  import type { Asset } from '$lib/api'

  type Props = {
    src: string
    vars: string
    asset: Asset
  }

  let { src, vars, asset }: Props = $props()
</script>

<style lang="scss">
  .zoom-overlay {
    transform-origin: center center;
    will-change: transform, border-radius, opacity;
    animation: card-zoom-in 380ms cubic-bezier(0.32, 0.72, 0.3, 1) forwards;
  }

  @keyframes card-zoom-in {
    from {
      transform: translate(var(--tx), var(--ty)) scale(var(--sx), var(--sy));
      opacity: 1;
    }
    80% {
      opacity: 1;
    }
    to {
      transform: translate(0px, 0px) scale(1, 1);
      opacity: 0;
    }
  }
</style>

<div class="zoom-overlay-bg fixed w-screen grid place-items-center p-40 inset-0 z-40 bg-black/70 backdrop-blur-lg"></div>
<div class="zoom-overlay fixed w-[75%] grid place-items-center p-40 inset-0 z-42" style={vars}>
  <img {src} width={asset.width || 640} alt="" class="object-cover min-w-xl max-w-3xl max-h-[80vh]" />
</div>
