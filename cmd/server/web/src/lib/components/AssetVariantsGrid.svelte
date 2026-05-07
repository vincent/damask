<script lang="ts">
  import { type Asset, type Variant } from '$lib/api'
  import VariantCard from './variants/VariantCard.svelte'

  interface Props {
    asset: Asset
    variants: Variant[]
    deleteVariant: (variantId: string) => void
    promoteVariant: (variant: Variant) => void
    thumbnailUpdated: () => void
    rerunVariant: () => void
  }

  let {
    asset,
    variants,
    deleteVariant,
    promoteVariant,
    thumbnailUpdated,
    rerunVariant,
  }: Props = $props()
</script>

<div class="grid grid-cols-2 gap-3">
  {#each variants as v}
    <VariantCard
      variant={v}
      assetId={asset.id}
      onDelete={() => deleteVariant(v.id)}
      onPromote={() => promoteVariant(v)}
      onThumbnailUpdated={thumbnailUpdated}
      onRerun={rerunVariant}
    />
  {/each}
</div>
