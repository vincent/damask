<script lang="ts">
    import { authStore } from '$lib/stores/auth.svelte'
    import Button from '$lib/components/ui/Button.svelte'
    import { type Asset } from '$lib/api'
  import { m } from '$lib/paraglide/messages'
    
    interface Props {
        asset: Asset
        creating: boolean
        handleCreate: (kind: string, params: Record<string, unknown>) => void
    }

    let { asset, creating, handleCreate }: Props = $props()

    const kind = 'video_capture_image'

  // Video params
  let videoTimestamp = $state(1)
</script>

<div class="space-y-6">
    <div class="space-y-4 rounded-xl border border-gray-200 p-4 dark:border-gray-700">
        <h3 class="text-md font-semibold text-gray-800 dark:text-gray-200">{m.extract_frame()}</h3>
        <div>
            <label for="variant-{kind}-timestamp" class="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-400">{m.timestamp_seconds()}</label>
            <input id="variant-{kind}-timestamp" type="number" min="0" step="0.1" bind:value={videoTimestamp}
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-md focus:border-indigo-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100" />
        </div>
        <Button disabled={creating || authStore.role === 'viewer'} onclick={() => handleCreate(kind, { timestamp: videoTimestamp })} class="w-full">
            {creating ? m.queuing_() : m.extract_frame()}
        </Button>
    </div>
</div>