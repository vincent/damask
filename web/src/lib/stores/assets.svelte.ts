import { assetApi, mimeCategory, type Asset } from '$lib/api'
import { navigationStore } from './navigation.svelte'
import { uploadsStore } from './uploads.svelte'

let assets = $state<Asset[]>([])
let nextCursor = $state<string | null>(null)
let loading = $state(false)
let initialLoad = $state(true)
let query = $state('')
let activeTags = $state<string[]>([])

const assetsByCategory = $derived({
  image: assets.filter((a) => mimeCategory(a.mime_type) === 'image'),
  video: assets.filter((a) => mimeCategory(a.mime_type) === 'video'),
  audio: assets.filter((a) => mimeCategory(a.mime_type) === 'audio'),
  document: assets.filter((a) => mimeCategory(a.mime_type) === 'document'),
})

let searchTimer: ReturnType<typeof setTimeout>

export const assetsStore = {
  get assets() { return assets },
  get nextCursor() { return nextCursor },
  get loading() { return loading },
  get initialLoad() { return initialLoad },
  get query() { return query },
  set query(v: string) { query = v },
  get activeTags() { return activeTags },
  get assetsByCategory() { return assetsByCategory },

  async load(reset = false) {
    if (loading) return
    loading = true
    try {
      const result = await assetApi.list({
        cursor: reset ? undefined : (nextCursor ?? undefined),
        q: query || undefined,
        project_id: navigationStore.activeProjectId ?? undefined,
        tags: activeTags.length > 0 ? activeTags : undefined,
        folder_id: navigationStore.activeFolderId ?? undefined,
        limit: 48,
      })
      assets = reset ? result.assets : [...assets, ...result.assets]
      nextCursor = result.next_cursor
    } catch {
      // 401 redirect handled by api client
    } finally {
      loading = false
      initialLoad = false
    }
  },

  search() {
    clearTimeout(searchTimer)
    searchTimer = setTimeout(() => {
      nextCursor = null
      assetsStore.load(true)
    }, 300)
  },

  setActiveTags(tags: string[]) {
    activeTags = tags
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
          uploadsStore.update(id, { status: 'done', asset })
        })
        .catch((err: Error) => uploadsStore.update(id, { status: 'error', error: err.message }))
    }
  },
}
