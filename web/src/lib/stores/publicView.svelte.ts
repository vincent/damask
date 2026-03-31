import type { PublicAsset, PublicShare, ShareComment } from '$lib/api/client'

const API_BASE = import.meta.env.VITE_API_URL ?? ''

export const TYPES_BACKGROUNDS: Record<string, string> = {
  image: 'bg-amber-300 dark:bg-amber-700',
  video: 'bg-rose-300 dark:bg-rose-700',
  audio: 'bg-emerald-300 dark:bg-emerald-700',
  document: 'bg-blue-200 dark:bg-blue-700',
}

function createPublicViewStore() {
  // ---- Session ----
  let sessionToken = $state<string | null>(null)

  // ---- Gallery ----
  let share = $state<PublicShare | null>(null)
  let assets = $state<PublicAsset[]>([])
  let loadingGallery = $state(false)
  let galleryError = $state('')

  // ---- Review panel ----
  let selectedAsset = $state<PublicAsset | null>(null)
  let panelOpen = $state(false)
  let comments = $state<ShareComment[]>([])
  let loadingComments = $state(false)

  // ---- Comment form ----
  let commentName = $state('')
  let commentEmail = $state('')
  let commentBody = $state('')
  let commentNameError = $state('')
  let commentBodyError = $state('')
  let postingComment = $state(false)
  let commentPosted = $state(false)

  // ---- Derived ----
  const expiryWarning = $derived.by(() => {
    if (!share?.expires_at) return null
    const diff = new Date(share.expires_at).getTime() - Date.now()
    const days = Math.ceil(diff / (1000 * 60 * 60 * 24))
    if (days < 0) return 'Expired'
    if (days <= 7) return `Expires in ${days} day${days === 1 ? '' : 's'}`
    return null
  })

  // ---- Helpers ----
  function authHeaders(): Record<string, string> {
    return sessionToken ? { 'X-Share-Token': sessionToken } : {}
  }

  function thumbUrl(shareId: string, assetId: string) {
    return `${API_BASE}/shared/${shareId}/assets/${assetId}/thumb`
  }

  function fileUrl(shareId: string, assetId: string) {
    return `${API_BASE}/shared/${shareId}/assets/${assetId}/file`
  }

  function fileUrlWithToken(shareId: string, assetId: string) {
    const base = fileUrl(shareId, assetId)
    return sessionToken ? `${base}?token=${encodeURIComponent(sessionToken)}` : base
  }

  // ---- Actions ----
  async function init(shareId: string) {
    sessionToken = (await cookieStore.get(`share_token_${shareId}`))?.value ?? null
  }

  async function loadGallery(shareId: string, onNeedsPassword: () => void) {
    loadingGallery = true
    galleryError = ''
    try {
      const res = await fetch(`${API_BASE}/shared/${shareId}/assets`, {
        headers: authHeaders(),
      })
      if (res.status === 401 || res.status === 403) {
        onNeedsPassword()
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

  async function loadComments(shareId: string, assetId: string) {
    loadingComments = true
    try {
      const res = await fetch(`${API_BASE}/shared/${shareId}/comments?asset_id=${assetId}`, {
        headers: authHeaders(),
      })
      comments = res.ok
        ? await res.json().then((data: any[]) => data.find((d) => d.asset_id === assetId)?.comments ?? [])
        : []
    } catch {
      comments = []
    } finally {
      loadingComments = false
    }
  }

  function openAsset(shareId: string, asset: PublicAsset) {
    selectedAsset = asset
    panelOpen = true
    commentPosted = false
    comments = []
    if (share?.allow_comments) {
      loadComments(shareId, asset.id)
    }
  }

  function closePanel() {
    panelOpen = false
    selectedAsset = null
  }

  async function postComment(shareId: string) {
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

  async function downloadAll(shareId: string) {
    for (const asset of assets) {
      const a = document.createElement('a')
      a.href = fileUrlWithToken(shareId, asset.id)
      a.download = asset.original_filename
      a.target = '_blank'
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      await new Promise((r) => setTimeout(r, 200))
    }
  }

  return {
    // State (getters)
    get sessionToken() { return sessionToken },
    get share() { return share },
    get assets() { return assets },
    get loadingGallery() { return loadingGallery },
    get galleryError() { return galleryError },
    get selectedAsset() { return selectedAsset },
    get panelOpen() { return panelOpen },
    get comments() { return comments },
    get loadingComments() { return loadingComments },
    get commentName() { return commentName },
    set commentName(v: string) { commentName = v },
    get commentEmail() { return commentEmail },
    set commentEmail(v: string) { commentEmail = v },
    get commentBody() { return commentBody },
    set commentBody(v: string) { commentBody = v },
    get commentNameError() { return commentNameError },
    get commentBodyError() { return commentBodyError },
    get postingComment() { return postingComment },
    get commentPosted() { return commentPosted },
    // Derived
    get expiryWarning() { return expiryWarning },
    // Helpers
    thumbUrl,
    fileUrl,
    fileUrlWithToken,
    // Actions
    init,
    loadGallery,
    openAsset,
    closePanel,
    postComment,
    downloadAll,
  }
}

export const publicViewStore = createPublicViewStore()
