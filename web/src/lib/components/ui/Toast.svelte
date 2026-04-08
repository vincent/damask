<script lang="ts">
  import { toastStore } from '$lib/stores/toast.svelte'
</script>

{#if toastStore.current}
  <div
    class="fixed bottom-4 right-4 z-500 rounded-lg px-4 py-3 text-md font-medium shadow-lg transition-all
      {toastStore.current.type === 'error'
        ? 'bg-red-600 text-white'
        : 'bg-gray-900 text-white dark:bg-gray-100 dark:text-gray-900'}"
  >
    {toastStore.current.msg}
  </div>
{/if}
