<script lang="ts">
  import { type Asset } from '$lib/api'
  import VariantCreateResize from './VariantCreateResize.svelte'
  import VariantCreateWatermark from './VariantCreateWatermark.svelte'
  import VariantCreateConvert from './VariantCreateConvert.svelte'
  import VariantCreateCrop from './VariantCreateCrop.svelte'
  import VariantCreateRemoveBackground from './VariantCreateRemoveBackground.svelte'
  import VariantCreateSmartCrop from './VariantCreateSmartCrop.svelte'
  import VariantCreateVideoThumbnail from './VariantCreateVideoThumbnail.svelte'
  import VariantCreateVideoTranscode from './VariantCreateVideoTranscode.svelte'

  export type VariantTab =
    | 'all'
    | 'resize'
    | 'watermark'
    | 'convert'
    | 'smart_crop'
    | 'crop'
    | 'bg_remove'
    | 'video_transcode'
    | 'video_capture_image'

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
{:else if tool === 'smart_crop'}
  <VariantCreateSmartCrop {asset} {creating} {handleCreate} />
{:else if tool === 'video_transcode'}
  <VariantCreateVideoTranscode {asset} {creating} {handleCreate} />
{:else if tool === 'video_capture_image'}
  <VariantCreateVideoThumbnail {asset} {creating} {handleCreate} />
{/if}
