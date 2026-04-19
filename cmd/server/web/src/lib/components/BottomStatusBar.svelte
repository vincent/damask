<script lang="ts">
    import { ZoomIn, ZoomOut, Languages } from "@lucide/svelte"
    import { setLocale, getLocale, locales } from "$lib/paraglide/runtime"
    import { statusBarStore as s } from "$lib/stores/bottomStatusBar.svelte"
    import Hint from "./ui/Hint.svelte"
    import ThemeToggle from "./ThemeToggle.svelte"
    import { gModeActive } from "$lib/shortcuts/sequence"
    import { fade } from "svelte/transition"

    let langOpen = $state(false)
    let currentLocale = $derived(getLocale())
</script>

<div class="absolute z-10 bottom-0 right-0 left-0 flex items-center justify-between border-t border-gray-200 bg-white px-4 py-3 dark:border-gray-800 dark:bg-gray-900 opacity-[92%]">
    {#if $gModeActive}
      <div transition:fade={{ duration: 100 }} class="flex items-center gap-1 text-xs text-zinc-500 dark:text-zinc-400">
        <kbd class="rounded border border-zinc-300 bg-zinc-100 px-1 py-px font-mono dark:border-zinc-600 dark:bg-zinc-800">g</kbd>
        <span>→</span>
        <span><kbd class="font-mono">l</kbd>ibrary</span>
        <span class="text-zinc-300 dark:text-zinc-600">·</span>
        <span><kbd class="font-mono">p</kbd>rojects</span>
        <span class="text-zinc-300 dark:text-zinc-600">·</span>
        <span><kbd class="font-mono">t</kbd>ags</span>
        <span class="text-zinc-300 dark:text-zinc-600">·</span>
        <span><kbd class="font-mono">s</kbd>ettings</span>
      </div>
    {:else if s.slot1}
        <Hint class="text-sm">
            {s.slot1}
        </Hint>
    {:else}
        <span></span>
    {/if}

    {#if s.showZoom}
        <div class="flex ml-auto mr-5 items-center gap-2 text-md text-gray-500 dark:text-gray-400">
            <ZoomOut class="h-4 w-4 cursor-pointer" onclick={() => s.zoomDecrease()} />
            <input class="w-full accent-blue-500"  type="range" min="0" max={s.sliderMax} bind:value={s.zoom} />
            <ZoomIn class="h-4 w-4 cursor-pointer" onclick={() => s.zoomIncrease()} />
        </div>
    {/if}

    <div class="flex items-center gap-3">
        <!-- lang selector dropup -->
        <div class="relative">
            <button
                onclick={() => langOpen = !langOpen}
                class="flex items-center gap-1 text-xs text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
            >
                <Languages class="h-3.5 w-3.5" />
            </button>
            {#if langOpen}
                <div class="absolute bottom-full mb-1 right-0 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded shadow-md overflow-hidden">
                    {#each locales as locale}
                        <button
                            onclick={() => { setLocale(locale); langOpen = false }}
                            class="block w-full px-3 py-1.5 text-xs text-left hover:bg-gray-100 dark:hover:bg-gray-700 {locale === currentLocale ? 'text-blue-500 font-medium' : 'text-gray-600 dark:text-gray-300'}"
                        >
                            {locale}
                        </button>
                    {/each}
                </div>
            {/if}
        </div>

        <ThemeToggle />
    </div>
</div>
