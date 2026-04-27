<script lang="ts">
    import type { Snippet } from "svelte"
    let { class: extraClass = '', children }: { class?: string; children: Snippet } = $props()
</script>

<p class={`text-sm md:text-md text-gray-500 dark:text-gray-400 ${extraClass}`}>{@render children()}</p>