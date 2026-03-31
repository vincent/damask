<script lang="ts">
    type Props = {
        x: number
        y: number
        zoom: number
        maxZoom: number
    }
    let { x, y, zoom, maxZoom }: Props = $props();
</script>

<div class="mb-10">
    <div class="mb-4 flex items-center gap-3">
        <div class="h-8 w-8 animate-pulse rounded-lg bg-gray-200 dark:bg-gray-700"></div>
        <div class="h-4 w-32 animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
    </div>
    {#each { length: y } as _}
        <div class="mb-6 grid gap-3 grid-cols-{1 + maxZoom - Math.floor(zoom)}">
            {#each { length: x } as _}
                <div class="overflow-hidden rounded-xl bg-white shadow-sm dark:bg-gray-800">
                    <div class="animate-pulse bg-gray-200 dark:bg-gray-700" style="aspect-ratio: 4/3"></div>
                    <div class="p-3">
                    <div class="mb-2 h-3 w-3/4 animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
                    <div class="h-3 w-1/2 animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
                    </div>
                </div>
            {/each}
        </div>
    {/each}
</div>

