<script lang="ts">
  import { type Asset } from '$lib/api'
  import VariantCreateResize from './VariantCreateResize.svelte'
  import VariantCreateWatermark from './VariantCreateWatermark.svelte'
  import VariantCreateConvert from './VariantCreateConvert.svelte'
  import VariantCreateCrop from './VariantCreateCrop.svelte'
  import VariantCreateRemoveBackground from './VariantCreateRemoveBackground.svelte'
  import VariantCreateImageWithPrompt from './VariantCreateImageWithPrompt.svelte'
  import VariantCreateSmartCrop from './VariantCreateSmartCrop.svelte'
  import VariantCreateVideoThumbnail from './VariantCreateVideoThumbnail.svelte'
  import VariantCreateVideoTranscode from './VariantCreateVideoTranscode.svelte'
  import VariantCreateVideoWatermark from './VariantCreateVideoWatermark.svelte'
  import VariantCreateAudioExtract from './VariantCreateAudioExtract.svelte'
  import VariantCreateAudioTranscode from './VariantCreateAudioTranscode.svelte'
  import VariantCreateAudioNormalize from './VariantCreateAudioNormalize.svelte'

  export type VariantTab =
    | 'all'
    | 'resize'
    | 'watermark'
    | 'convert'
    | 'smart_crop'
    | 'crop'
    | 'bg_remove'
    | 'image_with_prompt'
    | 'video_transcode'
    | 'video_watermark'
    | 'video_capture_image'
    | 'audio_extract'
    | 'audio_transcode'
    | 'audio_normalize'

  interface Props {
    asset: Asset
    tool: VariantTab
    creating?: boolean
    handleCreate: (type: string, params: object) => void
  }
  let { asset, tool, creating = false, handleCreate }: Props = $props()
</script>

{#if tool === 'resize'}
  <VariantCreateResize {asset} {creating} {handleCreate} />
{:else if tool === 'watermark'}
  <VariantCreateWatermark {asset} {creating} {handleCreate} />
{:else if tool === 'convert'}
  <VariantCreateConvert {asset} {creating} {handleCreate} />
{:else if tool === 'crop'}
  <VariantCreateCrop {asset} {creating} {handleCreate} />
{:else if tool === 'bg_remove'}
  <VariantCreateRemoveBackground {asset} {creating} {handleCreate} />
{:else if tool === 'image_with_prompt'}
  <VariantCreateImageWithPrompt {asset} {creating} {handleCreate} />
{:else if tool === 'smart_crop'}
  <VariantCreateSmartCrop {asset} {creating} {handleCreate} />
{:else if tool === 'video_transcode'}
  <VariantCreateVideoTranscode {asset} {creating} {handleCreate} />
{:else if tool === 'video_watermark'}
  <VariantCreateVideoWatermark {asset} {creating} {handleCreate} />
{:else if tool === 'video_capture_image'}
  <VariantCreateVideoThumbnail {asset} {creating} {handleCreate} />
{:else if tool === 'audio_extract'}
  <VariantCreateAudioExtract {asset} {creating} {handleCreate} />
{:else if tool === 'audio_transcode'}
  <VariantCreateAudioTranscode {asset} {creating} {handleCreate} />
{:else if tool === 'audio_normalize'}
  <VariantCreateAudioNormalize {asset} {creating} {handleCreate} />
{/if}
