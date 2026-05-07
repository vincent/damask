<script lang="ts">
  import { authStore } from '$lib/stores/auth.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import { type Asset, type ImageRouterModelsResponse } from '$lib/api'
  import { m } from '$lib/paraglide/messages'
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

  const kind = 'image_with_prompt'

  let prompt = $state('')
  let model = $state('')
  let defaultModelId = $state('')
  let configured = $state(true)

  function handleModelsLoaded(res: ImageRouterModelsResponse) {
    configured = res.configured
    defaultModelId = res.default_model
    if (!model) model = res.default_model
  }
</script>

<div class="space-y-5">
  <div class="form-header">
    <p class="form-title">AI image transform</p>
    <p class="form-desc">
      Transform this image with a text prompt. Output is always PNG.
    </p>
  </div>

  <div>
    <label for="variant-{kind}-prompt" class="field-label">Prompt</label>
    <textarea
      id="variant-{kind}-prompt"
      bind:value={prompt}
      rows="4"
      class="field-input textarea"
      placeholder="Describe how you want the image transformed…"
    ></textarea>
  </div>

  <ImageRouterModelSelect
    bind:value={model}
    {defaultModelId}
    disabled={creating}
    onloaded={handleModelsLoaded}
  />

  <p class="form-note">Output is always PNG.</p>

  <Button
    disabled={creating ||
      authStore.role === 'viewer' ||
      !configured ||
      !prompt.trim()}
    onclick={() => handleCreate(kind, { prompt: prompt.trim(), model })}
    class="w-full"
  >
    {creating ? m.queuing_() : 'Create variant'}
  </Button>
</div>

<style>
  .form-header {
    padding-bottom: 4px;
    border-bottom: 1px solid var(--border-subtle);
  }
  .form-title {
    font-size: 0.9375rem;
    font-weight: 600;
    color: var(--text-primary);
  }
  .form-desc,
  .form-note {
    font-size: 0.8125rem;
    color: var(--text-muted);
  }
  .field-label {
    display: block;
    margin-bottom: 4px;
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--text-secondary);
  }
  .field-input {
    width: 100%;
    border-radius: 8px;
    border: 1px solid var(--border);
    background: var(--bg-surface);
    color: var(--text-primary);
    padding: 7px 10px;
    font-size: 0.875rem;
    outline: none;
    transition: border-color 0.12s ease;
  }
  .field-input:focus {
    border-color: var(--accent-cta);
  }
  .textarea {
    min-height: 92px;
    resize: vertical;
  }
</style>
