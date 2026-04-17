<script lang="ts">
  import { type Asset } from '$lib/api'
  import { m } from '$lib/paraglide/messages'
  import SubSectionTitle from './ui/SubSectionTitle.svelte'
  
  interface Props {
    asset: Asset
  }

  let { asset }: Props = $props()
</script>

<div>
    <SubSectionTitle>{m.metadata()}</SubSectionTitle>
    <div class="grid grid-cols-3 gap-3">
        <div class="rounded-xl bg-gray-50 px-3 py-3 dark:bg-gray-800">
            <p class="mb-1 text-xs font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">{m.created_by()}</p>
            <p class="text-md font-semibold text-gray-900 dark:text-gray-100">—</p>
        </div>
        <div class="rounded-xl bg-gray-50 px-3 py-3 dark:bg-gray-800">
            <p class="mb-1 text-xs font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">{m.date_added()}</p>
            <p class="text-md font-semibold text-gray-900 dark:text-gray-100">
                {new Date(asset.created_at).toLocaleDateString()}
            </p>
        </div>
        <div class="rounded-xl bg-gray-50 px-3 py-3 dark:bg-gray-800">
            <p class="mb-1 text-xs font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">{m.modified()}</p>
            <p class="text-md font-semibold text-gray-900 dark:text-gray-100">
                {new Date(asset.updated_at ?? asset.created_at).toLocaleDateString()}
            </p>
        </div>
        {#if asset.width != null && asset.height != null}
            <div class="rounded-xl bg-gray-50 px-3 py-3 dark:bg-gray-800">
                <p class="mb-1 text-xs font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">{m.dimensions()}</p>
                <p class="text-md font-semibold text-gray-900 dark:text-gray-100">{asset.width} × {asset.height}</p>
            </div>
        {/if}
    </div>
</div>

