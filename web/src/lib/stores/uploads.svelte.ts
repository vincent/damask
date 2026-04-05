import type { Asset } from '$lib/api'

export interface UploadItem {
  id: string
  file: File
  progress: number
  status: 'uploading' | 'processing' | 'done' | 'error'
  asset?: Asset
  error?: string
}

let items = $state<UploadItem[]>([])
// IDs of assets completed in this session — used for the required-fields nudge
let recentlyUploadedIds = $state<Set<string>>(new Set())

export const uploadsStore = {
  get items() {
    return items
  },

  get recentlyUploadedIds() {
    return recentlyUploadedIds
  },

  add(item: UploadItem) {
    items = [...items, item]
  },

  update(id: string, patch: Partial<UploadItem>) {
    items = items.map((item) => (item.id === id ? { ...item, ...patch } : item))
    // Track newly completed uploads by their asset ID
    if (patch.status === 'done' && patch.asset) {
      recentlyUploadedIds = new Set([...recentlyUploadedIds, patch.asset.id])
    }
  },

  remove(id: string) {
    items = items.filter((item) => item.id !== id)
  },

  clear() {
    items = items.filter((item) => item.status === 'uploading' || item.status === 'processing')
  },

  clearRecentlyUploaded() {
    recentlyUploadedIds = new Set()
  },
}
