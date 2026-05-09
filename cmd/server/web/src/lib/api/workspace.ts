import { apiFetch } from './client'
import type {
  AuthResponse,
  Workspace,
  WorkspaceInvite,
  WorkspaceMeResponse,
  WorkspaceMember,
} from './models'

export interface ImageRouterKeyStatus {
  key_set: boolean
  source: 'workspace' | 'env' | 'none'
}

export interface WorkspaceWithRole extends Workspace {
  role: string
  member_count: number
  asset_count: number
}

export interface SwitchWorkspaceResponse {
  token: string
  workspace: Workspace
  role: string
}

export const workspaceApi = {
  fetch: window.fetch,
  useFetch: (
    f: (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>
  ) => (workspaceApi.fetch = f),

  /** GET /api/v1/workspace/me — fetch current workspace info. */
  me: () =>
    apiFetch<WorkspaceMeResponse>(
      '/api/v1/workspace/me',
      undefined,
      workspaceApi.fetch
    ),

  /** PUT /api/v1/workspace/settings (owner only) — update workspace settings. */
  updateSettings: (settings: {
    version_retention_count?: number
    exif_keep?: boolean
    exif_keep_gps?: boolean
    locked_taxonomy?: boolean
  }) =>
    apiFetch<Workspace>('/api/v1/workspace/settings', {
      method: 'PUT',
      body: JSON.stringify(settings),
    }),

  /** POST /api/v1/workspace/jobs/:type/trigger (owner only) — trigger a predefined background job. */
  triggerJob: (type: string) =>
    apiFetch<{ enqueued: number }>(`/api/v1/workspace/jobs/${type}/trigger`, {
      method: 'POST',
    }),

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
    apiFetch<void>(`/api/v1/workspace/invites/${inviteId}`, {
      method: 'DELETE',
    }),

  /** GET /api/v1/workspace/settings/imagerouter (owner only) — get ImageRouter key status. */
  getImageRouterKeyStatus: () =>
    apiFetch<ImageRouterKeyStatus>('/api/v1/workspace/settings/imagerouter'),

  /** PUT /api/v1/workspace/settings/imagerouter (owner only) — set a workspace ImageRouter API key. */
  setImageRouterKey: (key: string) =>
    apiFetch<void>('/api/v1/workspace/settings/imagerouter', {
      method: 'PUT',
      body: JSON.stringify({ key }),
    }),

  /** DELETE /api/v1/workspace/settings/imagerouter (owner only) — clear the workspace ImageRouter API key. */
  clearImageRouterKey: () =>
    apiFetch<void>('/api/v1/workspace/settings/imagerouter', {
      method: 'DELETE',
    }),

  /** POST /api/v1/workspace/settings/imagerouter/test (owner only) — validate the effective ImageRouter key. */
  testImageRouterKey: () =>
    apiFetch<void>('/api/v1/workspace/settings/imagerouter/test', {
      method: 'POST',
    }),
}
