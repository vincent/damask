<script lang="ts">
  import { type Asset, type Variant } from '$lib/api'
  import VariantCard from './variants/VariantCard.svelte'

  interface Props {
    asset: Asset
    variants: Variant[]
    selectedVariant: Variant | null
    onSelectVariant: (v: Variant) => void
    onVariantUpdated: (variant: Variant) => void
    onVariantsUpdated: (variants: Variant[]) => void
    deleteVariant: (variantId: string) => void
    promoteVariant: (variant: Variant) => void
    thumbnailUpdated: () => void
    rerunVariant: () => void
  }

  let {
    asset,
    variants,
    selectedVariant,
    onSelectVariant,
    onVariantUpdated,
    onVariantsUpdated,
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
      isSelected={selectedVariant?.id === v.id}
      onSelect={() => onSelectVariant(v)}
      {onVariantUpdated}
      {onVariantsUpdated}
      onDelete={() => deleteVariant(v.id)}
      onPromote={() => promoteVariant(v)}
      onThumbnailUpdated={thumbnailUpdated}
      onRerun={rerunVariant}
    />
  {/each}
</div>
