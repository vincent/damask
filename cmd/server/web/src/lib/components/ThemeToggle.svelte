<script lang="ts">
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
  aria-label={dark ? 'Switch to light mode' : 'Switch to dark mode'}
  class="flex items-center gap-2 rounded-lg px-2 text-md text-gray-400 hover:text-gray-800 dark:hover:text-gray-200"
>
  {#if dark}
    <Sun class="h-3.5 w-3.5" />
    <span>Light mode</span>
  {:else}
    <Moon class="h-3.5 w-3.5" />
    <span>Dark mode</span>
  {/if}
</button>
