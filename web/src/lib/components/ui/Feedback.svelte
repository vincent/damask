<script lang="ts">
    let { error = undefined, success = undefined }: { success?: string, error?: string } = $props()
</script>

{#if error}
    <div class="mx-5 mt-4 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-900/30 dark:text-red-300">
        {error}
    </div>
{/if}
{#if success}
    <div class="mx-5 mt-4 rounded-lg bg-green-50 px-4 py-3 text-sm text-green-700 dark:bg-green-900/30 dark:text-green-300">
        {success}
    </div>
{/if}
