<script lang="ts">
  import { m } from '$lib/paraglide/messages'
  import { type Asset, type ImageRouterModelsResponse } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import ImageRouterModelSelect from './ImageRouterModelSelect.svelte'

  interface Props {
    asset: Asset
    creating: boolean
    handleCreate: (
      kind: string,
      params: Record<string, unknown>
    ) => void | Promise<void>
  }

  let { asset, creating, handleCreate }: Props = $props()

  const kind = 'image_bg_remove'
  let model = $state('')
  let defaultModelId = $state('')
  let configured = $state(true)

  function handleModelsLoaded(res: ImageRouterModelsResponse) {
    configured = res.configured
    defaultModelId = res.default_bg_remove_model
    if (!model) model = res.default_bg_remove_model
  }
</script>

<div class="space-y-5">
  <div class="notice">
    <p class="notice-title">{m.bg_remove()}</p>
    <p class="notice-body">
      Remove the background from this image with AI. Output is always PNG.
    </p>
  </div>

  <ImageRouterModelSelect
    bind:value={model}
    {defaultModelId}
    disabled={creating}
    onloaded={handleModelsLoaded}
  />

  <Button
    disabled={creating || authStore.role === 'viewer' || !configured}
    onclick={() => handleCreate(kind, { model })}
    class="w-full"
  >
    {creating ? m.queuing_() : m.bg_remove()}
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
    color: var(--text-primary);
    margin-bottom: 4px;
  }
  :global(.dark) .notice-title {
    color: var(--text-primary);
  }
  .notice-body {
    font-size: 0.8125rem;
    color: var(--text-muted);
  }
  :global(.dark) .notice-body {
    color: var(--text-muted);
  }
</style>
