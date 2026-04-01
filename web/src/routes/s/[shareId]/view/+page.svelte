<script lang="ts">
  import { onMount } from 'svelte'
  import { goto } from '$app/navigation'
  import { page } from '$app/state'
  import { Download, MessageSquare, Send } from '@lucide/svelte'
  import { formatBytes, mimeCategory } from '$lib/api'
  import Button from '$lib/components/ui/Button.svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import SharedAsset from '$lib/components/SharedAsset.svelte'
  import { publicViewStore as store, TYPES_BACKGROUNDS } from '$lib/stores/publicView.svelte'
  import PublicAssetCard from '$lib/components/PublicAssetCard.svelte'
  import PublicFooter from '$lib/components/PublicFooter.svelte'
  import AssetIcon from '$lib/components/AssetIcon.svelte'
  import Close from '$lib/components/ui/Close.svelte'
  import ThemeToggle from '$lib/components/ThemeToggle.svelte'

  let shareId = $derived(page.params.shareId || '')

  const avatarColors = [
    'bg-violet-200 text-violet-700 dark:bg-violet-800 dark:text-violet-200',
    'bg-blue-200 text-blue-700 dark:bg-blue-800 dark:text-blue-200',
    'bg-emerald-200 text-emerald-700 dark:bg-emerald-800 dark:text-emerald-200',
    'bg-amber-200 text-amber-700 dark:bg-amber-800 dark:text-amber-200',
  ]

  function formatDateTime(iso: string) {
    return new Date(iso).toLocaleString('en-US', {
      month: 'short',
      day: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
    })
  }

  function initials(name: string) {
    return name
      .split(' ')
      .slice(0, 2)
      .map((w) => w[0]?.toUpperCase() ?? '')
      .join('')
  }

  function avatarColor(name: string) {
    let hash = 0
    for (let i = 0; i < name.length; i++) hash = (hash * 31 + name.charCodeAt(i)) & 0xffffff
    return avatarColors[hash % avatarColors.length]
  }

  onMount(async () => {
    await store.init(shareId)
    store.loadGallery(shareId, () => goto(`/s/${shareId}`, { replaceState: true }))
  })

  function handleWindowKeydown(e: KeyboardEvent) {
    if (store.panelOpen && (e.key === 'ArrowLeft' || e.key === 'ArrowRight')) {
      e.preventDefault()
      store.navigateAsset(shareId, e.key === 'ArrowLeft' ? 'prev' : 'next')
    }
  }
</script>

<svelte:window onkeydown={handleWindowKeydown} />

<svelte:head>
  <title>{store.share?.label ?? 'Shared Gallery'} — Damask</title>
</svelte:head>

<div class="damask-texture relative flex min-h-screen flex-col">

  <!-- Header -->
  <header class="border-b border-gray-200 bg-white px-6 py-4 dark:border-gray-800 dark:bg-gray-900">
    <div class="mx-auto flex max-w-6xl items-center justify-between">
      <div>
        {#if store.share}
          <h1 class="text-xl font-bold text-gray-900 dark:text-gray-100">{store.share.label}</h1>
          <div class="mt-0.5 flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
            <span>{store.assets.length} asset{store.assets.length === 1 ? '' : 's'}</span>
            {#if store.expiryWarning}
              <span class="text-gray-300 dark:text-gray-600">•</span>
              <span class="flex items-center gap-1 text-amber-500 dark:text-amber-400">
                <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <circle cx="12" cy="12" r="10" /><path d="M12 6v6l4 2" stroke-linecap="round" />
                </svg>
                {store.expiryWarning}
              </span>
            {/if}
          </div>
        {:else if store.loadingGallery}
          <div class="h-5 w-48 animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
          <div class="mt-1 h-4 w-32 animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
        {/if}
      </div>

      <div class="flex items-center gap-2">
        {#if store.share?.allow_download && store.assets.length > 0}
          <Button variant="secondary" onclick={() => store.downloadAll(shareId)} size="md">
            {#snippet icon()}
              <Download class="h-4 w-4" />
            {/snippet}
            Download All
          </Button>
        {/if}

        <ThemeToggle />
      </div>
    </div>
  </header>

  <!-- Body -->
  <main class="mx-auto w-full max-w-6xl flex-1 px-6 py-8">
    {#if store.loadingGallery}
      <div class="flex items-center justify-center py-24">
        <Spinner size="lg" />
      </div>
    {:else if store.galleryError}
      <div class="flex flex-col items-center justify-center py-24 text-center">
        <p class="text-gray-500 dark:text-gray-400">{store.galleryError}</p>
      </div>
    {:else if store.assets.length === 0}
      <div class="flex flex-col items-center justify-center py-24 text-center">
        <p class="text-gray-500 dark:text-gray-400">No assets in this share.</p>
      </div>
    {:else}
      <div class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
        {#each store.assets as asset}
          <PublicAssetCard
            {asset}
            thumbUrl={store.thumbUrl(shareId, asset.id)}
            openAsset={() => store.openAsset(shareId, asset)}
          />
        {/each}
      </div>
    {/if}
  </main>

  <PublicFooter/>
</div>

<!-- Review Panel (S12) -->
{#if store.panelOpen && store.selectedAsset}
  {@const category = mimeCategory(store.selectedAsset.mime_type)}
  <!-- Backdrop -->
  <div
    class="fixed hidden md:grid md:w-[75%] inset-0 z-40 bg-black/40 backdrop-blur-sm grid place-items-center p-40"
    role="button"
    tabindex="-1"
    onclick={store.closePanel}
    onkeydown={(e) => e.key === 'Escape' && store.closePanel()}
  >
    <SharedAsset
      {category} asset={store.selectedAsset}
      thumbUrl={store.thumbUrl(shareId, store.selectedAsset.id)}
      assetUrl={store.fileUrlWithToken(shareId, store.selectedAsset.id)}
    />
  </div>

  <!-- Panel -->
  <!-- svelte-ignore a11y_no_noninteractive_element_to_interactive_role -->
  <aside
    class="fixed w-full md:w-[25%] max-w-full md:max-w-2xl inset-y-0 right-0 z-50 flex flex-col bg-white shadow-2xl dark:bg-gray-900"
    role="dialog"
    aria-label="Asset review"
  >
    <!-- Panel header -->
    <div class="flex items-center gap-3 border-b border-gray-200 px-4 py-3 dark:border-gray-800">
      <AssetIcon {category} />
      <div class="min-w-0 flex-1">
        <p class="truncate text-sm font-semibold text-gray-900 dark:text-gray-100" title={store.selectedAsset.original_filename}>
          {store.selectedAsset.original_filename}
        </p>
        <p class="text-xs text-gray-500 dark:text-gray-400">
          {formatBytes(store.selectedAsset.size)} • {category.toUpperCase()}
        </p>
      </div>
      <div class="flex items-center gap-1">
        {#if store.share?.allow_download}
          <a
            href={store.fileUrlWithToken(shareId, store.selectedAsset.id)}
            download={store.selectedAsset.original_filename}
            class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-gray-200"
            aria-label="Download"
          >
            <Download class="h-4 w-4" />
          </a>
        {/if}
        
        <Close close={store.closePanel} />
      </div>
    </div>

    <!-- Mobile Preview area -->
    <div class="block md:hidden flex-shrink-0 {TYPES_BACKGROUNDS[category]}" style="height: 220px">
      <div class="flex h-full items-center justify-center">
        <SharedAsset
          {category} asset={store.selectedAsset}
          thumbUrl={store.thumbUrl(shareId, store.selectedAsset.id)}
          assetUrl={store.fileUrlWithToken(shareId, store.selectedAsset.id)}
        />
      </div>
    </div>

    <!-- Comments section -->
    {#if store.share?.allow_comments}
      <div class="flex flex-1 flex-col overflow-hidden">
        <!-- Comments header -->
        <div class="flex items-center gap-2 px-4 py-3">
          <MessageSquare class="h-4 w-4 text-gray-500 dark:text-gray-400" />
          <span class="text-sm font-semibold text-gray-900 dark:text-gray-100">Comments</span>
          {#if store.comments.length > 0}
            <span class="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-gray-800 dark:text-gray-300">
              {store.comments.length}
            </span>
          {/if}
        </div>

        <!-- Comment list -->
        <div class="flex-1 overflow-y-auto px-4">
          {#if store.loadingComments}
            <div class="flex justify-center py-6">
              <Spinner size="sm" />
            </div>
          {:else if store.comments.length === 0}
            <p class="py-4 text-center text-sm text-gray-400 dark:text-gray-600">No comments yet. Be the first!</p>
          {:else}
            <div class="flex flex-col gap-4 pb-4">
              {#each store.comments as comment}
                <div class="flex gap-3">
                  <div
                    class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full text-xs font-bold {avatarColor(comment.author_name)}"
                  >
                    {initials(comment.author_name)}
                  </div>
                  <div class="flex-1">
                    <div class="flex items-center gap-2">
                      <span class="text-sm font-medium text-gray-900 dark:text-gray-100">{comment.author_name}</span>
                      <span class="text-xs text-gray-400">{formatDateTime(comment.created_at)}</span>
                    </div>
                    <div class="mt-1 rounded-xl rounded-tl-sm bg-gray-50 px-3 py-2 text-sm text-gray-700 dark:bg-gray-800 dark:text-gray-300">
                      {comment.body}
                    </div>
                  </div>
                </div>
              {/each}
            </div>
          {/if}
        </div>

        <!-- Comment form -->
        <div class="border-t border-gray-200 px-4 py-4 dark:border-gray-800">
          <p class="mb-3 text-sm font-medium text-gray-700 dark:text-gray-300">Leave a comment</p>

          {#if store.commentPosted}
            <div class="rounded-lg bg-emerald-50 px-3 py-2 text-sm text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300">
              Comment posted! Thank you.
            </div>
          {:else}
            <form onsubmit={(e) => { e.preventDefault(); store.postComment(shareId) }}>
              <div class="mb-2 grid grid-cols-2 gap-2">
                <Input
                  placeholder="Your name"
                  label="Name *"
                  bind:value={store.commentName}
                  error={store.commentNameError}
                  autocomplete="name"
                />
                <Input
                  placeholder="For notifications"
                  label="Email (optional)"
                  type="email"
                  bind:value={store.commentEmail}
                  autocomplete="email"
                />
              </div>
              <div class="mb-3">
                <label for="comment-body" class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
                  Message *
                </label>
                <textarea
                  bind:value={store.commentBody}
                  placeholder="Add your feedback…"
                  id="comment-body"
                  rows="3"
                  class="w-full resize-none rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 placeholder-gray-400 shadow-sm transition-colors focus:border-indigo-400 focus:outline-none focus:ring-2 focus:ring-indigo-200 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500 dark:focus:border-indigo-500 dark:focus:ring-indigo-900 {store.commentBodyError ? 'border-red-400 focus:ring-red-200 dark:border-red-500' : ''}"
                ></textarea>
                {#if store.commentBodyError}
                  <p class="mt-1 text-xs text-red-600 dark:text-red-400">{store.commentBodyError}</p>
                {/if}
              </div>
              <div class="flex justify-end">
                <Button type="submit" variant="primary" loading={store.postingComment} size="md">
                  {#snippet icon()}
                    {#if !store.postingComment}
                      <Send class="h-3.5 w-3.5" />
                    {/if}
                  {/snippet}
                  Post Comment
                </Button>
              </div>
            </form>
          {/if}
        </div>
      </div>
    {:else}
      <!-- No comments allowed - just spacer -->
      <div class="flex-1"></div>
    {/if}
  </aside>
{/if}
