import { assetApi, mimeCategory, openThumbnailEvents, type Asset } from '$lib/api'
import type { FieldFilter } from '$lib/api/models'
import { navigationStore } from './navigation.svelte'
import { uploadsStore } from './uploads.svelte'

let assets = $state<Asset[]>([])
let nextCursor = $state<string | null>(null)
let loading = $state(false)
let initialLoad = $state(true)
let query = $state('')
let activeTags = $state<string[]>([])

let sortKey = $state('date_created')
let sortAsc = $state(false)
let fieldFilters = $state<FieldFilter[]>([])
let generation = 0
let resetDone = $state(0)

const assetsByCategory = $derived({
  image: assets.filter((a) => mimeCategory(a.mime_type) === 'image'),
  video: assets.filter((a) => mimeCategory(a.mime_type) === 'video'),
  audio: assets.filter((a) => mimeCategory(a.mime_type) === 'audio'),
  document: assets.filter((a) => mimeCategory(a.mime_type) === 'document'),
})

let searchTimer: ReturnType<typeof setTimeout>

// ---- SSE state ----
let sseSource: EventSource | null = null
// Maps assetId -> uploadId for in-flight thumbnail waits
const pendingThumbnails = new Map<string, string>()

function patchAsset(assetId: string, patch: Partial<Asset>) {
  assets = assets.map((a) => (a.id === assetId ? { ...a, ...patch } : a))
}

function ensureSSE() {
  if (sseSource && sseSource.readyState !== EventSource.CLOSED) return
  sseSource = openThumbnailEvents()

  sseSource.addEventListener('message', (e: MessageEvent) => {
    try {
      const event = JSON.parse(e.data) as { type: string; asset_id: string; thumbnail_key: string }
      if (event.type !== 'thumbnail_ready') return

      const uploadId = pendingThumbnails.get(event.asset_id)
      if (!uploadId) return

      pendingThumbnails.delete(event.asset_id)
      patchAsset(event.asset_id, { thumbnail_key: event.thumbnail_key })
      uploadsStore.update(uploadId, { status: 'done' })

      if (pendingThumbnails.size === 0) {
        sseSource?.close()
        sseSource = null
      }
    } catch {
      // ignore malformed events
    }
  })

  sseSource.onerror = () => {
    // SSE connection failed; fall back to polling for all pending assets
    const pending = new Map(pendingThumbnails)
    pendingThumbnails.clear()
    sseSource?.close()
    sseSource = null
    for (const [assetId, uploadId] of pending) {
      pollThumbnail(assetId, uploadId)
    }
  }
}

async function pollThumbnail(assetId: string, uploadId: string, retries = 5) {
  for (let i = 0; i < retries; i++) {
    await new Promise((r) => setTimeout(r, 3000))
    try {
      const asset = await assetApi.get(assetId)
      if (asset.thumbnail_key) {
        patchAsset(assetId, { thumbnail_key: asset.thumbnail_key })
        uploadsStore.update(uploadId, { status: 'done' })
        return
      }
    } catch {
      // continue retrying
    }
  }
  // Exhausted retries — mark done so the spinner doesn't stay forever
  uploadsStore.update(uploadId, { status: 'done' })
}

export const assetsStore = {
  get assets() { return assets },
  get nextCursor() { return nextCursor },
  get loading() { return loading },
  get initialLoad() { return initialLoad },
  get query() { return query },
  set query(v: string) { query = v },
  get activeTags() { return activeTags },
  get fieldFilters() { return fieldFilters },
  get assetsByCategory() { return assetsByCategory },
  get resetDone() { return resetDone },

  async load(reset = false) {
    if (!reset && loading) return
    if (reset) {
      generation++
      nextCursor = null
    }
    const myGen = generation
    loading = true
    try {
      const result = await assetApi.list({
        cursor: nextCursor ?? undefined,
        sortKey: sortKey || undefined,
        sortAsc: !!sortAsc,
        q: query || undefined,
        project_id: navigationStore.activeProjectId ?? undefined,
        tags: activeTags.length > 0 ? activeTags : undefined,
        folder_id: navigationStore.activeFolderId ?? undefined,
        fieldFilters: fieldFilters.length > 0 ? fieldFilters : undefined,
        limit: 20,
      })
      if (myGen !== generation) return
      assets = reset ? result.assets : [...assets, ...result.assets]
      nextCursor = result.next_cursor
      if (reset) resetDone++
    } catch {
      // 401 redirect handled by api client
    } finally {
      if (myGen === generation) loading = false
      initialLoad = false
    }
  },

  sort(key: string, asc: boolean) {
    sortKey = key
    sortAsc = asc
    this.load(true)
  },

  search() {
    clearTimeout(searchTimer)
    nextCursor = null
    searchTimer = setTimeout(() => {
      assetsStore.load(true)
    }, 300)
  },

  setActiveTags(tags: string[]) {
    activeTags = tags
    nextCursor = null
    assetsStore.load(true)
  },

  setFieldFilters(filters: FieldFilter[]) {
    fieldFilters = filters
    nextCursor = null
    assetsStore.load(true)
  },

  prepend(asset: Asset) {
    assets = [asset, ...assets]
  },

  remove(id: string) {
    assets = assets.filter((a) => a.id !== id)
  },

  upload(files: File[]) {
    for (const file of files) {
      const id = crypto.randomUUID()
      uploadsStore.add({ id, file, progress: 0, status: 'uploading' })
      assetApi
        .upload(file, (pct) => uploadsStore.update(id, { progress: pct }))
        .then((asset) => {
          assetsStore.prepend(asset)
          uploadsStore.update(id, { status: 'processing', asset })

          if (asset.thumbnail_key) {
            // Thumbnail already ready (non-image or instant processing)
            uploadsStore.update(id, { status: 'done' })
            return
          }

          // Register for SSE notification
          pendingThumbnails.set(asset.id, id)
          ensureSSE()

          // 30s timeout: fall back to polling if SSE hasn't delivered
          setTimeout(() => {
            if (pendingThumbnails.has(asset.id)) {
              pendingThumbnails.delete(asset.id)
              if (pendingThumbnails.size === 0) {
                sseSource?.close()
                sseSource = null
              }
              pollThumbnail(asset.id, id)
            }
          }, 30_000)
        })
        .catch((err: Error) => uploadsStore.update(id, { status: 'error', error: err.message }))
    }
  },
}
