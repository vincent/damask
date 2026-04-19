import type { Asset, AssetFieldsResponse, AssetVersion, AuthResponse, BulkDeleteTagsResult, CreateIngressRuleParams, CreateIngressSourceParams, CreateShareParams, CreateVariantResponse, DuplicateTagPair, FieldDefinition, FieldDefinitionStats, FieldFilter, FieldScope, Folder, IngressLogEntry, IngressRule, IngressSource, ListVariantsResponse, MergeTagsResult, Project, ProjectFieldsResponse, RestoreVersionResponse, Share, ShareComment, Tag, UpdateIngressSourceParams, UpdateShareParams, UploadVersionResponse, Variant, Workspace, WorkspaceMeResponse, WorkspaceMember, WorkspaceInvite, Config, Collection } from "./models"

const API_BASE = import.meta.env.VITE_API_URL ?? ''

/**
 * Custom error class thrown by apiFetch when an API response is not ok.
 * Contains the HTTP status code and error message.
 */
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

/**
 * Base fetch wrapper that injects credentials and Content-Type headers.
 * Redirects to /login on 401 responses. Throws ApiError on non-ok responses.
 * @param path API path (relative to API_BASE)
 * @param init Fetch request options
 * @param fetch Optional custom fetch implementation (defaults to window.fetch)
 */
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

/** Server configuration — fetch public config flags (e.g. demo mode). GET /config (public) */
export const configApi = {
  load: () => apiFetch<Config>('/config')
    .catch(() => {
      console.warn('unable to load config')
      return {
        demo: false,
        mailHost: '' 
      }
    }),
}

/** Authentication endpoints — register, login, token refresh. */
export const authApi = {
  /** POST /auth/register (public) — create a new account. */
  register: (name: string, email: string, password: string) =>
    apiFetch<AuthResponse>('/auth/register', {
      method: 'POST',
      body: JSON.stringify({ name, email, password }),
    }),

  /** POST /auth/login (public) — authenticate with email and password. */
  login: (email: string, password: string) =>
    apiFetch<AuthResponse>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    }),

  /** POST /auth/refresh — refresh the JWT token (requires auth). */
  refresh: () => apiFetch<{ token: string }>('/auth/refresh', { method: 'POST' }),

  /** POST /demo/session (demo build only) — create a demo session. */
  demoSession: () => apiFetch<{ token: string; workspace_id: string; user_id: string; is_demo: boolean }>('/demo/session', { method: 'POST' }),
}

/** Workspace with role information. */
export interface WorkspaceWithRole extends Workspace {
  role: string
}

/** Response from workspace switch endpoint. */
export interface SwitchWorkspaceResponse {
  token: string
  workspace: Workspace
  role: string
}

/** Workspace management endpoints — fetch, list, create, switch, invites. */
export const workspaceApi = {
  fetch: window.fetch,
  useFetch: (f: (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>) => workspaceApi.fetch = f,

  /** GET /api/v1/workspace/me — fetch current workspace info. */
  me: () => apiFetch<WorkspaceMeResponse>('/api/v1/workspace/me', undefined, workspaceApi.fetch),

  /** PUT /api/v1/workspace/settings (owner only) — update workspace settings. */
  updateSettings: (settings: { version_retention_count: number; exif_keep?: boolean; exif_keep_gps?: boolean }) =>
    apiFetch<Workspace>('/api/v1/workspace/settings', {
      method: 'PUT',
      body: JSON.stringify(settings),
    }),

  /** POST /api/v1/workspace/jobs/:type/trigger (owner only) — trigger a predefined background job. */
  triggerJob: (type: string) =>
    apiFetch<{ enqueued: number }>(`/api/v1/workspace/jobs/${type}/trigger`, { method: 'POST' }),

  /** GET /api/v1/workspaces — list all workspaces the user is a member of. */
  list: () => apiFetch<WorkspaceWithRole[]>('/api/v1/workspaces'),

  /** POST /api/v1/workspace/switch — switch to a different workspace. */
  switch: (workspaceId: string) =>
    apiFetch<SwitchWorkspaceResponse>('/api/v1/workspace/switch', {
      method: 'POST',
      body: JSON.stringify({ workspace_id: workspaceId }),
    }),

  /** POST /api/v1/workspace (editor+) — create a new workspace. */
  createWorkspace: (name: string) =>
    apiFetch<AuthResponse>('/api/v1/workspace', {
      method: 'POST',
      body: JSON.stringify({ name }),
    }),

  /** POST /api/v1/workspace/invites (owner only) — send a workspace invitation. */
  createInvite: (email: string, role: 'editor' | 'viewer' = 'editor') =>
    apiFetch<{ id: string; invite_token: string; email: string; role: string }>(
      '/api/v1/workspace/invites',
      { method: 'POST', body: JSON.stringify({ email, role }) }
    ),

  /** POST /auth/invite/accept (public) — accept a workspace invitation. */
  acceptInvite: (token: string, name: string, password: string) =>
    apiFetch<AuthResponse>('/auth/invite/accept', {
      method: 'POST',
      body: JSON.stringify({ token, name, password }),
    }),

  /** GET /api/v1/workspace/members (owner only) — list workspace members. */
  listMembers: () => apiFetch<WorkspaceMember[]>('/api/v1/workspace/members'),

  /** DELETE /api/v1/workspace/members/:userId (owner only) — remove a member. */
  removeMember: (userId: string) =>
    apiFetch<void>(`/api/v1/workspace/members/${userId}`, { method: 'DELETE' }),

  /** PUT /api/v1/workspace/members/:userId (owner only) — change member role. */
  updateMemberRole: (userId: string, role: string) =>
    apiFetch<void>(`/api/v1/workspace/members/${userId}`, {
      method: 'PUT',
      body: JSON.stringify({ role }),
    }),

  /** GET /api/v1/workspace/invites (owner only) — list pending invites. */
  listInvites: () => apiFetch<WorkspaceInvite[]>('/api/v1/workspace/invites'),

  /** DELETE /api/v1/workspace/invites/:inviteId (owner only) — cancel an invite. */
  deleteInvite: (inviteId: string) =>
    apiFetch<void>(`/api/v1/workspace/invites/${inviteId}`, { method: 'DELETE' }),
}


/** Project management endpoints — list, get, create, update, delete. */
export const projectApi = {
  /** GET /api/v1/projects — list projects in current workspace. */
  list: () => apiFetch<Project[]>('/api/v1/projects'),

  /** GET /api/v1/projects/:id — fetch a project by ID. */
  get: (id: string) => apiFetch<Project>(`/api/v1/projects/${id}`),

  /** POST /api/v1/projects (editor+) — create a new project. */
  create: (data: { name: string; description?: string; color?: string }) =>
    apiFetch<Project>('/api/v1/projects', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  /** PUT /api/v1/projects/:id (editor+) — update project metadata. */
  update: (id: string, data: { name?: string; description?: string; color?: string; cover_asset_id?: string }) =>
    apiFetch<Project>(`/api/v1/projects/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  /** DELETE /api/v1/projects/:id (owner only) — delete a project. */
  delete: (id: string) =>
    apiFetch<void>(`/api/v1/projects/${id}`, { method: 'DELETE' }),
}


/** Folder management endpoints — list, create, update, delete. */
export const folderApi = {
  /** GET /api/v1/projects/:id/folders — list folders (hierarchy tree) in a project. */
  list: (projectId: string) => apiFetch<Folder[]>(`/api/v1/projects/${projectId}/folders`),

  /** POST /api/v1/projects/:id/folders (editor+) — create a new folder. */
  create: (projectId: string, data: { name: string; parent_id?: string; position?: number }) =>
    apiFetch<Folder>(`/api/v1/projects/${projectId}/folders`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  /** PUT /api/v1/folders/:id (editor+) — update folder name or position. */
  update: (id: string, data: { name?: string; position?: number }) =>
    apiFetch<Folder>(`/api/v1/folders/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  /** DELETE /api/v1/folders/:id (owner only) — delete a folder. */
  delete: (id: string) =>
    apiFetch<void>(`/api/v1/folders/${id}`, { method: 'DELETE' }),
}



/** Tag management endpoints — list, add, remove, bulk tag, merge, duplicate suggestions. */
export const tagApi = {
  /** GET /api/v1/tags — list all tags in the workspace (with usage counts). */
  list: () => apiFetch<Tag[]>('/api/v1/tags'),

  /** POST /api/v1/tags (editor+) — create a tag directly (asset_count = 0). */
  create: (name: string, color?: string | null, group_name?: string | null) =>
    apiFetch<Tag>('/api/v1/tags', {
      method: 'POST',
      body: JSON.stringify({ name, color, group_name }),
    }),

  /** PATCH /api/v1/tags/:name (editor+) — rename, recolor, or regroup a tag. */
  patch: (name: string, fields: { name?: string; color?: string | null; group_name?: string | null }) =>
    apiFetch<Tag>(`/api/v1/tags/${encodeURIComponent(name)}`, {
      method: 'PATCH',
      body: JSON.stringify(fields),
    }),

  /** DELETE /api/v1/tags (editor+) — bulk delete tags by name. */
  bulkDelete: (names: string[]) =>
    apiFetch<BulkDeleteTagsResult>('/api/v1/tags', {
      method: 'DELETE',
      body: JSON.stringify({ names }),
    }),

  /** POST /api/v1/tags/merge (editor+) — merge source tags into target. */
  merge: (sources: string[], target: string) =>
    apiFetch<MergeTagsResult>('/api/v1/tags/merge', {
      method: 'POST',
      body: JSON.stringify({ sources, target }),
    }),

  /** GET /api/v1/tags/suggestions/duplicates — near-duplicate tag pairs. */
  duplicateSuggestions: () => apiFetch<DuplicateTagPair[]>('/api/v1/tags/suggestions/duplicates'),

  /** GET /api/v1/assets/:id/tags — fetch tags assigned to a specific asset. */
  getForAsset: (assetId: string) => apiFetch<string[]>(`/api/v1/assets/${assetId}/tags`),

  /** POST /api/v1/assets/:id/tags (editor+) — add a tag to an asset (auto-creates if needed). */
  addToAsset: (assetId: string, name: string) =>
    apiFetch<{ name: string }>(`/api/v1/assets/${assetId}/tags`, {
      method: 'POST',
      body: JSON.stringify({ name }),
    }),

  /** DELETE /api/v1/assets/:id/tags/:name (editor+) — remove a tag from an asset. */
  removeFromAsset: (assetId: string, name: string) =>
    apiFetch<void>(`/api/v1/assets/${assetId}/tags/${encodeURIComponent(name)}`, {
      method: 'DELETE',
    }),

  /** POST /api/v1/assets/bulk/tag (editor+) — apply a tag to multiple assets. */
  bulkTag: (assetIds: string[], tagName: string) =>
    apiFetch<void>('/api/v1/assets/bulk/tag', {
      method: 'POST',
      body: JSON.stringify({ asset_ids: assetIds, tag_name: tagName }),
    }),
}

/** Asset list response with pagination cursor. */
export interface AssetListResponse {
  assets: Asset[]
  next_cursor: string | null
}

/** Asset management endpoints — upload, list, get, delete, bulk operations. */
export const assetApi = {
  /** POST /api/v1/assets (editor+) — upload a new asset via XHR (with progress callback). */
  upload(file: File, projectId: string|null, folderId: string|null, onProgress?: (pct: number) => void): Promise<Asset> {
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
      if (projectId) fd.append('project_id', projectId)
      if (folderId) fd.append('folder_id', folderId)
      xhr.send(fd)
    })
  },

  /** GET /api/v1/assets — list assets with pagination, search, filtering, sorting. */
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
    collection_id?: string
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
    if (params.collection_id) qs.set('collection_id', params.collection_id)
    if (params.fieldFilters) {
      for (const f of params.fieldFilters) {
        const paramKey = f.op === 'eq' ? `field[${f.key}]` : `field[${f.key}][${f.op}]`
        qs.set(paramKey, f.value)
      }
    }
    const query = qs.toString()
    return apiFetch<AssetListResponse>(`/api/v1/assets${query ? '?' + query : ''}`)
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

  /** GET /api/v1/assets/:id — fetch asset metadata by ID. */
  get(id: string): Promise<Asset> {
    return apiFetch<Asset>(`/api/v1/assets/${id}`)
  },

  /** DELETE /api/v1/assets/:id (editor+) — delete an asset. */
  delete(id: string): Promise<void> {
    return apiFetch<void>(`/api/v1/assets/${id}`, { method: 'DELETE' })
  },

  /** GET /api/v1/assets/:id/thumb — thumbnail URL for an asset. */
  thumbUrl(id: string): string {
    return `${API_BASE}/api/v1/assets/${id}/thumb`
  },

  /** GET /api/v1/assets/:id/file — original file URL for an asset. */
  fileUrl(id: string): string {
    return `${API_BASE}/api/v1/assets/${id}/file`
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

  /** GET /api/v1/assets/:id/comments — list comments for an asset. */
  listAssetComments: (id: string) =>
    apiFetch<ShareComment[]>(`${API_BASE}/api/v1/assets/${id}/comments`)
}


/** Field definition endpoints — list, get, stats, create, update, delete, reorder. */
export const fieldDefinitionApi = {
  /** GET /api/v1/field-definitions — list field definitions by scope (workspace or project). */
  list: (scope: FieldScope) =>
    apiFetch<FieldDefinition[]>(`/api/v1/field-definitions?scope=${scope}`),

  /** GET /api/v1/field-definitions/:id — fetch a field definition by ID. */
  get: (id: string) =>
    apiFetch<FieldDefinition>(`/api/v1/field-definitions/${id}`),

  /** GET /api/v1/field-definitions/:id/stats — fetch usage stats for a field definition. */
  stats: (id: string) =>
    apiFetch<FieldDefinitionStats>(`/api/v1/field-definitions/${id}/stats`),

  /** POST /api/v1/field-definitions (editor+) — create a new field definition. */
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

  /** PUT /api/v1/field-definitions/:id (editor+) — update a field definition. */
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

  /** DELETE /api/v1/field-definitions/:id (editor+) — delete a field definition. */
  delete: (id: string) =>
    apiFetch<void>(`/api/v1/field-definitions/${id}`, { method: 'DELETE' }),

  /** PUT /api/v1/field-definitions/reorder (editor+) — reorder field definitions. */
  reorder: (entries: { id: string; position: number }[]) =>
    apiFetch<void>('/api/v1/field-definitions/reorder', {
      method: 'PUT',
      body: JSON.stringify(entries),
    }),
}

/** Asset field value endpoints — get and patch asset custom field values. */
export const assetFieldApi = {
  /** GET /api/v1/assets/:id/fields — fetch field values for an asset. */
  get: (assetId: string) =>
    apiFetch<AssetFieldsResponse>(`/api/v1/assets/${assetId}/fields`),

  /** PATCH /api/v1/assets/:id/fields (editor+) — update field values for an asset. */
  patch: (assetId: string, values: { field_id: string; value: string | number | boolean | null }[]) =>
    apiFetch<AssetFieldsResponse>(`/api/v1/assets/${assetId}/fields`, {
      method: 'PATCH',
      body: JSON.stringify({ values }),
    }),
}

/** Project field value endpoints — get and patch project custom field values. */
export const projectFieldApi = {
  /** GET /api/v1/projects/:id/fields — fetch field values for a project. */
  get: (projectId: string) =>
    apiFetch<ProjectFieldsResponse>(`/api/v1/projects/${projectId}/fields`),

  /** PATCH /api/v1/projects/:id/fields (editor+) — update field values for a project. */
  patch: (projectId: string, values: { field_id: string; value: string | number | boolean | null }[]) =>
    apiFetch<ProjectFieldsResponse>(`/api/v1/projects/${projectId}/fields`, {
      method: 'PATCH',
      body: JSON.stringify({ values }),
    }),
}

/** Asset version management endpoints — list, upload, restore, delete, file/thumbnail URLs. */
export const versionApi = {
  /** GET /api/v1/assets/:id/versions — list all versions of an asset. */
  list: (assetId: string) =>
    apiFetch<AssetVersion[]>(`/api/v1/assets/${assetId}/versions`),

  /** POST /api/v1/assets/:id/versions (editor+) — upload a new version via XHR (with progress). */
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

  /** POST /api/v1/assets/:id/versions/:vid/restore (editor+) — restore a previous version as current. */
  restore: (assetId: string, versionId: string) =>
    apiFetch<RestoreVersionResponse>(`/api/v1/assets/${assetId}/versions/${versionId}/restore`, { method: 'POST' }),

  /** DELETE /api/v1/assets/:id/versions/:vid (owner only) — delete a version. */
  delete: (assetId: string, versionId: string) =>
    apiFetch<void>(`/api/v1/assets/${assetId}/versions/${versionId}`, { method: 'DELETE' }),

  /** GET /api/v1/assets/:id/versions/:vid/file — file URL for a specific version. */
  fileUrl: (assetId: string, versionId: string): string =>
    `${API_BASE}/api/v1/assets/${assetId}/versions/${versionId}/file`,

  /** GET /api/v1/assets/:id/versions/:vid/thumb — thumbnail URL for a specific version. */
  thumbUrl: (assetId: string, versionId: string): string =>
    `${API_BASE}/api/v1/assets/${assetId}/versions/${versionId}/thumb`,
}

/** Opens a Server-Sent Events stream for workspace-scoped real-time events (e.g. thumbnail_ready). */
export function openThumbnailEvents(): EventSource {
  return new EventSource(`${API_BASE}/api/v1/events`, { withCredentials: true })
}

/** Variant management endpoints — list, create, upload, delete, file/preview URLs. */
export const variantApi = {
  /** GET /api/v1/assets/:id/variants — list all variants for an asset. */
  list: (assetId: string) =>
    apiFetch<ListVariantsResponse>(`/api/v1/assets/${assetId}/variants`),

  /** POST /api/v1/assets/:id/variants (editor+) — enqueue a variant generation job. */
  create: (assetId: string, type: string, params: object) =>
    apiFetch<CreateVariantResponse>(`/api/v1/assets/${assetId}/variants`, {
      method: 'POST',
      body: JSON.stringify({ type, params }),
    }),

  /** POST /api/v1/assets/:id/variants/upload (editor+) — upload a manually-created variant. */
  uploadManual: (assetId: string, file: File) => {
    const fd = new FormData()
    fd.append('file', file)
    return apiFetch<Variant>(`/api/v1/assets/${assetId}/variants/upload`, {
      method: 'POST',
      headers: {},
      body: fd,
    })
  },

  /** DELETE /api/v1/assets/:id/variants/:vid (editor+) — delete a variant. */
  delete: (assetId: string, variantId: string) =>
    apiFetch<void>(`/api/v1/assets/${assetId}/variants/${variantId}`, {
      method: 'DELETE',
    }),

  /** GET /api/v1/assets/:id/variants/:vid/file — file URL for a variant. */
  fileUrl: (assetId: string, variantId: string): string =>
    `${API_BASE}/api/v1/assets/${assetId}/variants/${variantId}/file`,

  /** GET /api/v1/assets/:id/preview — in-memory cached preview transform URL (query params: w, h, fit, format, q). */
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



/** Share management endpoints — list, get, create, update, revoke. */
export const shareApi = {
  /** GET /api/v1/shares — list all shares in the workspace. */
  list: () => apiFetch<Share[]>('/api/v1/shares'),

  /** GET /api/v1/shares/:id — fetch a share by ID. */
  get: (id: string) => apiFetch<Share>(`/api/v1/shares/${id}`),

  /** POST /api/v1/shares — create a new share link for assets. */
  create: (params: CreateShareParams) =>
    apiFetch<Share>('/api/v1/shares', {
      method: 'POST',
      body: JSON.stringify(params),
    }),

  /** PUT /api/v1/shares/:id — update share settings (name, expiry, password). */
  update: (id: string, params: UpdateShareParams) =>
    apiFetch<Share>(`/api/v1/shares/${id}`, {
      method: 'PUT',
      body: JSON.stringify(params),
    }),

  /** DELETE /api/v1/shares/:id — revoke a share link. */
  revoke: (id: string) =>
    apiFetch<void>(`/api/v1/shares/${id}`, { method: 'DELETE' }),
}

/**
 * Ingress (asset import from external sources) endpoints.
 * Includes: sources CRUD, rules, and event log.
 */
export const ingressApi = {
  // Sources
  /** GET /api/v1/ingress/sources — list all ingress sources in the workspace. */
  list: () => apiFetch<IngressSource[]>('/api/v1/ingress/sources'),

  /** GET /api/v1/ingress/sources/:id — fetch an ingress source by ID. */
  get: (id: string) => apiFetch<IngressSource>(`/api/v1/ingress/sources/${id}`),

  /** POST /api/v1/ingress/sources (editor+) — create a new ingress source. */
  create: (params: CreateIngressSourceParams) =>
    apiFetch<IngressSource>('/api/v1/ingress/sources', {
      method: 'POST',
      body: JSON.stringify(params),
    }),

  /** PUT /api/v1/ingress/sources/:id (editor+) — update an ingress source config. */
  update: (id: string, params: UpdateIngressSourceParams) =>
    apiFetch<IngressSource>(`/api/v1/ingress/sources/${id}`, {
      method: 'PUT',
      body: JSON.stringify(params),
    }),

  /** DELETE /api/v1/ingress/sources/:id (owner only) — delete an ingress source. */
  delete: (id: string) =>
    apiFetch<void>(`/api/v1/ingress/sources/${id}`, { method: 'DELETE' }),

  /** POST /api/v1/ingress/sources/:id/test (editor+) — test connection to source. */
  test: (id: string) =>
    apiFetch<{ ok: boolean }>(`/api/v1/ingress/sources/${id}/test`, { method: 'POST' }),

  /** POST /api/v1/ingress/sources/:id/poll (editor+) — manually trigger a poll job. */
  poll: (id: string) =>
    apiFetch<{ job_id: string }>(`/api/v1/ingress/sources/${id}/poll`, { method: 'POST' }),

  // Rules
  /** GET /api/v1/ingress/sources/:id/rules — list rules for a source. */
  listRules: (sourceId: string) =>
    apiFetch<IngressRule[]>(`/api/v1/ingress/sources/${sourceId}/rules`),

  /** POST /api/v1/ingress/sources/:id/rules (editor+) — create an ingress rule. */
  createRule: (sourceId: string, params: CreateIngressRuleParams) =>
    apiFetch<IngressRule>(`/api/v1/ingress/sources/${sourceId}/rules`, {
      method: 'POST',
      body: JSON.stringify(params),
    }),

  /** PUT /api/v1/ingress/sources/:id/rules/:rid (editor+) — update an ingress rule. */
  updateRule: (sourceId: string, ruleId: string, params: Partial<CreateIngressRuleParams>) =>
    apiFetch<IngressRule>(`/api/v1/ingress/sources/${sourceId}/rules/${ruleId}`, {
      method: 'PUT',
      body: JSON.stringify(params),
    }),

  /** DELETE /api/v1/ingress/sources/:id/rules/:rid (editor+) — delete an ingress rule. */
  deleteRule: (sourceId: string, ruleId: string) =>
    apiFetch<void>(`/api/v1/ingress/sources/${sourceId}/rules/${ruleId}`, { method: 'DELETE' }),

  /** PUT /api/v1/ingress/sources/:id/rules/reorder (editor+) — reorder rules by position. */
  reorderRules: (sourceId: string, entries: { id: string; position: number }[]) =>
    apiFetch<IngressRule[]>(`/api/v1/ingress/sources/${sourceId}/rules/reorder`, {
      method: 'PUT',
      body: JSON.stringify(entries),
    }),

  // Log
  /** GET /api/v1/ingress/sources/:id/log — list ingress log entries for a source (filter by status). */
  getSourceLog: (sourceId: string, status?: string) => {
    const qs = status ? `?status=${status}` : ''
    return apiFetch<IngressLogEntry[]>(`/api/v1/ingress/sources/${sourceId}/log${qs}`)
  },

  /** GET /api/v1/ingress/log — list workspace-wide ingress log entries (filter by status). */
  getWorkspaceLog: (status?: string) => {
    const qs = status ? `?status=${status}` : ''
    return apiFetch<IngressLogEntry[]>(`/api/v1/ingress/log${qs}`)
  },

  /** DELETE /api/v1/ingress/log/:id (editor+) — delete a log entry. */
  deleteLogEntry: (entryId: string) =>
    apiFetch<void>(`/api/v1/ingress/log/${entryId}`, { method: 'DELETE' }),

  /** POST /api/v1/ingress/log/:id/retry (editor+) — retry a failed ingestion job. */
  retryLogEntry: (entryId: string) =>
    apiFetch<{ job_id: string }>(`/api/v1/ingress/log/${entryId}/retry`, { method: 'POST' }),
}

/**
 * Formats a byte count as a human-readable string.
 * @param bytes Number of bytes
 * @returns Formatted string (e.g., "1.5 MB")
 */
export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

/** Activity log and audit event endpoints — list asset/project/workspace events, export. */
export const activityApi = {
  /** GET /api/v1/assets/:id/events — list audit events for an asset (paginated). */
  listAssetEvents: (assetId: string, params: { limit?: number; cursor?: string; types?: string } = {}) => {
    const q = new URLSearchParams()
    if (params.limit) q.set('limit', String(params.limit))
    if (params.cursor) q.set('cursor', params.cursor)
    if (params.types) q.set('types', params.types)
    const qs = q.toString()
    return apiFetch<import('./models').AuditLogResponse>(`/api/v1/assets/${assetId}/events${qs ? '?' + qs : ''}`)
  },

  /** GET /api/v1/projects/:id/events — list audit events for a project (paginated). */
  listProjectEvents: (projectId: string, params: { limit?: number; cursor?: string; types?: string } = {}) => {
    const q = new URLSearchParams()
    if (params.limit) q.set('limit', String(params.limit))
    if (params.cursor) q.set('cursor', params.cursor)
    if (params.types) q.set('types', params.types)
    const qs = q.toString()
    return apiFetch<import('./models').AuditLogResponse>(`/api/v1/projects/${projectId}/events${qs ? '?' + qs : ''}`)
  },

  /** GET /api/v1/activity — list workspace-wide activity feed (paginated, filterable by user and event type). */
  listWorkspaceActivity: (params: { limit?: number; cursor?: string; types?: string; user_id?: string } = {}) => {
    const q = new URLSearchParams()
    if (params.limit) q.set('limit', String(params.limit))
    if (params.cursor) q.set('cursor', params.cursor)
    if (params.types) q.set('types', params.types)
    if (params.user_id) q.set('user_id', params.user_id)
    const qs = q.toString()
    return apiFetch<import('./models').ActivityFeedResponse>(`/api/v1/activity${qs ? '?' + qs : ''}`)
  },

  /** GET /api/v1/activity/export — export workspace activity as CSV (optionally filtered by date range). */
  exportCSV: (params: { since?: string; until?: string } = {}) => {
    const q = new URLSearchParams({ format: 'csv' })
    if (params.since) q.set('since', params.since)
    if (params.until) q.set('until', params.until)
    return `${API_BASE}/api/v1/activity/export?${q.toString()}`
  },
}

/** Stack endpoints — export as ZIP. */
export const stackApi = {
  /** POST /api/v1/stack/export — download stacked assets as a ZIP file. */
  exportZip: async (assetIds: string[], filename = 'stack-export'): Promise<void> => {
    const res = await fetch(`${API_BASE}/api/v1/stack/export`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ asset_ids: assetIds, filename }),
    })
    if (!res.ok) {
      const body = await res.json().catch(() => ({ error: res.statusText }))
      throw new Error(body.error ?? res.statusText)
    }
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${filename}.zip`
    a.click()
    URL.revokeObjectURL(url)
  },

  /** POST /api/v1/stack/merge — enqueue a GIF or PDF merge job. Returns job_id. */
  merge: async (assetIds: string[], outputType: 'gif' | 'pdf', filename = 'stack-merge', gifFrameMs = 500): Promise<string> => {
    const res = await apiFetch<{ job_id: string }>('/api/v1/stack/merge', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ asset_ids: assetIds, output_type: outputType, filename, gif_frame_ms: gifFrameMs }),
    })
    return res.job_id
  },
}

export const collectionApi = {
  list: (): Promise<Collection[]> => apiFetch('/api/v1/collections'),
  create: (name: string, description = '', assetIds: string[] = []): Promise<Collection> =>
    apiFetch('/api/v1/collections', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, description, asset_ids: assetIds }),
    }),
  get: (id: string): Promise<Collection & { assets: import('./models').Asset[] }> =>
    apiFetch(`/api/v1/collections/${id}`),
  update: (id: string, name: string, description = ''): Promise<Collection> =>
    apiFetch(`/api/v1/collections/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, description }),
    }),
  delete: (id: string): Promise<void> => apiFetch(`/api/v1/collections/${id}`, { method: 'DELETE' }),
  addAsset: (collectionId: string, assetId: string): Promise<void> =>
    apiFetch(`/api/v1/collections/${collectionId}/assets/${assetId}`, { method: 'POST' }),
  removeAsset: (collectionId: string, assetId: string): Promise<void> =>
    apiFetch(`/api/v1/collections/${collectionId}/assets/${assetId}`, { method: 'DELETE' }),
  forAsset: (assetId: string): Promise<Collection[]> =>
    apiFetch(`/api/v1/assets/${assetId}/collections`),
}


/**
 * Maps a MIME type string to a broad asset category.
 * @param mimeType MIME type string (e.g., "image/jpeg")
 * @returns Category: 'image' | 'video' | 'audio' | 'document'
 */
export function mimeCategory(mimeType: string): 'image' | 'video' | 'audio' | 'document' {
  if (mimeType.startsWith('image/')) return 'image'
  if (mimeType.startsWith('video/')) return 'video'
  if (mimeType.startsWith('audio/')) return 'audio'
  return 'document'
}
