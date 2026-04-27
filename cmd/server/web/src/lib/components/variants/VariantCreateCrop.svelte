<script lang="ts">
    import Button from '$lib/components/ui/Button.svelte'
    import { authStore } from '$lib/stores/auth.svelte'
    import { assetApi, type Asset } from '$lib/api'
    import { m } from '$lib/paraglide/messages'

    interface Props {
        asset: Asset
        creating: boolean
        handleCreate: (kind: string, params: Record<string, unknown>) => void
    }

    let { asset, creating, handleCreate }: Props = $props()

    const kind = 'image_crop'

    let cropFormat = $state<'jpeg' | 'png'>('png')

    let img = $state<HTMLImageElement>()
    let imgLoaded = $state(false)

    // Displayed image dimensions (updated on load and resize)
    let dispW = $state(0)
    let dispH = $state(0)
    let scaleX = $state(1)
    let scaleY = $state(1)

    // Drag state in display coords (relative to image top-left)
    let dragging = $state(false)
    let startX = $state(0)
    let startY = $state(0)
    let endX = $state(0)
    let endY = $state(0)

    let box = $derived(endX !== startX || endY !== startY ? {
        x: Math.min(startX, endX),
        y: Math.min(startY, endY),
        w: Math.abs(endX - startX),
        h: Math.abs(endY - startY),
    } : null)

    let crop = $derived(box && scaleX > 0 && scaleY > 0 ? {
        x: Math.round(box.x / scaleX),
        y: Math.round(box.y / scaleY),
        width: Math.round(box.w / scaleX),
        height: Math.round(box.h / scaleY),
    } : null)

    function updateScale() {
        if (!img) return
        dispW = img.offsetWidth
        dispH = img.offsetHeight
        scaleX = dispW / img.naturalWidth
        scaleY = dispH / img.naturalHeight
    }

    function onImgLoad() {
        updateScale()
        imgLoaded = true
    }

    function pos(e: MouseEvent | TouchEvent): { x: number; y: number } {
        if (!img) return { x: 0, y: 0 }
        const rect = img.getBoundingClientRect()
        const client = 'touches' in e ? e.touches[0] : e
        return {
            x: Math.max(0, Math.min(client.clientX - rect.left, rect.width)),
            y: Math.max(0, Math.min(client.clientY - rect.top, rect.height)),
        }
    }

    function onDown(e: MouseEvent | TouchEvent) {
        if (!imgLoaded) return
        e.preventDefault()
        updateScale()
        const p = pos(e)
        startX = p.x; startY = p.y
        endX = p.x;   endY = p.y
        dragging = true
    }

    function onMove(e: MouseEvent | TouchEvent) {
        if (!dragging) return
        e.preventDefault()
        const p = pos(e)
        endX = p.x; endY = p.y
    }

    function onUp(e: MouseEvent | TouchEvent) {
        if (!dragging) return
        e.preventDefault()
        dragging = false
    }
</script>

<div class="space-y-5">
    <div class="flex justify-center rounded-xl border border-gray-200 bg-gray-50 p-3 select-none dark:border-gray-700 dark:bg-gray-800 min-h-40">
        <div class="relative">
        <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
        <img
            bind:this={img}
            src={assetApi.fileUrl(asset.id)}
            alt="Preview"
            class="max-h-[480px] w-full rounded object-contain cursor-crosshair block"
            draggable="false"
            onload={onImgLoad}
            onmousedown={onDown}
            onmousemove={onMove}
            onmouseup={onUp}
            onmouseleave={onUp}
            ontouchstart={onDown}
            ontouchmove={onMove}
            ontouchend={onUp}
        />

        {#if box && box.w > 2 && box.h > 2 && img}
            <svg
                class="pointer-events-none absolute inset-0 rounded"
                style="width:{dispW}px; height:{dispH}px;"
            >
                <defs>
                    <mask id="crop-mask">
                        <rect width="100%" height="100%" fill="white"/>
                        <rect x={box.x} y={box.y} width={box.w} height={box.h} fill="black"/>
                    </mask>
                </defs>
                <rect width="100%" height="100%" fill="rgba(0,0,0,0.45)" mask="url(#crop-mask)"/>
                <rect x={box.x} y={box.y} width={box.w} height={box.h} fill="none" stroke="white" stroke-width="1.5"/>
                {#each [1,2] as t}
                    <line x1={box.x + box.w * t/3} y1={box.y} x2={box.x + box.w * t/3} y2={box.y + box.h} stroke="rgba(255,255,255,0.4)" stroke-width="1"/>
                    <line x1={box.x} y1={box.y + box.h * t/3} x2={box.x + box.w} y2={box.y + box.h * t/3} stroke="rgba(255,255,255,0.4)" stroke-width="1"/>
                {/each}
                {#each [[box.x, box.y], [box.x+box.w, box.y], [box.x, box.y+box.h], [box.x+box.w, box.y+box.h]] as [cx, cy]}
                    <rect x={cx-4} y={cy-4} width="8" height="8" fill="white" rx="1"/>
                {/each}
            </svg>
        {/if}
        </div>
    </div>

    {#if crop}
        <p class="text-sm text-gray-400 dark:text-gray-500">
            {crop.x}, {crop.y} — {crop.width} × {crop.height} px
        </p>
    {:else}
        <p class="text-sm text-gray-400 dark:text-gray-500">Draw a selection on the image</p>
    {/if}

    <div>
        <label for="crop-format" class="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-400">{m.format()}</label>
        <div id="crop-format" class="flex gap-2">
            {#each ['jpeg', 'png'] as fmt}
                <button type="button"
                    class="flex-1 rounded-lg border py-2 text-sm font-medium transition-colors {cropFormat === fmt
                        ? 'border-indigo-500 bg-indigo-50 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300'
                        : 'border-gray-300 text-gray-600 hover:border-gray-400 dark:border-gray-600 dark:text-gray-400'}"
                    onclick={() => { cropFormat = fmt as typeof cropFormat }}
                >{fmt.toUpperCase()}</button>
            {/each}
        </div>
    </div>

    <Button
        disabled={creating || authStore.role === 'viewer' || !crop || crop.width < 1 || crop.height < 1}
        onclick={() => crop && handleCreate(kind, { ...crop, format: cropFormat, quality: 85 })}
        class="w-full"
    >
        {creating ? m.queuing_() : m.variant_create_crop()}
    </Button>
</div>
