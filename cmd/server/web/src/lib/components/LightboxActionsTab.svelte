<script lang="ts">
  import { assetApi, type Asset } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { RefreshCw, Share } from '@lucide/svelte'
  import SubSectionTitle from './ui/SubSectionTitle.svelte'
  import AssetExportImage from './AssetExportImage.svelte'
  import AssetDeleteButton from './AssetDeleteButton.svelte'
  import ShareModal from './ShareModal.svelte'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    asset: Asset
    onDeleted: (id: string) => void
  }

  let { asset, onDeleted }: Props = $props()

  let regenThumbLoading = $state(false)
  let showShareModal = $state(false)

  const shareTargets = $derived([
    { type: 'asset' as const, id: asset.id, label: m.selected_asset() },
  ])

  async function handleRegenerateThumbnail() {
    regenThumbLoading = true
    try {
      await assetApi.regenerateThumbnail(asset.id)
    } catch {
      /* silently ignore */
    } finally {
      regenThumbLoading = false
    }
  }
</script>

<div class="space-y-6 px-5 py-5">
  <div>
    <SubSectionTitle>{m.quick_actions()}</SubSectionTitle>
    <div class="space-y-2">
      <button
        class="text-md flex w-full items-center gap-3 rounded-xl border border-gray-200 px-4 py-3 text-gray-700 transition-colors hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800"
        onclick={() => {
          showShareModal = true
        }}
      >
        <Share class="h-4 w-4 shrink-0 text-gray-400 dark:text-gray-500" />
        {m.share()}
      </button>
      {#if authStore.role !== 'viewer'}
        <button
          class="text-md flex w-full items-center gap-3 rounded-xl border border-gray-200 px-4 py-3 text-gray-700 transition-colors hover:bg-gray-50 disabled:opacity-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800"
          onclick={handleRegenerateThumbnail}
          disabled={regenThumbLoading}
        >
          <RefreshCw
            class="h-4 w-4 shrink-0 text-gray-400 dark:text-gray-500"
          />
          {regenThumbLoading ? m.queuing_() : m.thumbnail_regen()}
        </button>
      {/if}
    </div>
  </div>

  <AssetExportImage {asset} />

  {#if authStore.role !== 'viewer'}
    <AssetDeleteButton {asset} ondeleted={onDeleted} />
  {/if}
</div>

{#if showShareModal}
  <ShareModal
    bind:open={showShareModal}
    targets={shareTargets}
    onclose={() => {
      showShareModal = false
    }}
  />
{/if}
