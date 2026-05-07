<script lang="ts">
  import type { Snippet } from 'svelte'

  let {
    onclose,
    children,
    class: extraClass,
  }: { onclose?: () => void; children?: Snippet; class?: string } = $props()
</script>

<div
  class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-md {extraClass}"
  role="button"
  tabindex="-1"
  onclick={(e) => {
    if (e.target === e.currentTarget) onclose?.()
  }}
  onkeydown={(e) => e.key === 'Enter' && onclose?.()}
  aria-label="Close modal"
>
  {@render children?.()}
</div>
