<script lang="ts">
  import { fly } from 'svelte/transition'
  import { type Asset } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import VariantFormatSelect from './VariantFormatSelect.svelte'
  import BitrateSelect from './BitrateSelect.svelte'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    asset: Asset
    creating: boolean
    handleCreate: (kind: string, params: Record<string, unknown>) => void
  }

  let { asset, creating, handleCreate }: Props = $props()

  const kind = 'transcode_audio'
  const formats = [
    { value: 'mp3', label: 'MP3' },
    { value: 'aac', label: 'AAC' },
    { value: 'opus', label: 'Opus' },
    { value: 'ogg', label: 'OGG' },
    { value: 'flac', label: 'FLAC' },
    { value: 'wav', label: 'WAV' },
  ]

  let format = $state('mp3')
  let bitrate = $state('192k')
  let mono = $state(false)
  const lossless = $derived(format === 'flac' || format === 'wav')
</script>

<div class="space-y-5">
  <div>
    <h3 class="text-md font-semibold text-gray-800 dark:text-gray-200">
      {m.audio_transcode_title()}
    </h3>
    <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
      {m.audio_transcode_description()}
    </p>
  </div>

  <VariantFormatSelect
    id="variant-{kind}-format"
    value={format}
    options={formats}
    label={m.audio_format_label()}
    onchange={(next) => (format = next)}
  />

  {#if lossless}
    <p
      transition:fly={{ y: -6, duration: 140 }}
      class="rounded-lg bg-gray-50 px-3 py-2 text-sm text-gray-500 dark:bg-gray-800 dark:text-gray-400"
    >
      {m.audio_lossless_note()}
    </p>
  {:else}
    <div transition:fly={{ y: -6, duration: 140 }}>
      <BitrateSelect
        id="variant-{kind}-bitrate"
        value={bitrate}
        label={m.audio_bitrate_label()}
        onchange={(next) => (bitrate = next)}
      />
    </div>
  {/if}

  <label
    class="text-md flex items-center gap-2 text-gray-700 dark:text-gray-300"
  >
    <input type="checkbox" bind:checked={mono} class="rounded" />
    {m.audio_mono_label()}
  </label>

  <Button
    disabled={creating || authStore.role === 'viewer'}
    onclick={() =>
      handleCreate(kind, {
        format,
        bitrate: lossless ? undefined : bitrate,
        mono,
      })}
    class="w-full"
  >
    {creating ? m.queuing_() : m.audio_transcode_submit()}
  </Button>
</div>
