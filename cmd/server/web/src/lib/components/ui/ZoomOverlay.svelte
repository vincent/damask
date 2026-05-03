<script lang="ts">
  import type { Asset } from '$lib/api'

  type Props = {
    src: string
    vars: string
    asset: Asset
  }

  let { src, vars, asset }: Props = $props()
</script>

<div
  class="zoom-overlay-bg fixed inset-0 z-40 grid w-screen place-items-center bg-black/70 p-40 backdrop-blur-lg"
></div>
<div
  class="zoom-overlay fixed inset-0 z-42 grid w-[75%] place-items-center p-40"
  style={vars}
>
  <img
    {src}
    width={asset.width || 640}
    alt=""
    class="max-h-[80vh] max-w-3xl min-w-xl object-cover"
  />
</div>

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
