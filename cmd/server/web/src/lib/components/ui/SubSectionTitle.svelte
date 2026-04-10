<script lang="ts">
    import type { Snippet } from "svelte"

    let { children = undefined }: { children?: Snippet } = $props();
</script>

<p class="mb-3 text-xs font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">
    {@render children?.()}
</p>
