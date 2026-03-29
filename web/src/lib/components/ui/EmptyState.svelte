<script lang="ts">
  import type { Snippet } from 'svelte'

  interface Props {
    title: string
    description?: string
    icon?: Snippet
    action?: Snippet
  }

  let { title, description, icon, action }: Props = $props()
</script>

<div class="flex flex-col items-center justify-center py-24 text-center">
  {#if icon}
    <div class="mb-4 text-gray-300 dark:text-gray-600">
      {@render icon()}
    </div>
  {/if}
  <p class="text-sm font-medium text-gray-600 dark:text-gray-300">{title}</p>
  {#if description}
    <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">{description}</p>
  {/if}
  {#if action}
    <div class="mt-4">
      {@render action()}
    </div>
  {/if}
</div>
