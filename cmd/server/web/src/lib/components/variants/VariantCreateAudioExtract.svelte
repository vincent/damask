<script lang="ts">
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

  const kind = 'extract_audio'
  const formats = [
    { value: 'aac', label: 'AAC' },
    { value: 'mp3', label: 'MP3' },
    { value: 'opus', label: 'Opus' },
    { value: 'flac', label: 'FLAC' },
  ]

  let format = $state('aac')
  let bitrate = $state('192k')
</script>

<div class="space-y-5">
  <div>
    <h3 class="text-md font-semibold text-gray-800 dark:text-gray-200">
      {m.audio_extract_title()}
    </h3>
    <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
      {m.audio_extract_description()}
    </p>
  </div>

  <VariantFormatSelect
    id="variant-{kind}-format"
    value={format}
    options={formats}
    label={m.audio_format_label()}
    onchange={(next) => (format = next)}
  />

  <BitrateSelect
    id="variant-{kind}-bitrate"
    value={bitrate}
    label={m.audio_bitrate_label()}
    onchange={(next) => (bitrate = next)}
  />

  <Button
    disabled={creating || authStore.role === 'viewer'}
    onclick={() => handleCreate(kind, { format, bitrate })}
    class="w-full"
  >
    {creating ? m.queuing_() : m.audio_extract_submit()}
  </Button>
</div>
