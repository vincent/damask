<script lang="ts">
  import { type Asset } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import VariantFormatSelect from './VariantFormatSelect.svelte'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    asset: Asset
    creating: boolean
    handleCreate: (kind: string, params: Record<string, unknown>) => void
  }

  let { asset, creating, handleCreate }: Props = $props()

  const kind = 'normalize_audio'
  const formats = [
    { value: 'source', label: m.audio_format_source() },
    { value: 'mp3', label: 'MP3' },
    { value: 'aac', label: 'AAC' },
    { value: 'wav', label: 'WAV' },
  ]
  const targets = [
    { value: -14, label: m.audio_target_streaming() },
    { value: -16, label: m.audio_target_general() },
    { value: -23, label: m.audio_target_broadcast() },
  ]

  let targetLUFS = $state(-16)
  let format = $state('source')
</script>

<div class="space-y-5">
  <div>
    <h3 class="text-md font-semibold text-gray-800 dark:text-gray-200">
      {m.audio_normalize_title()}
    </h3>
    <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
      {m.audio_normalize_description()}
    </p>
  </div>

  <div>
    <label
      for="variant-{kind}-target"
      class="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-400"
    >
      {m.audio_target_label()}
    </label>
    <select
      id="variant-{kind}-target"
      value={targetLUFS}
      class="text-md w-full rounded-lg border border-gray-300 px-3 py-2 focus:border-indigo-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
      onchange={(e) =>
        (targetLUFS = Number((e.currentTarget as HTMLSelectElement).value))}
    >
      {#each targets as target}
        <option value={target.value}>{target.label}</option>
      {/each}
    </select>
  </div>

  <VariantFormatSelect
    id="variant-{kind}-format"
    value={format}
    options={formats}
    label={m.audio_output_format_label()}
    onchange={(next) => (format = next)}
  />

  <p
    class="rounded-lg bg-gray-50 px-3 py-2 text-sm text-gray-500 dark:bg-gray-800 dark:text-gray-400"
  >
    {m.audio_normalize_explainer()}
  </p>

  <Button
    disabled={creating || authStore.role === 'viewer'}
    onclick={() => handleCreate(kind, { target_lufs: targetLUFS, format })}
    class="w-full"
  >
    {creating ? m.queuing_() : m.audio_normalize_submit()}
  </Button>
</div>
