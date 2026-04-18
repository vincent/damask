<script lang="ts">
  import { type Asset } from '$lib/api'
  import { assetsStore } from '$lib/stores/assets.svelte'
  import { selectionStore } from '$lib/stores/selection.svelte'
  import { uploadsStore } from '$lib/stores/uploads.svelte'
  import AssetCard from '$lib/components/AssetCard.svelte'
  import GridSkeleton from '$lib/components/ui/GridSkeleton.svelte'
  import EmptyState from '$lib/components/ui/EmptyState.svelte'
  import AssetIcon from '$lib/components/AssetIcon.svelte'
  import OnboardingScreen from '$lib/components/OnboardingScreen.svelte'
  import { CATEGORY_BORDER, CATEGORY_ICON_BG, CATEGORY_LABELS, CATEGORY_ORDER } from '$lib/stores/shared'
  import { CloudUpload, Inbox, Loader } from '@lucide/svelte'
  import { m } from '$lib/paraglide/messages'

  const fmt = new Intl.DateTimeFormat(undefined, { month: 'long', year: 'numeric' })

  const SIZE_BUCKETS: { label: string; min: number }[] = [
    { label: '> 1 GB',   min: 1_000_000_000 },
    { label: '> 500 MB', min:   500_000_000 },
    { label: '> 100 MB', min:   100_000_000 },
    { label: '> 10 MB',  min:    10_000_000 },
    { label: '> 1 MB',   min:     1_000_000 },
    { label: '< 1 MB',   min:             0 },
  ]

  const assetsBySize = $derived.by(() => {
    if (sort !== 'size') return []
    const groups: { label: string; assets: Asset[] }[] = []
    for (let i = 0; i < SIZE_BUCKETS.length; i++) {
      const { label, min } = SIZE_BUCKETS[i]
      const max = i === 0 ? Infinity : SIZE_BUCKETS[i - 1].min
      const assets = assetsStore.assets.filter((a) => a.size >= min && a.size < max)
      if (assets.length > 0) groups.push({ label, assets })
    }
    return groups
  })

  const assetsByMonth = $derived.by(() => {
    if (sort === 'created_at') {
      const groups: { label: string; assets: Asset[] }[] = []
      for (const asset of assetsStore.assets) {
        const label = fmt.format(new Date(asset.created_at))
        if (groups.length === 0 || groups[groups.length - 1].label !== label) {
          groups.push({ label, assets: [asset] })
        } else {
          groups[groups.length - 1].assets.push(asset)
        }
      }
      return groups
    }
    return []
  })

  type Props = {
    zoom: number
    maxZoom: number
    seenSplashScreen: boolean
    sort: 'mimetype' | 'created_at' | 'size' | 'taken_at'
    isDraggingFiles: boolean
    mainEl?: HTMLElement
    onCardClick: (asset: Asset, index: number, event: MouseEvent) => void
    onDismissOnboarding: () => void
    onDragOver: (e: DragEvent) => void
    onDragLeave: (e: DragEvent) => void
    onDrop: (e: DragEvent) => void
    onMouseDown: (e: MouseEvent) => void
  }

  let {
    zoom,
    maxZoom,
    seenSplashScreen,
    sort,
    isDraggingFiles,
    mainEl = $bindable(),
    onCardClick,
    onDismissOnboarding,
    onDragOver,
    onDragLeave,
    onDrop,
    onMouseDown,
  }: Props = $props()

  let sentinel = $state<HTMLDivElement | undefined>(undefined)

  $effect(() => {
    void assetsStore.resetDone  // re-create observer after each reset so it fires immediately if sentinel is already visible
    if (!sentinel) return
    const el = sentinel
    const observer = new IntersectionObserver(
      async (entries) => {
        if (entries[0].isIntersecting && assetsStore.nextCursor && !assetsStore.loading) {
          await assetsStore.load()
        }
      },
      { rootMargin: '200px' },
    )
    observer.observe(el)
    return () => observer.disconnect()
  })
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<main
  bind:this={mainEl}
  class="library-content relative flex-1 overflow-y-auto px-6 py-6"
  ondragover={onDragOver}
  ondragleave={onDragLeave}
  ondrop={onDrop}
  onmousedown={onMouseDown}
>
  {#snippet assetCardGrid(assets: Asset[])}
    <div class="pt-1 grid gap-3 grid-cols-{1 + maxZoom - Math.floor(zoom)}">
      {#each assets as asset (asset.id)}
        {@const globalIndex = assetsStore.assets.indexOf(asset)}
        <div class="relative" data-asset-id={asset.id}>
          {#if selectionStore.selectedIds.has(asset.id)}
            <div class="pointer-events-none absolute inset-0 z-5 rounded-lg ring-2 ring-indigo-500">
              <div class="absolute right-1.5 top-1.5 flex h-5 w-5 items-center justify-center rounded-full bg-indigo-600">
                <svg class="h-3 w-3 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7" />
                </svg>
              </div>
            </div>
          {/if}
          <AssetCard {asset} {zoom} requiresFields={uploadsStore.recentlyUploadedIds.has(asset.id)} onclick={(e) => onCardClick(asset, globalIndex, e)} />
        </div>
      {/each}
    </div>
  {/snippet}

  {#if isDraggingFiles}
    <div class="pointer-events-none absolute inset-0 z-20 flex items-center justify-center bg-indigo-50/80 ring-2 ring-inset ring-indigo-400 dark:bg-indigo-950/80">
      <div class="flex flex-col items-center gap-2 text-indigo-600 dark:text-indigo-400">
        <CloudUpload class="h-10 w-10" />
        <p class="text-md font-medium">{m.drop_to_upload()}</p>
      </div>
    </div>
  {/if}

  {#if assetsStore.initialLoad}
    <GridSkeleton x={7} y={3} {zoom} {maxZoom} />
  {:else if !seenSplashScreen}
    <OnboardingScreen onDismiss={onDismissOnboarding} />
  {:else if assetsStore.assets.length === 0}
    <EmptyState
      title={assetsStore.query ? `No results for "${assetsStore.query}"` : m.no_assets_yet()}
      description={assetsStore.query ? m.search_try_different() : m.upload_get_started()}
    >
      {#snippet icon()}<Inbox class="h-16 w-16" />{/snippet}
    </EmptyState>
  {:else if sort === 'mimetype'}
    {#each CATEGORY_ORDER as cat}
      {@const group = assetsStore.assetsByCategory[cat]}
      {#if group.length > 0}
        <div class="mb-10">
          <div class="sticky top-[-25px] z-10 bg-gray-50 dark:bg-gray-950 py-2 flex items-center gap-3">
            <AssetIcon category={cat} class="h-8 w-8 items-center justify-center rounded-lg {CATEGORY_ICON_BG[cat].light} {CATEGORY_ICON_BG[cat].dark}"/>
            <h2 class="text-md font-semibold text-gray-900 dark:text-gray-50">{CATEGORY_LABELS[cat]}</h2>
            <span class="text-md text-gray-400 dark:text-gray-500">{group.length}</span>
          </div>
          <div class="border-l-2 {CATEGORY_BORDER[cat]} pl-4">
            {@render assetCardGrid(group)}
          </div>
        </div>
      {/if}
    {/each}
    <div bind:this={sentinel} class="flex justify-center py-6">
      {#if assetsStore.loading && assetsStore.nextCursor}
        <Loader class="h-6 w-6 animate-spin text-gray-400" />
      {/if}
    </div>
  {:else if sort === 'taken_at'}
    {@render assetCardGrid(assetsStore.assets)}
    <div bind:this={sentinel} class="flex justify-center py-6">
      {#if assetsStore.loading && assetsStore.nextCursor}
        <Loader class="h-6 w-6 animate-spin text-gray-400" />
      {/if}
    </div>
  {:else if sort === 'created_at'}
    {#each assetsByMonth as group}
      <div class="mb-10">
        <div class="sticky top-[-25px] z-10 bg-gray-50 dark:bg-gray-950 py-2 flex items-center gap-2">
          <h2 class="text-md font-semibold text-gray-900 dark:text-gray-50">{group.label}</h2>
          <span class="text-md text-gray-400 dark:text-gray-500">{group.assets.length}</span>
        </div>
        {@render assetCardGrid(group.assets)}
      </div>
    {/each}
    <div bind:this={sentinel} class="flex justify-center py-6">
      {#if assetsStore.loading && assetsStore.nextCursor}
        <Loader class="h-6 w-6 animate-spin text-gray-400" />
      {/if}
    </div>
  {:else}
    {#each assetsBySize as group}
      <div class="mb-10">
        <div class="sticky top-[-25px] z-10 bg-gray-50 dark:bg-gray-950 py-2 flex items-center gap-2">
          <h2 class="text-md font-semibold text-gray-900 dark:text-gray-50">{group.label}</h2>
          <span class="text-md text-gray-400 dark:text-gray-500">{group.assets.length}</span>
        </div>
        {@render assetCardGrid(group.assets)}
      </div>
    {/each}
    <div bind:this={sentinel} class="flex justify-center py-6">
      {#if assetsStore.loading && assetsStore.nextCursor}
        <Loader class="h-6 w-6 animate-spin text-gray-400" />
      {/if}
    </div>
  {/if}


  <div class="hidden force-tailwind-to-include-these
    grid-cols-1  grid-cols-2  grid-cols-3  grid-cols-4  grid-cols-5
    grid-cols-6  grid-cols-7  grid-cols-8  grid-cols-9  grid-cols-10
    grid-cols-11 grid-cols-12 grid-cols-13 grid-cols-14 grid-cols-15
    grid-cols-16 grid-cols-17 grid-cols-18 grid-cols-19 grid-cols-20 grid-cols-21
  "></div>
</main>
