<script lang="ts">
  import { Layers } from '@lucide/svelte'
  import {
    assetApi,
    type ImageRouterModel,
    type ImageRouterModelsResponse,
  } from '$lib/api'

  interface Props {
    value: string
    values?: string[]
    defaultModelId?: string
    disabled?: boolean
    allowMulti?: boolean
    onloaded?: (response: ImageRouterModelsResponse) => void
  }

  let {
    value = $bindable(''),
    values = $bindable([]),
    defaultModelId = '',
    disabled = false,
    allowMulti = false,
    onloaded,
  }: Props = $props()

  let multiMode = $state(false)

  function toggleMulti() {
    multiMode = !multiMode
    if (multiMode) {
      values = value ? [value] : []
    } else {
      values = []
    }
  }

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
    <div class="label-row">
      <label class="field-label" for="imagerouter-model">Model</label>
      {#if allowMulti}
        <div class="flex items-center justify-between">
          <span class="field-label mx-2">multi</span>
          <button
            type="button"
            class="multi-toggle"
            class:active={multiMode}
            title={multiMode ? 'Single model' : 'Select multiple models'}
            onclick={toggleMulti}
          >
            <Layers size={13} strokeWidth={1.75} />
          </button>
        </div>
      {/if}
    </div>
    {#if multiMode}
      <select
        id="imagerouter-model"
        multiple
        bind:value={values}
        {disabled}
        class="field-input multi"
        onwheel={(e) => e.stopPropagation()}
      >
        {#each models as model}
          <option value={model.id}>
            {model.name} ({model.price_per_image
              ? `$${model.price_per_image.toFixed(4)} /image`
              : 'free'})
            {#if model.id === defaultModelId}
              (default)
            {/if}
          </option>
        {/each}
      </select>
    {:else}
      <select id="imagerouter-model" bind:value {disabled} class="field-input">
        {#each models as model}
          <option value={model.id}>
            {model.name} ({model.price_per_image
              ? `$${model.price_per_image.toFixed(4)} /image`
              : 'free'})
            {#if model.id === defaultModelId}
              (default)
            {/if}
          </option>
        {/each}
      </select>
    {/if}
    {#if error}
      <p class="status-line">
        Could not refresh the model list. Using defaults.
      </p>
    {/if}
  </div>
{/if}

<style>
  .label-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 4px;
  }

  .field-label {
    display: block;
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--text-secondary);
  }

  .multi-toggle {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 22px;
    height: 22px;
    flex-shrink: 0;
    border-radius: 5px;
    border: 1px solid var(--border);
    background: transparent;
    color: var(--text-secondary);
    cursor: pointer;
    transition:
      background 120ms ease,
      color 120ms ease,
      border-color 120ms ease;
  }

  .multi-toggle.active {
    background: var(--accent-cta-subtle, oklch(95% 0.05 250));
    color: var(--accent-cta);
    border-color: var(--accent-cta);
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
  .field-input.multi {
    height: 300px;
    overflow-y: auto;
    padding: 4px 0;
  }
  .field-input.multi option {
    padding: 4px 10px;
  }
  .status-line {
    font-size: 0.8125rem;
    color: var(--text-muted);
  }
  .status-line.warning {
    color: var(--accent-danger);
  }
</style>
