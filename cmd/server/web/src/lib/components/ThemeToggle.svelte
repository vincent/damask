<script lang="ts">
  import { m } from '$lib/paraglide/messages'
  import { Moon, Sun } from '@lucide/svelte'

  const STORAGE_KEY = 'theme'

  let dark = $state(false)

  $effect(() => {
    const stored = localStorage.getItem(STORAGE_KEY)
    dark = stored === 'dark' || (!stored && window.matchMedia('(prefers-color-scheme: dark)').matches)
    applyTheme(dark)
  })

  function applyTheme(isDark: boolean) {
    document.documentElement.classList.toggle('dark', isDark)
  }

  function toggle() {
    dark = !dark
    localStorage.setItem(STORAGE_KEY, dark ? 'dark' : 'light')
    applyTheme(dark)
  }
</script>

<button
  onclick={toggle}
  aria-label={dark ? m.theme_switch_light() : m.theme_switch_dark()}
  class="flex items-center gap-2 rounded-lg text-sm text-gray-400 hover:text-gray-800 dark:hover:text-gray-200"
>
  {#if dark}
    <span>{m.theme_light()}</span>
    <Sun class="h-3.5 w-3.5" />
  {:else}
    <span>{m.theme_dark()}</span>
    <Moon class="h-3.5 w-3.5" />
  {/if}
</button>
