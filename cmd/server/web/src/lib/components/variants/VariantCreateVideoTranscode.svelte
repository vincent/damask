<script lang="ts">
    import { type Asset } from '$lib/api'
    import { authStore } from '$lib/stores/auth.svelte'
    import Button from '$lib/components/ui/Button.svelte'
    import ResolutionOptions from '../ResolutionOptions.svelte';
    import { m } from '$lib/paraglide/messages'
    
    interface Props {
        asset: Asset
        creating: boolean
        handleCreate: (kind: string, params: Record<string, unknown>) => void
    }

    let { asset, creating, handleCreate }: Props = $props()

    const kind = 'video_transcode'

    // Video params
    let transcodeFormat = $state<'mp4' | 'webm'>('mp4')
    let transcodeResolution = $state<'' | '1080p' | '720p' | '480p'>('')
    let transcodeStripAudio = $state(false)

</script>

<div class="space-y-6">
    <div class="space-y-4 rounded-xl border border-gray-200 p-4 dark:border-gray-700">
        <h3 class="text-md font-semibold text-gray-800 dark:text-gray-200">{m.transcode()}</h3>
        <p class="text-sm text-gray-500 dark:text-gray-400">{m.transcode_hint()}</p>

        <div>
        <label for="variant-{kind}-format" class="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-400">{m.output_format()}</label>
            <div class="flex gap-2">
                {#each ['mp4', 'webm'] as fmt}
                    <button type="button"
                        class="flex-1 rounded-lg border py-2 text-sm font-medium transition-colors {transcodeFormat === fmt
                        ? 'border-indigo-500 bg-indigo-50 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300'
                        : 'border-gray-300 text-gray-600 hover:border-gray-400 dark:border-gray-600 dark:text-gray-400'}"
                        onclick={() => { transcodeFormat = fmt as typeof transcodeFormat }}
                    >{fmt.toUpperCase()}</button>
                {/each}
            </div>
        </div>

        <div>
        <label for="variant-{kind}-resolution" class="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-400">{m.resolution()} <span class="text-gray-400">({m.optional()})</span></label>
            <select id="variant-{kind}-resolution" bind:value={transcodeResolution}
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-md focus:border-indigo-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100">
                <ResolutionOptions />
            </select>
        </div>
        
        <label class="flex items-center gap-2 text-md text-gray-700 dark:text-gray-300">
            <input type="checkbox" bind:checked={transcodeStripAudio} class="rounded" />
            {m.strip_audio()}
        </label>

        <Button disabled={creating || authStore.role === 'viewer'} onclick={() => handleCreate(kind, { format: transcodeFormat, resolution: transcodeResolution || undefined, strip_audio: transcodeStripAudio })} class="w-full">
        {creating ? m.queuing_() : m.transcode()}
        </Button>
    </div>
</div>