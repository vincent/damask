<script lang="ts">
  import { m } from '$lib/paraglide/messages'
  import { Moon, Sun } from '@lucide/svelte'
  import { themeStore } from '$lib/stores/theme.svelte'
</script>

<button
  onclick={() => themeStore.toggle()}
  aria-label={themeStore.dark ? m.theme_switch_light() : m.theme_switch_dark()}
  class="flex items-center gap-2 rounded-lg text-sm text-gray-400 hover:text-gray-800 dark:hover:text-gray-200"
>
  {#if themeStore.dark}
    <span>{m.theme_light()}</span>
    <Sun class="h-3.5 w-3.5" />
  {:else}
    <span>{m.theme_dark()}</span>
    <Moon class="h-3.5 w-3.5" />
  {/if}
</button>
