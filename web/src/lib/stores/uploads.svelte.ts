import type { Asset } from '$lib/api/client'

export interface UploadItem {
  id: string
  file: File
  progress: number
  status: 'uploading' | 'processing' | 'done' | 'error'
  asset?: Asset
  error?: string
}

let items = $state<UploadItem[]>([])

export const uploadsStore = {
  get items() {
    return items
  },

  add(item: UploadItem) {
    items = [...items, item]
  },

  update(id: string, patch: Partial<UploadItem>) {
    items = items.map((item) => (item.id === id ? { ...item, ...patch } : item))
  },

  remove(id: string) {
    items = items.filter((item) => item.id !== id)
  },

  clear() {
    items = items.filter((item) => item.status === 'uploading' || item.status === 'processing')
  },
}
