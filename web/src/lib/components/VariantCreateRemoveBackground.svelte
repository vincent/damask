<script lang="ts">
    import { authStore } from '$lib/stores/auth.svelte'
    import Button from '$lib/components/ui/Button.svelte'
    import { type Asset } from '$lib/api'
    import Spinner from './ui/Spinner.svelte'
    
    interface Props {
        asset: Asset
        creating: boolean
        handleCreate: (kind: string, params: Record<string, unknown>) => void
    }

    let { asset, creating, handleCreate }: Props = $props()

    const kind = 'image_bg_remove'
</script>

<div class="space-y-5">
    <div class="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-md text-amber-800 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300">
        <p class="mb-1 font-medium">Requires Remove.bg API key</p>
        <p class="text-sm">Set <code class="rounded bg-amber-100 px-1 dark:bg-amber-800/50">REMOVEBG_API_KEY</code> in your server environment. Returns a transparent PNG.</p>
    </div>

    <Button disabled={creating || authStore.role === 'viewer'} onclick={() => handleCreate(kind, {})} class="w-full">
        {#if creating}<Spinner size="sm" />{/if}
        {creating ? 'Queuing…' : 'Remove Background'}
    </Button>
</div>
