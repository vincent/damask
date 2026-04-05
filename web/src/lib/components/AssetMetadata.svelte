<script lang="ts">
  import { type Asset } from '$lib/api'
  
  interface Props {
    asset: Asset
  }

  let { asset }: Props = $props()
</script>

<div>
    <p class="mb-3 text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">Metadata</p>
    <div class="grid grid-cols-3 gap-3">
        <div class="rounded-xl bg-gray-50 px-3 py-3 dark:bg-gray-800">
            <p class="mb-1 text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">Created By</p>
            <p class="text-sm font-semibold text-gray-900 dark:text-gray-100">—</p>
        </div>
        <div class="rounded-xl bg-gray-50 px-3 py-3 dark:bg-gray-800">
            <p class="mb-1 text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">Date Added</p>
            <p class="text-sm font-semibold text-gray-900 dark:text-gray-100">
                {new Date(asset.created_at).toLocaleDateString()}
            </p>
        </div>
        <div class="rounded-xl bg-gray-50 px-3 py-3 dark:bg-gray-800">
            <p class="mb-1 text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">Modified</p>
            <p class="text-sm font-semibold text-gray-900 dark:text-gray-100">
                {new Date(asset.updated_at ?? asset.created_at).toLocaleDateString()}
            </p>
        </div>
        {#if asset.width != null && asset.height != null}
            <div class="rounded-xl bg-gray-50 px-3 py-3 dark:bg-gray-800">
                <p class="mb-1 text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">Dimensions</p>
                <p class="text-sm font-semibold text-gray-900 dark:text-gray-100">{asset.width} × {asset.height}</p>
            </div>
        {/if}
    </div>
</div>

