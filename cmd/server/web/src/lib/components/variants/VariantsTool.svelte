<script lang="ts">
  import { type Asset } from '$lib/api'
  import VariantCreateImageResize from './VariantCreateImageResize.svelte'
  import VariantCreateImageWatermark from './VariantCreateImageWatermark.svelte'
  import VariantCreateImageConvert from './VariantCreateImageConvert.svelte'
  import VariantCreateImageCrop from './VariantCreateImageCrop.svelte'
  import VariantCreateImageRemoveBackground from './VariantCreateImageRemoveBackground.svelte'
  import VariantCreateImageWithPrompt from './VariantCreateImageWithPrompt.svelte'
  import VariantCreateImageSmartCrop from './VariantCreateImageSmartCrop.svelte'
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
    onDone?: () => void
    onDraftStarted?: (nonce: string) => void
    sessionActive?: boolean
  }
  let {
    asset,
    tool,
    creating = false,
    handleCreate,
    onDone,
    onDraftStarted,
    sessionActive,
  }: Props = $props()
</script>

{#if tool === 'resize'}
  <VariantCreateImageResize {asset} {creating} {handleCreate} />
{:else if tool === 'watermark'}
  <VariantCreateImageWatermark
    {asset}
    {onDone}
    {onDraftStarted}
    {sessionActive}
  />
{:else if tool === 'convert'}
  <VariantCreateImageConvert {asset} {creating} {handleCreate} />
{:else if tool === 'crop'}
  <VariantCreateImageCrop {asset} {creating} {handleCreate} />
{:else if tool === 'bg_remove'}
  <VariantCreateImageRemoveBackground
    {asset}
    {onDone}
    {onDraftStarted}
    {sessionActive}
  />
{:else if tool === 'image_with_prompt'}
  <VariantCreateImageWithPrompt
    {asset}
    {onDone}
    {onDraftStarted}
    {sessionActive}
  />
{:else if tool === 'smart_crop'}
  <VariantCreateImageSmartCrop {asset} {creating} {handleCreate} />
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
