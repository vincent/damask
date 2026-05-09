import { ApiError, apiFetch } from './client'
import type {
  Asset,
  FieldFilter,
  ImageRouterModelsResponse,
  ShareComment,
} from './models'

const API_BASE = import.meta.env.VITE_API_URL ?? ''

export interface AssetListResponse {
  assets: Asset[]
  next_cursor: string | null
}

let imageRouterModelsCache: ImageRouterModelsResponse | null = null
let imageRouterModelsPromise: Promise<ImageRouterModelsResponse> | null = null

export function invalidateImageRouterModelsCache() {
  imageRouterModelsCache = null
  imageRouterModelsPromise = null
}

async function fetchImageRouterModelsCached(): Promise<ImageRouterModelsResponse> {
  if (imageRouterModelsCache) return imageRouterModelsCache
  if (imageRouterModelsPromise) return imageRouterModelsPromise

  imageRouterModelsPromise = apiFetch<ImageRouterModelsResponse>(
    '/api/v1/imagerouter/models'
  )

  try {
    const response = await imageRouterModelsPromise
    imageRouterModelsCache = response
    return response
  } catch (error) {
    imageRouterModelsPromise = null
    throw error
  } finally {
    if (imageRouterModelsCache) {
      imageRouterModelsPromise = null
    }
  }
}

export const assetApi = {
  /** POST /api/v1/assets (editor+) — upload a new asset via XHR (with progress callback). */
  upload(
    file: File,
    projectId: string | null,
    folderId: string | null,
    onProgress?: (pct: number) => void
  ): Promise<Asset> {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest()
      xhr.open('POST', `${API_BASE}/api/v1/assets`)
      xhr.withCredentials = true

      if (onProgress) {
        xhr.upload.addEventListener('progress', (e) => {
          if (e.lengthComputable) {
            onProgress(Math.round((e.loaded / e.total) * 100))
          }
        })
      }

      xhr.addEventListener('load', () => {
        if (xhr.status === 201) {
          resolve(JSON.parse(xhr.responseText) as Asset)
        } else if (xhr.status === 401 && typeof window !== 'undefined') {
          window.location.href = '/login'
          reject(new ApiError(401, 'Unauthorized'))
        } else {
          const body = JSON.parse(xhr.responseText) as { error?: string }
          reject(new ApiError(xhr.status, body.error ?? xhr.statusText))
        }
      })

      xhr.addEventListener('error', () =>
        reject(new ApiError(0, 'Network error'))
      )

      const fd = new FormData()
      fd.append('file', file)
      if (projectId) fd.append('project_id', projectId)
      if (folderId) fd.append('folder_id', folderId)
      xhr.send(fd)
    })
  },

  /** GET /api/v1/assets — list assets with pagination, search, filtering, sorting. */
  list(
    params: {
      cursor?: string
      limit?: number
      sortKey?: string
      sortAsc?: boolean
      q?: string
      project_id?: string
      mime?: string
      tags?: string[]
      folder_id?: string
      collection_id?: string
      fieldFilters?: FieldFilter[]
    } = {}
  ): Promise<AssetListResponse> {
    const qs = new URLSearchParams()
    if (params.sortKey)
      qs.set('sort', `${params.sortKey}_${params.sortAsc ? 'asc' : 'desc'}`)
    if (params.cursor) qs.set('cursor', params.cursor)
    if (params.limit) qs.set('limit', String(params.limit))
    if (params.q) qs.set('q', params.q)
    if (params.project_id) qs.set('project_id', params.project_id)
    if (params.mime) qs.set('mime', params.mime)
    if (params.tags && params.tags.length > 0)
      qs.set('tags', params.tags.join(','))
    if (params.folder_id) qs.set('folder_id', params.folder_id)
    if (params.collection_id) qs.set('collection_id', params.collection_id)
    if (params.fieldFilters) {
      for (const f of params.fieldFilters) {
        const paramKey =
          f.op === 'eq' ? `field[${f.key}]` : `field[${f.key}][${f.op}]`
        qs.set(paramKey, f.value)
      }
    }
    const query = qs.toString()
    return apiFetch<AssetListResponse>(
      `/api/v1/assets${query ? '?' + query : ''}`
    )
  },

  /** GET /api/v1/assets/:id — fetch asset metadata by ID. */
  get(id: string): Promise<Asset> {
    return apiFetch<Asset>(`/api/v1/assets/${id}`)
  },

  /** PATCH /api/v1/assets/:id (editor+) — move asset to a folder (pass null to unassign). */
  updateFolder: (assetId: string, folderId: string | null) =>
    apiFetch<Asset>(`/api/v1/assets/${assetId}`, {
      method: 'PATCH',
      body: JSON.stringify({ folder_id: folderId }),
    }),

  /** PUT /api/v1/assets/:id/rename (editor+) — rename an asset. */
  rename: (assetId: string, name: string) =>
    apiFetch<Asset>(`/api/v1/assets/${assetId}/rename`, {
      method: 'PUT',
      body: JSON.stringify({ name }),
    }),

  /** DELETE /api/v1/assets/:id (editor+) — delete an asset. */
  delete(id: string): Promise<void> {
    return apiFetch<void>(`/api/v1/assets/${id}`, { method: 'DELETE' })
  },

  /** POST /api/v1/assets/bulk/project (editor+) — assign multiple assets to a project. */
  bulkProject: (assetIds: string[], projectId: string | null) =>
    apiFetch<void>('/api/v1/assets/bulk/project', {
      method: 'POST',
      body: JSON.stringify({ asset_ids: assetIds, project_id: projectId }),
    }),

  /** DELETE /api/v1/assets/bulk (owner only) — delete multiple assets. */
  bulkDelete: (assetIds: string[]) =>
    apiFetch<void>('/api/v1/assets/bulk', {
      method: 'DELETE',
      body: JSON.stringify({ asset_ids: assetIds }),
    }),

  /** GET /api/v1/assets/:id/thumb — thumbnail URL for an asset.
   * Pass thumbnailKey to append a cache-busting version token so the browser
   * refetches when the thumbnail is regenerated (24h max-age otherwise wins). */
  thumbUrl(id: string, thumbnailKey?: string | null): string {
    const base = `${API_BASE}/api/v1/assets/${id}/thumb`
    if (!thumbnailKey) return base
    // Use the last path segment of the storage key as an opaque version token.
    const v = thumbnailKey.split('/').pop() ?? thumbnailKey
    return `${base}?v=${encodeURIComponent(v)}`
  },

  /** GET /api/v1/assets/:id/file — original file URL for an asset. */
  fileUrl(id: string): string {
    return `${API_BASE}/api/v1/assets/${id}/file`
  },

  /** GET /api/v1/assets/:id/comments — list comments for an asset. */
  listAssetComments: (id: string) =>
    apiFetch<ShareComment[]>(`/api/v1/assets/${id}/comments`),

  /** POST /api/v1/assets/:id/thumb/regenerate (editor+) — requeue the thumbnail generation job. */
  regenerateThumbnail: (id: string) =>
    apiFetch<{ job_id: string; status: string; message: string }>(
      `/api/v1/assets/${id}/thumb/regenerate`,
      {
        method: 'POST',
      }
    ),

  /** GET /api/v1/imagerouter/models — list image-to-image models via the authenticated backend proxy. */
  fetchImageRouterModels: () => fetchImageRouterModelsCached(),
}
