import type { Asset, AssetFieldsResponse, AssetVersion, AuthResponse, CreateIngressRuleParams, CreateIngressSourceParams, CreateShareParams, CreateVariantResponse, FieldDefinition, FieldDefinitionStats, FieldFilter, FieldScope, Folder, IngressLogEntry, IngressRule, IngressSource, Project, ProjectFieldsResponse, RestoreVersionResponse, Share, Tag, UpdateIngressSourceParams, UpdateShareParams, UploadVersionResponse, Variant, Workspace, WorkspaceMeResponse } from "./models"

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
  if (res.status === 204) {
    // No content
    return {} as T
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

  updateSettings: (settings: { version_retention_count: number }) =>
    apiFetch<Workspace>('/api/v1/workspace/settings', {
      method: 'PUT',
      body: JSON.stringify(settings),
    }),

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
    sortKey?: string
    sortAsc?: boolean
    q?: string
    project_id?: string
    mime?: string
    tags?: string[]
    folder_id?: string
    fieldFilters?: FieldFilter[]
  } = {}): Promise<AssetListResponse> {
    const qs = new URLSearchParams()
    if (params.sortKey) qs.set('sort', `${params.sortKey}_${params.sortAsc ? 'asc' : 'desc'}`)
    if (params.cursor) qs.set('cursor', params.cursor)
    if (params.limit) qs.set('limit', String(params.limit))
    if (params.q) qs.set('q', params.q)
    if (params.project_id) qs.set('project_id', params.project_id)
    if (params.mime) qs.set('mime', params.mime)
    if (params.tags && params.tags.length > 0) qs.set('tags', params.tags.join(','))
    if (params.folder_id) qs.set('folder_id', params.folder_id)
    if (params.fieldFilters) {
      for (const f of params.fieldFilters) {
        const paramKey = f.op === 'eq' ? `field[${f.key}]` : `field[${f.key}][${f.op}]`
        qs.set(paramKey, f.value)
      }
    }
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


export const fieldDefinitionApi = {
  list: (scope: FieldScope) =>
    apiFetch<FieldDefinition[]>(`/api/v1/field-definitions?scope=${scope}`),

  get: (id: string) =>
    apiFetch<FieldDefinition>(`/api/v1/field-definitions/${id}`),

  stats: (id: string) =>
    apiFetch<FieldDefinitionStats>(`/api/v1/field-definitions/${id}/stats`),

  create: (params: {
    scope: FieldScope
    name: string
    key: string
    field_type: string
    options?: string | null
    required?: boolean
    position?: number
    inherit_from_project?: boolean
  }) =>
    apiFetch<FieldDefinition>('/api/v1/field-definitions', {
      method: 'POST',
      body: JSON.stringify(params),
    }),

  update: (id: string, params: {
    name?: string
    options?: string | null
    required?: boolean
    position?: number
    inherit_from_project?: boolean
  }) =>
    apiFetch<FieldDefinition>(`/api/v1/field-definitions/${id}`, {
      method: 'PUT',
      body: JSON.stringify(params),
    }),

  delete: (id: string) =>
    apiFetch<void>(`/api/v1/field-definitions/${id}`, { method: 'DELETE' }),

  reorder: (entries: { id: string; position: number }[]) =>
    apiFetch<void>('/api/v1/field-definitions/reorder', {
      method: 'PUT',
      body: JSON.stringify(entries),
    }),
}

export const assetFieldApi = {
  get: (assetId: string) =>
    apiFetch<AssetFieldsResponse>(`/api/v1/assets/${assetId}/fields`),

  patch: (assetId: string, values: { field_id: string; value: string | number | boolean | null }[]) =>
    apiFetch<AssetFieldsResponse>(`/api/v1/assets/${assetId}/fields`, {
      method: 'PATCH',
      body: JSON.stringify({ values }),
    }),
}

export const projectFieldApi = {
  get: (projectId: string) =>
    apiFetch<ProjectFieldsResponse>(`/api/v1/projects/${projectId}/fields`),

  patch: (projectId: string, values: { field_id: string; value: string | number | boolean | null }[]) =>
    apiFetch<ProjectFieldsResponse>(`/api/v1/projects/${projectId}/fields`, {
      method: 'PATCH',
      body: JSON.stringify({ values }),
    }),
}

export const versionApi = {
  list: (assetId: string) =>
    apiFetch<AssetVersion[]>(`/api/v1/assets/${assetId}/versions`),

  upload(assetId: string, file: File, comment: string, onProgress?: (pct: number) => void): Promise<UploadVersionResponse> {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest()
      xhr.open('POST', `${API_BASE}/api/v1/assets/${assetId}/versions`)
      xhr.withCredentials = true
      if (onProgress) {
        xhr.upload.addEventListener('progress', (e) => {
          if (e.lengthComputable) onProgress(Math.round((e.loaded / e.total) * 100))
        })
      }
      xhr.addEventListener('load', () => {
        if (xhr.status === 201) {
          resolve(JSON.parse(xhr.responseText) as UploadVersionResponse)
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
      if (comment) fd.append('comment', comment)
      xhr.send(fd)
    })
  },

  restore: (assetId: string, versionId: string) =>
    apiFetch<RestoreVersionResponse>(`/api/v1/assets/${assetId}/versions/${versionId}/restore`, { method: 'POST' }),

  delete: (assetId: string, versionId: string) =>
    apiFetch<void>(`/api/v1/assets/${assetId}/versions/${versionId}`, { method: 'DELETE' }),

  fileUrl: (assetId: string, versionId: string): string =>
    `${API_BASE}/api/v1/assets/${assetId}/versions/${versionId}/file`,

  thumbUrl: (assetId: string, versionId: string): string =>
    `${API_BASE}/api/v1/assets/${assetId}/versions/${versionId}/thumb`,
}

export function openThumbnailEvents(): EventSource {
  return new EventSource(`${API_BASE}/api/v1/events`, { withCredentials: true })
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

export const ingressApi = {
  // Sources
  list: () => apiFetch<IngressSource[]>('/api/v1/ingress/sources'),

  get: (id: string) => apiFetch<IngressSource>(`/api/v1/ingress/sources/${id}`),

  create: (params: CreateIngressSourceParams) =>
    apiFetch<IngressSource>('/api/v1/ingress/sources', {
      method: 'POST',
      body: JSON.stringify(params),
    }),

  update: (id: string, params: UpdateIngressSourceParams) =>
    apiFetch<IngressSource>(`/api/v1/ingress/sources/${id}`, {
      method: 'PUT',
      body: JSON.stringify(params),
    }),

  delete: (id: string) =>
    apiFetch<void>(`/api/v1/ingress/sources/${id}`, { method: 'DELETE' }),

  test: (id: string) =>
    apiFetch<{ ok: boolean }>(`/api/v1/ingress/sources/${id}/test`, { method: 'POST' }),

  poll: (id: string) =>
    apiFetch<{ job_id: string }>(`/api/v1/ingress/sources/${id}/poll`, { method: 'POST' }),

  // Rules
  listRules: (sourceId: string) =>
    apiFetch<IngressRule[]>(`/api/v1/ingress/sources/${sourceId}/rules`),

  createRule: (sourceId: string, params: CreateIngressRuleParams) =>
    apiFetch<IngressRule>(`/api/v1/ingress/sources/${sourceId}/rules`, {
      method: 'POST',
      body: JSON.stringify(params),
    }),

  updateRule: (sourceId: string, ruleId: string, params: Partial<CreateIngressRuleParams>) =>
    apiFetch<IngressRule>(`/api/v1/ingress/sources/${sourceId}/rules/${ruleId}`, {
      method: 'PUT',
      body: JSON.stringify(params),
    }),

  deleteRule: (sourceId: string, ruleId: string) =>
    apiFetch<void>(`/api/v1/ingress/sources/${sourceId}/rules/${ruleId}`, { method: 'DELETE' }),

  reorderRules: (sourceId: string, entries: { id: string; position: number }[]) =>
    apiFetch<IngressRule[]>(`/api/v1/ingress/sources/${sourceId}/rules/reorder`, {
      method: 'PUT',
      body: JSON.stringify(entries),
    }),

  // Log
  getSourceLog: (sourceId: string, status?: string) => {
    const qs = status ? `?status=${status}` : ''
    return apiFetch<IngressLogEntry[]>(`/api/v1/ingress/sources/${sourceId}/log${qs}`)
  },

  getWorkspaceLog: (status?: string) => {
    const qs = status ? `?status=${status}` : ''
    return apiFetch<IngressLogEntry[]>(`/api/v1/ingress/log${qs}`)
  },

  deleteLogEntry: (entryId: string) =>
    apiFetch<void>(`/api/v1/ingress/log/${entryId}`, { method: 'DELETE' }),

  retryLogEntry: (entryId: string) =>
    apiFetch<{ job_id: string }>(`/api/v1/ingress/log/${entryId}/retry`, { method: 'POST' }),
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

export const activityApi = {
  listAssetEvents: (assetId: string, params: { limit?: number; cursor?: string; types?: string } = {}) => {
    const q = new URLSearchParams()
    if (params.limit) q.set('limit', String(params.limit))
    if (params.cursor) q.set('cursor', params.cursor)
    if (params.types) q.set('types', params.types)
    const qs = q.toString()
    return apiFetch<import('./models').AuditLogResponse>(`/api/v1/assets/${assetId}/events${qs ? '?' + qs : ''}`)
  },

  listProjectEvents: (projectId: string, params: { limit?: number; cursor?: string; types?: string } = {}) => {
    const q = new URLSearchParams()
    if (params.limit) q.set('limit', String(params.limit))
    if (params.cursor) q.set('cursor', params.cursor)
    if (params.types) q.set('types', params.types)
    const qs = q.toString()
    return apiFetch<import('./models').AuditLogResponse>(`/api/v1/projects/${projectId}/events${qs ? '?' + qs : ''}`)
  },

  listWorkspaceActivity: (params: { limit?: number; cursor?: string; types?: string; user_id?: string } = {}) => {
    const q = new URLSearchParams()
    if (params.limit) q.set('limit', String(params.limit))
    if (params.cursor) q.set('cursor', params.cursor)
    if (params.types) q.set('types', params.types)
    if (params.user_id) q.set('user_id', params.user_id)
    const qs = q.toString()
    return apiFetch<import('./models').ActivityFeedResponse>(`/api/v1/activity${qs ? '?' + qs : ''}`)
  },

  exportCSV: (params: { since?: string; until?: string } = {}) => {
    const q = new URLSearchParams({ format: 'csv' })
    if (params.since) q.set('since', params.since)
    if (params.until) q.set('until', params.until)
    return `${API_BASE}/api/v1/activity/export?${q.toString()}`
  },
}

export function mimeCategory(mimeType: string): 'image' | 'video' | 'audio' | 'document' {
  if (mimeType.startsWith('image/')) return 'image'
  if (mimeType.startsWith('video/')) return 'video'
  if (mimeType.startsWith('audio/')) return 'audio'
  return 'document'
}
