import type { Asset, AuthResponse, CreateShareParams, CreateVariantResponse, Folder, Project, Share, Tag, UpdateShareParams, Variant, Workspace, WorkspaceMeResponse } from "./models"

const API_BASE = import.meta.env.VITE_API_URL ?? ''

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

async function handleResponse<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }))
    throw new ApiError(res.status, body.error ?? res.statusText)
  }
  return res.json() as Promise<T>
}

export async function apiFetch<T>(path: string, init: RequestInit = {}, fetch = window.fetch): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...init.headers },
    ...init,
  })

  if (res.status === 401 && typeof window !== 'undefined') {
    window.location.href = '/login'
    throw new ApiError(401, 'Unauthorized')
  }

  return handleResponse<T>(res)
}

export const authApi = {
  register: (name: string, email: string, password: string) =>
    apiFetch<AuthResponse>('/auth/register', {
      method: 'POST',
      body: JSON.stringify({ name, email, password }),
    }),

  login: (email: string, password: string) =>
    apiFetch<AuthResponse>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    }),

  refresh: () => apiFetch<{ token: string }>('/auth/refresh', { method: 'POST' }),
}

export interface WorkspaceWithRole extends Workspace {
  role: string
}

export interface SwitchWorkspaceResponse {
  token: string
  workspace: Workspace
  role: string
}

export const workspaceApi = {
  fetch: window.fetch,
  useFetch: (f: (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>) => workspaceApi.fetch = f,

  me: () => apiFetch<WorkspaceMeResponse>('/api/v1/workspace/me', undefined, workspaceApi.fetch),

  list: () => apiFetch<WorkspaceWithRole[]>('/api/v1/workspaces'),

  switch: (workspaceId: string) =>
    apiFetch<SwitchWorkspaceResponse>('/api/v1/workspace/switch', {
      method: 'POST',
      body: JSON.stringify({ workspace_id: workspaceId }),
    }),

  createWorkspace: (name: string) =>
    apiFetch<AuthResponse>('/api/v1/workspace', {
      method: 'POST',
      body: JSON.stringify({ name }),
    }),

  createInvite: (email: string, role: 'editor' | 'viewer' = 'editor') =>
    apiFetch<{ id: string; invite_token: string; email: string; role: string }>(
      '/api/v1/workspace/invites',
      { method: 'POST', body: JSON.stringify({ email, role }) }
    ),

  acceptInvite: (token: string, name: string, password: string) =>
    apiFetch<AuthResponse>('/auth/invite/accept', {
      method: 'POST',
      body: JSON.stringify({ token, name, password }),
    }),
}


export const projectApi = {
  list: () => apiFetch<Project[]>('/api/v1/projects'),

  get: (id: string) => apiFetch<Project>(`/api/v1/projects/${id}`),

  create: (data: { name: string; description?: string; color?: string }) =>
    apiFetch<Project>('/api/v1/projects', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  update: (id: string, data: { name?: string; description?: string; color?: string; cover_asset_id?: string }) =>
    apiFetch<Project>(`/api/v1/projects/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  delete: (id: string) =>
    apiFetch<void>(`/api/v1/projects/${id}`, { method: 'DELETE' }),
}


export const folderApi = {
  list: (projectId: string) => apiFetch<Folder[]>(`/api/v1/projects/${projectId}/folders`),

  create: (projectId: string, data: { name: string; parent_id?: string; position?: number }) =>
    apiFetch<Folder>(`/api/v1/projects/${projectId}/folders`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  update: (id: string, data: { name?: string; position?: number }) =>
    apiFetch<Folder>(`/api/v1/folders/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  delete: (id: string) =>
    apiFetch<void>(`/api/v1/folders/${id}`, { method: 'DELETE' }),
}



export const tagApi = {
  list: () => apiFetch<Tag[]>('/api/v1/tags'),

  getForAsset: (assetId: string) => apiFetch<string[]>(`/api/v1/assets/${assetId}/tags`),

  addToAsset: (assetId: string, name: string) =>
    apiFetch<{ name: string }>(`/api/v1/assets/${assetId}/tags`, {
      method: 'POST',
      body: JSON.stringify({ name }),
    }),

  removeFromAsset: (assetId: string, name: string) =>
    apiFetch<void>(`/api/v1/assets/${assetId}/tags/${encodeURIComponent(name)}`, {
      method: 'DELETE',
    }),

  bulkTag: (assetIds: string[], tagName: string) =>
    apiFetch<void>('/api/v1/assets/bulk/tag', {
      method: 'POST',
      body: JSON.stringify({ asset_ids: assetIds, tag_name: tagName }),
    }),

  bulkProject: (assetIds: string[], projectId: string | null) =>
    apiFetch<void>('/api/v1/assets/bulk/project', {
      method: 'POST',
      body: JSON.stringify({ asset_ids: assetIds, project_id: projectId }),
    }),

  bulkDelete: (assetIds: string[]) =>
    apiFetch<void>('/api/v1/assets/bulk', {
      method: 'DELETE',
      body: JSON.stringify({ asset_ids: assetIds }),
    }),
}

export interface AssetListResponse {
  assets: Asset[]
  next_cursor: string | null
}

export const assetApi = {
  upload(file: File, onProgress?: (pct: number) => void): Promise<Asset> {
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

      xhr.addEventListener('error', () => reject(new ApiError(0, 'Network error')))

      const fd = new FormData()
      fd.append('file', file)
      xhr.send(fd)
    })
  },

  list(params: {
    cursor?: string
    limit?: number
    q?: string
    project_id?: string
    mime?: string
    tags?: string[]
    folder_id?: string
  } = {}): Promise<AssetListResponse> {
    const qs = new URLSearchParams()
    if (params.cursor) qs.set('cursor', params.cursor)
    if (params.limit) qs.set('limit', String(params.limit))
    if (params.q) qs.set('q', params.q)
    if (params.project_id) qs.set('project_id', params.project_id)
    if (params.mime) qs.set('mime', params.mime)
    if (params.tags && params.tags.length > 0) qs.set('tags', params.tags.join(','))
    if (params.folder_id) qs.set('folder_id', params.folder_id)
    const query = qs.toString()
    return apiFetch<AssetListResponse>(`/api/v1/assets${query ? '?' + query : ''}`)
  },

  updateFolder: (assetId: string, folderId: string | null) =>
    apiFetch<Asset>(`/api/v1/assets/${assetId}`, {
      method: 'PATCH',
      body: JSON.stringify({ folder_id: folderId }),
    }),

  get(id: string): Promise<Asset> {
    return apiFetch<Asset>(`/api/v1/assets/${id}`)
  },

  delete(id: string): Promise<void> {
    return apiFetch<void>(`/api/v1/assets/${id}`, { method: 'DELETE' })
  },

  thumbUrl(id: string): string {
    return `${API_BASE}/api/v1/assets/${id}/thumb`
  },

  fileUrl(id: string): string {
    return `${API_BASE}/api/v1/assets/${id}/file`
  },
}



export const variantApi = {
  list: (assetId: string) =>
    apiFetch<Variant[]>(`/api/v1/assets/${assetId}/variants`),

  create: (assetId: string, type: string, params: object) =>
    apiFetch<CreateVariantResponse>(`/api/v1/assets/${assetId}/variants`, {
      method: 'POST',
      body: JSON.stringify({ type, params }),
    }),

  delete: (assetId: string, variantId: string) =>
    apiFetch<void>(`/api/v1/assets/${assetId}/variants/${variantId}`, {
      method: 'DELETE',
    }),

  fileUrl: (assetId: string, variantId: string): string =>
    `${API_BASE}/api/v1/assets/${assetId}/variants/${variantId}/file`,

  previewUrl: (assetId: string, params: {
    w?: number
    h?: number
    fit?: string
    format?: string
    q?: number
  }): string => {
    const qs = new URLSearchParams()
    if (params.w) qs.set('w', String(params.w))
    if (params.h) qs.set('h', String(params.h))
    if (params.fit) qs.set('fit', params.fit)
    if (params.format) qs.set('format', params.format)
    if (params.q) qs.set('q', String(params.q))
    return `${API_BASE}/api/v1/assets/${assetId}/preview?${qs.toString()}`
  },
}



export const shareApi = {
  list: () => apiFetch<Share[]>('/api/v1/shares'),

  get: (id: string) => apiFetch<Share>(`/api/v1/shares/${id}`),

  create: (params: CreateShareParams) =>
    apiFetch<Share>('/api/v1/shares', {
      method: 'POST',
      body: JSON.stringify(params),
    }),

  update: (id: string, params: UpdateShareParams) =>
    apiFetch<Share>(`/api/v1/shares/${id}`, {
      method: 'PUT',
      body: JSON.stringify(params),
    }),

  revoke: (id: string) =>
    apiFetch<void>(`/api/v1/shares/${id}`, { method: 'DELETE' }),
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

export function mimeCategory(mimeType: string): 'image' | 'video' | 'audio' | 'document' {
  if (mimeType.startsWith('image/')) return 'image'
  if (mimeType.startsWith('video/')) return 'video'
  if (mimeType.startsWith('audio/')) return 'audio'
  return 'document'
}
