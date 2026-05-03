<script lang="ts">
  import { authStore } from '$lib/stores/auth.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import { variantApi, type Asset } from '$lib/api'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    asset: Asset
    creating: boolean
    handleCreate: (kind: string, params: Record<string, unknown>) => void
  }

  let { asset, creating, handleCreate }: Props = $props()

  const kind = 'image_watermark'

  // Watermark params
  let watermarkOpacity = $state(50)
  let watermarkQuality = $state(80)
  let watermarkFormat = $state<'jpeg' | 'png' | 'tiff'>('jpeg')
</script>

<div class="space-y-5">
  <div class="grid grid-cols-2 gap-4">
    <div>
      <label
        for="variant-{kind}-width"
        class="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-400"
        >{m.opacity()} ({watermarkOpacity}%)</label
      >
      <input
        id="variant-{kind}-width"
        type="range"
        min="1"
        max="100"
        bind:value={watermarkOpacity}
        class="w-full accent-indigo-500"
      />
    </div>
    <div>
      <label
        for="variant-{kind}-height"
        class="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-400"
        >{m.quality()}: {watermarkQuality}%</label
      >
      <input
        id="variant-{kind}-height"
        type="range"
        min="1"
        max="100"
        bind:value={watermarkQuality}
        class="w-full accent-indigo-500"
      />
    </div>
  </div>

  <div>
    <label
      for="variant-{kind}-format"
      class="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-400"
      >{m.format()}</label
    >
    <div class="flex gap-2">
      {#each ['jpeg', 'png', 'tiff'] as fmt}
        <button
          type="button"
          class="flex-1 rounded-lg border py-2 text-sm font-medium transition-colors {watermarkFormat ===
          fmt
            ? 'border-indigo-500 bg-indigo-50 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300'
            : 'border-gray-300 text-gray-600 hover:border-gray-400 dark:border-gray-600 dark:text-gray-400'}"
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
