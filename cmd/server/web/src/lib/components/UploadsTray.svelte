<script lang="ts">
  import { m } from "$lib/paraglide/messages"
  import { uploadsStore } from "$lib/stores/uploads.svelte"
</script>

{#if uploadsStore.items.length > 0}
  <div class="fixed bottom-4 right-1 transform z-50">
    <div class="bg-white dark:bg-gray-800 rounded-lg shadow-lg p-4 w-[300px]">
        <h3 class="font-semibold mb-2 text-md text-gray-400">{m.uploads()}</h3>
        <ul class="divide-y divide-gray-200 dark:divide-gray-700 max-h-60 overflow-y-auto">
          {#each uploadsStore.items as item (item.id)}
            <li class="py-2 flex items-center gap-3">
              <div class="min-w-0 flex-1">
                <p class="truncate text-sm font-medium text-gray-800 dark:text-gray-200">{item.file.name}</p>
                {#if item.status === 'uploading'}
                  <div class="mt-1 h-1.5 overflow-hidden rounded-full bg-gray-200 dark:bg-gray-700">
                    <div
                      class="h-full rounded-full bg-blue-500 transition-all"
                      style="width: {item.progress}%"
                    ></div>
                  </div>
                {/if}
              </div>
              {#if item.status === 'uploading'}
                <span class="text-sm text-gray-500 dark:text-gray-400">{item.progress}%</span>
              {:else if item.status === 'done'}
                <span class="text-sm text-green-500">{m.upload_done()}</span>
              {:else if item.status === 'error'}
                <span class="text-sm text-red-500">{m.error()}</span>
              {/if}
            </li>
          {/each}
        </ul>
    </div>
  </div>
{/if}
