<script lang="ts">
    import { X } from "@lucide/svelte"
    import type { Snippet } from "svelte"
    let { x = false, title = 'Cancel', class: extraClass, onclick, children }: { x?: boolean, onclick: () => void, title?: string, class?: string, children?: Snippet } = $props()
</script>

<button
    type="button"
    {onclick}
    {title}
    class={`rounded px-2 py-0.5 text-md text-zinc-500 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-800 ${extraClass}`}
>
    {#if x}
        <X class="h-3.5 w-3.5" />
    {:else if children}
        {@render children()}
    {:else}
        Cancel
    {/if}
</button>
