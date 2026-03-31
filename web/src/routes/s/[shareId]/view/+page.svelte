<script lang="ts">
  import { onMount } from 'svelte'
  import { goto } from '$app/navigation'
  import { page } from '$app/state'
  import { Download, X, MessageSquare, Send, File, Play, Music, Image } from '@lucide/svelte'
  import { formatBytes, mimeCategory, type PublicAsset, type PublicShare, type ShareComment } from '$lib/api/client'
  import Badge from '$lib/components/ui/Badge.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import SharedAsset from '$lib/components/SharedAsset.svelte'

  const API_BASE = import.meta.env.VITE_API_URL ?? ''

  // ---- State ----

  let shareId = $derived(page.params.shareId)
  let sessionToken = $state<string | null>(null)

  let share = $state<PublicShare | null>(null)
  let assets = $state<PublicAsset[]>([])
  let loadingGallery = $state(true)
  let galleryError = $state('')

  // Review panel
  let selectedAsset = $state<PublicAsset | null>(null)
  let panelOpen = $state(false)
  let comments = $state<ShareComment[]>([])
  let loadingComments = $state(false)

  // Comment form
  let commentName = $state('')
  let commentEmail = $state('')
  let commentBody = $state('')
  let commentNameError = $state('')
  let commentBodyError = $state('')
  let postingComment = $state(false)
  let commentPosted = $state(false)

  // ---- Derived ----

  let expiryWarning = $derived(() => {
    if (!share?.expires_at) return null
    const diff = new Date(share.expires_at).getTime() - Date.now()
    const days = Math.ceil(diff / (1000 * 60 * 60 * 24))
    if (days < 0) return 'Expired'
    if (days <= 7) return `Expires in ${days} day${days === 1 ? '' : 's'}`
    return null
  })

  // ---- Helpers ----

  function thumbUrl(assetId: string) {
    return `${API_BASE}/shared/${shareId}/assets/${assetId}/thumb`
  }

  function fileUrl(assetId: string) {
    return `${API_BASE}/shared/${shareId}/assets/${assetId}/file`
  }

  function authHeaders(): Record<string, string> {
    const h: Record<string, string> = {}
    if (sessionToken) h['X-Share-Token'] = sessionToken
    return h
  }

  function formatDate(iso: string) {
    return new Date(iso).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
  }

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

  const cardBg: Record<string, string> = {
    image: 'bg-amber-300 dark:bg-amber-700',
    video: 'bg-rose-300 dark:bg-rose-700',
    audio: 'bg-emerald-300 dark:bg-emerald-700',
    document: 'bg-blue-200 dark:bg-blue-700',
  }

  const avatarColors = [
    'bg-violet-200 text-violet-700 dark:bg-violet-800 dark:text-violet-200',
    'bg-blue-200 text-blue-700 dark:bg-blue-800 dark:text-blue-200',
    'bg-emerald-200 text-emerald-700 dark:bg-emerald-800 dark:text-emerald-200',
    'bg-amber-200 text-amber-700 dark:bg-amber-800 dark:text-amber-200',
  ]

  function avatarColor(name: string) {
    let hash = 0
    for (let i = 0; i < name.length; i++) hash = (hash * 31 + name.charCodeAt(i)) & 0xffffff
    return avatarColors[hash % avatarColors.length]
  }

  // ---- Data loading ----

  async function loadGallery() {
    loadingGallery = true
    galleryError = ''
    try {
      const res = await fetch(`${API_BASE}/shared/${shareId}/assets`, {
        headers: authHeaders(),
      })
      if (res.status === 401 || res.status === 403) {
        // Needs password - redirect to gate
        goto(`/s/${shareId}`, { replaceState: true })
        return
      }
      if (!res.ok) {
        galleryError = 'This share link is invalid or has expired.'
        return
      }
      const data = await res.json()
      share = data.share
      assets = data.assets ?? []
    } catch {
      galleryError = 'Failed to load gallery. Please check your connection.'
    } finally {
      loadingGallery = false
    }
  }

  async function loadComments(assetId: string) {
    loadingComments = true
    try {
      const res = await fetch(`${API_BASE}/shared/${shareId}/comments?asset_id=${assetId}`, {
        headers: authHeaders(),
      })
      if (res.ok) {
        comments = await res.json().then((data) => data.find((d: any) => d.asset_id === assetId)?.comments ?? [])
      } else {
        comments = []
      }
    } catch {
      comments = []
    } finally {
      loadingComments = false
    }
  }

  async function handlePostComment() {
    commentNameError = ''
    commentBodyError = ''
    if (!commentName.trim()) { commentNameError = 'Name is required'; return }
    if (!commentBody.trim()) { commentBodyError = 'Message is required'; return }

    postingComment = true
    try {
      const res = await fetch(`${API_BASE}/shared/${shareId}/comments`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...authHeaders() },
        body: JSON.stringify({
          asset_id: selectedAsset?.id,
          author_name: commentName.trim(),
          author_email: commentEmail.trim() || undefined,
          body: commentBody.trim(),
        }),
      })
      if (res.ok) {
        const newComment = await res.json()
        comments = [...comments, newComment]
        commentName = ''
        commentEmail = ''
        commentBody = ''
        commentPosted = true
        setTimeout(() => (commentPosted = false), 3000)
      }
    } catch {
      // silent
    } finally {
      postingComment = false
    }
  }

  function openAsset(asset: PublicAsset) {
    selectedAsset = asset
    panelOpen = true
    commentPosted = false
    comments = []
    if (share?.allow_comments) {
      loadComments(asset.id)
    }
  }

  function closePanel() {
    panelOpen = false
    selectedAsset = null
  }

  async function downloadAll() {
    // Open each asset file URL in a new tab is not ideal;
    // the backend should provide a zip endpoint. For now, download individually.
    for (const asset of assets) {
      const a = document.createElement('a')
      a.href = `${fileUrl(asset.id)}${sessionToken ? `?token=${encodeURIComponent(sessionToken)}` : ''}`
      a.download = asset.original_filename
      a.target = '_blank'
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      await new Promise((r) => setTimeout(r, 200))
    }
  }

  onMount(async () => {
    sessionToken = (await cookieStore.get(`share_token_${shareId}`))?.value || null
    loadGallery()
  })
</script>

<svelte:head>
  <title>{share?.label ?? 'Shared Gallery'} — Damask</title>
</svelte:head>

<div class="flex min-h-screen flex-col bg-gray-50 dark:bg-gray-950">

  <!-- Header -->
  <header class="border-b border-gray-200 bg-white px-6 py-4 dark:border-gray-800 dark:bg-gray-900">
    <div class="mx-auto flex max-w-6xl items-center justify-between">
      <div>
        {#if share}
          <h1 class="text-xl font-bold text-gray-900 dark:text-gray-100">{share.label}</h1>
          <div class="mt-0.5 flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
            <span>{assets.length} asset{assets.length === 1 ? '' : 's'}</span>
            {#if expiryWarning()}
              <span class="text-gray-300 dark:text-gray-600">•</span>
              <span class="flex items-center gap-1 text-amber-500 dark:text-amber-400">
                <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <circle cx="12" cy="12" r="10" /><path d="M12 6v6l4 2" stroke-linecap="round" />
                </svg>
                {expiryWarning()}
              </span>
            {/if}
          </div>
        {:else if loadingGallery}
          <div class="h-5 w-48 animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
          <div class="mt-1 h-4 w-32 animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
        {/if}
      </div>

      {#if share?.allow_download && assets.length > 0}
        <Button variant="secondary" onclick={downloadAll} size="md">
          {#snippet icon()}
            <Download class="h-4 w-4" />
          {/snippet}
          Download All
        </Button>
      {/if}
    </div>
  </header>

  <!-- Body -->
  <main class="mx-auto w-full max-w-6xl flex-1 px-6 py-8">
    {#if loadingGallery}
      <div class="flex items-center justify-center py-24">
        <Spinner size="lg" />
      </div>
    {:else if galleryError}
      <div class="flex flex-col items-center justify-center py-24 text-center">
        <p class="text-gray-500 dark:text-gray-400">{galleryError}</p>
      </div>
    {:else if assets.length === 0}
      <div class="flex flex-col items-center justify-center py-24 text-center">
        <p class="text-gray-500 dark:text-gray-400">No assets in this share.</p>
      </div>
    {:else}
      <div class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
        {#each assets as asset}
          {@const category = mimeCategory(asset.mime_type)}
          <button
            type="button"
            class="group flex w-full flex-col overflow-hidden rounded-xl bg-white text-left shadow-sm transition-shadow hover:shadow-md focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-400 dark:bg-gray-800"
            onclick={() => openAsset(asset)}
          >
            <!-- Thumbnail -->
            <div
              class="relative w-full overflow-hidden rounded-t-xl {cardBg[category]}"
              style="aspect-ratio: 4/3"
            >
              <!-- Category badge -->
              <div class="absolute right-2 top-2 z-10">
                <Badge variant={category} size="sm">
                  {category.toUpperCase()}
                </Badge>
              </div>

              {#if category === 'image'}
                <img
                  src={thumbUrl(asset.id)}
                  alt={asset.original_filename}
                  class="h-full w-full object-cover"
                  loading="lazy"
                  onerror={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none' }}
                />
              {:else}
                <div class="flex h-full items-center justify-center">
                  <div class="flex h-12 w-12 items-center justify-center rounded-xl bg-white/25">
                    {#if category === 'video'}
                      <Play class="h-7 w-7 text-white" />
                    {:else if category === 'audio'}
                      <Music class="h-7 w-7 text-white" />
                    {:else}
                      <File class="h-7 w-7 text-white" />
                    {/if}
                  </div>
                </div>
              {/if}
            </div>

            <!-- Info -->
            <div class="flex flex-col gap-1 px-3 py-2.5">
              <p class="truncate text-sm font-medium text-gray-900 dark:text-gray-100" title={asset.original_filename}>
                {asset.original_filename}
              </p>
              <div class="flex items-center justify-between">
                <span class="text-xs text-gray-400">{formatBytes(asset.size)}</span>
                <span class="text-xs text-gray-400">{formatDate(asset.created_at)}</span>
              </div>
            </div>
          </button>
        {/each}
      </div>
    {/if}
  </main>

  <!-- Footer -->
  <footer class="py-6">
    <div class="flex items-center justify-center gap-1.5 text-xs text-gray-400 dark:text-gray-600">
      <span class="uppercase tracking-widest">Powered by</span>
      <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="currentColor">
        <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5" />
      </svg>
      <span class="font-semibold uppercase tracking-widest">Damask</span>
    </div>
  </footer>
</div>

<!-- Review Panel (S12) -->
{#if panelOpen && selectedAsset}
  {@const category = mimeCategory(selectedAsset.mime_type)}
  <!-- Backdrop -->
  <div
    class="fixed hidden md:grid md:w-[75%] inset-0 z-40 bg-black/40 backdrop-blur-sm grid place-items-center p-40"
    role="button"
    tabindex="-1"
    onclick={closePanel}
    onkeydown={(e) => e.key === 'Escape' && closePanel()}
  >
    <SharedAsset
      {category} asset={selectedAsset}
      thumbUrl={thumbUrl(selectedAsset.id)}
      assetUrl="{fileUrl(selectedAsset.id)}{sessionToken ? `?token=${encodeURIComponent(sessionToken)}` : ''}"
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
      <div class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg {cardBg[category]}">
        {#if category === 'image'}
          <Image class="h-5 w-5 text-white" />
        {:else if category === 'video'}
          <Play class="h-5 w-5 text-white" />
        {:else if category === 'audio'}
          <Music class="h-5 w-5 text-white" />
        {:else}
          <File class="h-5 w-5 text-white" />
        {/if}
      </div>
      <div class="min-w-0 flex-1">
        <p class="truncate text-sm font-semibold text-gray-900 dark:text-gray-100" title={selectedAsset.original_filename}>
          {selectedAsset.original_filename}
        </p>
        <p class="text-xs text-gray-500 dark:text-gray-400">
          {formatBytes(selectedAsset.size)} • {category.toUpperCase()}
        </p>
      </div>
      <div class="flex items-center gap-1">
        {#if share?.allow_download}
          <a
            href="{fileUrl(selectedAsset.id)}{sessionToken ? `?token=${encodeURIComponent(sessionToken)}` : ''}"
            download={selectedAsset.original_filename}
            class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-gray-200"
            aria-label="Download"
          >
            <Download class="h-4 w-4" />
          </a>
        {/if}
        <button
          type="button"
          class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-gray-200"
          onclick={closePanel}
          aria-label="Close panel"
        >
          <X class="h-4 w-4" />
        </button>
      </div>
    </div>

    <!-- Mobile Preview area -->
    <div class="block md:hidden flex-shrink-0 {cardBg[category]}" style="height: 220px">
      <div class="flex h-full items-center justify-center">
        <SharedAsset
          {category} asset={selectedAsset}
          thumbUrl={thumbUrl(selectedAsset.id)}
          assetUrl="{fileUrl(selectedAsset.id)}{sessionToken ? `?token=${encodeURIComponent(sessionToken)}` : ''}"
        />
      </div>
    </div>

    <!-- Comments section -->
    {#if share?.allow_comments}
      <div class="flex flex-1 flex-col overflow-hidden">
        <!-- Comments header -->
        <div class="flex items-center gap-2 px-4 py-3">
          <MessageSquare class="h-4 w-4 text-gray-500 dark:text-gray-400" />
          <span class="text-sm font-semibold text-gray-900 dark:text-gray-100">Comments</span>
          {#if comments.length > 0}
            <span class="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-gray-800 dark:text-gray-300">
              {comments.length}
            </span>
          {/if}
        </div>

        <!-- Comment list -->
        <div class="flex-1 overflow-y-auto px-4">
          {#if loadingComments}
            <div class="flex justify-center py-6">
              <Spinner size="sm" />
            </div>
          {:else if comments.length === 0}
            <p class="py-4 text-center text-sm text-gray-400 dark:text-gray-600">No comments yet. Be the first!</p>
          {:else}
            <div class="flex flex-col gap-4 pb-4">
              {#each comments as comment}
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

          {#if commentPosted}
            <div class="rounded-lg bg-emerald-50 px-3 py-2 text-sm text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300">
              Comment posted! Thank you.
            </div>
          {:else}
            <form onsubmit={(e) => { e.preventDefault(); handlePostComment() }}>
              <div class="mb-2 grid grid-cols-2 gap-2">
                <Input
                  placeholder="Your name"
                  label="Name *"
                  bind:value={commentName}
                  error={commentNameError}
                  autocomplete="name"
                />
                <Input
                  placeholder="For notifications"
                  label="Email (optional)"
                  type="email"
                  bind:value={commentEmail}
                  autocomplete="email"
                />
              </div>
              <div class="mb-3">
                <label for="comment-body" class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
                  Message *
                </label>
                <textarea
                  bind:value={commentBody}
                  placeholder="Add your feedback…"
                  id="comment-body"
                  rows="3"
                  class="w-full resize-none rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 placeholder-gray-400 shadow-sm transition-colors focus:border-indigo-400 focus:outline-none focus:ring-2 focus:ring-indigo-200 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500 dark:focus:border-indigo-500 dark:focus:ring-indigo-900 {commentBodyError ? 'border-red-400 focus:ring-red-200 dark:border-red-500' : ''}"
                ></textarea>
                {#if commentBodyError}
                  <p class="mt-1 text-xs text-red-600 dark:text-red-400">{commentBodyError}</p>
                {/if}
              </div>
              <div class="flex justify-end">
                <Button type="submit" variant="primary" loading={postingComment} size="md">
                  {#snippet icon()}
                    {#if !postingComment}
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
