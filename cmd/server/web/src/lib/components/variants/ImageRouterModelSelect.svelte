<script lang="ts">
  import {
    assetApi,
    type ImageRouterModel,
    type ImageRouterModelsResponse,
  } from '$lib/api'

  interface Props {
    value: string
    defaultModelId?: string
    disabled?: boolean
    onloaded?: (response: ImageRouterModelsResponse) => void
  }

  let {
    value = $bindable(''),
    defaultModelId = '',
    disabled = false,
    onloaded,
  }: Props = $props()

  let models = $state<ImageRouterModel[]>([])
  let loading = $state(true)
  let error = $state(false)
  let configured = $state(true)

  $effect(() => {
    let cancelled = false

    const load = async () => {
      loading = true
      error = false
      try {
        const res = await assetApi.fetchImageRouterModels()
        if (cancelled) return
        models = res.models
        configured = res.configured
        if (!defaultModelId) defaultModelId = res.default_model
        if (!value) value = res.default_model || defaultModelId
        onloaded?.(res)
      } catch {
        if (cancelled) return
        error = true
      } finally {
        if (!cancelled) loading = false
      }
    }

    load()
    return () => {
      cancelled = true
    }
  })
</script>

{#if loading}
  <p class="status-line">Loading models…</p>
{:else if !configured}
  <p class="status-line warning">
    ImageRouter is not configured. Ask your admin to set IMAGEROUTER_API_KEY.
  </p>
{:else}
  <div class="space-y-2">
    <label class="field-label" for="imagerouter-model">Model</label>
    <select id="imagerouter-model" bind:value {disabled} class="field-input">
      {#each models as model}
        <option value={model.id}>
          {model.name} ({model.price_per_image
            ? `$${model.price_per_image.toFixed(4)}/image`
            : 'free'})
          {#if model.id === defaultModelId}
            (default)
          {/if}
        </option>
      {/each}
    </select>
    {#if error}
      <p class="status-line">
        Could not refresh the model list. Using defaults.
      </p>
    {/if}
  </div>
{/if}

<style>
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
  .status-line {
    font-size: 0.8125rem;
    color: var(--text-muted);
  }
  .status-line.warning {
    color: var(--accent-danger);
  }
</style>
