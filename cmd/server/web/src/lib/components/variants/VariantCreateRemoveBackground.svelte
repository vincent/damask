<script lang="ts">
  import { authStore } from '$lib/stores/auth.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import Spinner from '../ui/Spinner.svelte'
  import { type Asset } from '$lib/api'

  interface Props {
    asset: Asset
    creating: boolean
    handleCreate: (kind: string, params: Record<string, unknown>) => void
  }

  let { asset, creating, handleCreate }: Props = $props()

  const kind = 'image_bg_remove'
</script>

<div class="space-y-5">
  <div class="notice">
    <p class="notice-title">Requires Remove.bg API key</p>
    <p class="notice-body">
      Set <code class="notice-code">REMOVEBG_API_KEY</code> in your server environment.
      Returns a transparent PNG.
    </p>
  </div>

  <Button
    disabled={creating || authStore.role === 'viewer'}
    onclick={() => handleCreate(kind, {})}
    class="w-full"
  >
    {#if creating}<Spinner size="sm" />{/if}
    {creating ? 'Queuing…' : 'Remove Background'}
  </Button>
</div>

<style>
  .notice {
    border-radius: 8px;
    border: 1px solid oklch(85% 0.08 80);
    background: oklch(97% 0.03 80);
    padding: 12px 14px;
  }
  :global(.dark) .notice {
    border-color: oklch(40% 0.08 80 / 0.5);
    background: oklch(30% 0.06 80 / 0.2);
  }
  .notice-title {
    font-size: 0.875rem;
    font-weight: 600;
    color: oklch(42% 0.1 60);
    margin-bottom: 4px;
  }
  :global(.dark) .notice-title {
    color: oklch(72% 0.1 70);
  }
  .notice-body {
    font-size: 0.8125rem;
    color: oklch(50% 0.08 60);
  }
  :global(.dark) .notice-body {
    color: oklch(62% 0.08 70);
  }
  .notice-code {
    border-radius: 4px;
    background: oklch(90% 0.05 80);
    padding: 1px 5px;
    font-size: 0.75rem;
  }
  :global(.dark) .notice-code {
    background: oklch(35% 0.07 70 / 0.6);
  }
</style>
