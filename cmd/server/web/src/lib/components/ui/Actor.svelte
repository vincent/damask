<script lang="ts">
    import { Bot, User } from "@lucide/svelte"
    let { type }: { type: 'system' | 'user' } = $props();
</script>

<div class="mt-0.5 flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full
    {type === 'system'
        ? 'bg-zinc-100 text-zinc-400 dark:bg-zinc-800 dark:text-zinc-500'
        : 'bg-indigo-100 text-indigo-600 dark:bg-indigo-900/50 dark:text-indigo-400'}">
    {#if type === 'system'}
        <Bot class="h-4 w-4" />
    {:else}
        <User class="h-4 w-4" />
    {/if}
</div>
