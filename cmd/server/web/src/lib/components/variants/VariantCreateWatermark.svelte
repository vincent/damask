<script lang="ts">
  import { authStore } from '$lib/stores/auth.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import { type Asset } from '$lib/api'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    asset: Asset
    creating: boolean
    handleCreate: (kind: string, params: Record<string, unknown>) => void
  }

  let { asset, creating, handleCreate }: Props = $props()

  const kind = 'image_watermark'

  let watermarkOpacity = $state(50)
  let watermarkQuality = $state(80)
  let watermarkFormat = $state<'jpeg' | 'png' | 'tiff'>('jpeg')
</script>

<div class="space-y-5">
  <div class="grid grid-cols-2 gap-4">
    <div>
      <label for="variant-{kind}-opacity" class="field-label"
        >{m.opacity()}:
        <span class="quality-val">{watermarkOpacity}%</span></label
      >
      <input
        id="variant-{kind}-opacity"
        type="range"
        min="1"
        max="100"
        bind:value={watermarkOpacity}
        class="range-input"
      />
    </div>
    <div>
      <label for="variant-{kind}-quality" class="field-label"
        >{m.quality()}:
        <span class="quality-val">{watermarkQuality}%</span></label
      >
      <input
        id="variant-{kind}-quality"
        type="range"
        min="1"
        max="100"
        bind:value={watermarkQuality}
        class="range-input"
      />
    </div>
  </div>

  <div>
    <label class="field-label">{m.format()}</label>
    <div class="flex gap-2">
      {#each ['jpeg', 'png', 'tiff'] as fmt}
        <button
          type="button"
          class="toggle-btn flex-1 {watermarkFormat === fmt ? 'active' : ''}"
          onclick={() => {
            watermarkFormat = fmt as typeof watermarkFormat
          }}>{fmt.toUpperCase()}</button
        >
      {/each}
    </div>
  </div>

  <Button
    disabled={creating || authStore.role === 'viewer'}
    onclick={() =>
      handleCreate(kind, {
        opacity: watermarkOpacity,
        quality: watermarkQuality,
        format: watermarkFormat,
      })}
    class="w-full"
  >
    {creating ? m.queuing_() : m.variant_create_watermark()}
  </Button>
</div>

<style>
  .field-label {
    display: block;
    margin-bottom: 4px;
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--text-secondary);
  }
  .toggle-btn {
    border-radius: 7px;
    border: 1px solid var(--border);
    background: var(--bg-surface);
    color: var(--text-secondary);
    padding: 7px 0;
    font-size: 0.8125rem;
    font-weight: 500;
    transition: all 0.1s ease;
    cursor: pointer;
  }
  .toggle-btn:hover {
    border-color: var(--accent-cta);
    color: var(--text-primary);
  }
  .toggle-btn.active {
    border-color: var(--accent-cta);
    background: oklch(93% 0.04 270);
    color: oklch(40% 0.18 270);
  }
  :global(.dark) .toggle-btn.active {
    background: oklch(30% 0.08 270 / 0.4);
    color: oklch(78% 0.12 270);
  }
  .range-input {
    width: 100%;
    accent-color: var(--accent-cta);
  }
  .quality-val {
    font-weight: 600;
    color: var(--accent-cta);
  }
</style>
