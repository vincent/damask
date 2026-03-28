const API_BASE = 'http://localhost:8080'

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

export async function apiFetch<T>(
  path: string,
  init: RequestInit = {}
): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    credentials: 'include', // send auth_token cookie
    headers: { 'Content-Type': 'application/json', ...init.headers },
    ...init,
  })

  if (res.status === 401 && typeof window !== 'undefined') {
    window.location.href = '/login'
    throw new ApiError(401, 'Unauthorized')
  }

  return handleResponse<T>(res)
}

// ---- Auth ----

export interface User {
  id: string
  workspace_id: string
  email: string
  name: string
  created_at: string
}

export interface Workspace {
  id: string
  name: string
  created_at: string
  updated_at: string
}

export interface AuthResponse {
  token: string
  user: User
  workspace?: Workspace
}

export interface WorkspaceMeResponse {
  workspace: Workspace
  user: User
  role: string
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

export const workspaceApi = {
  me: () => apiFetch<WorkspaceMeResponse>('/api/v1/workspace/me'),

  createInvite: (email: string, role: 'editor' | 'viewer' = 'editor') =>
    apiFetch<{ id: string; token: string; email: string; role: string }>(
      '/api/v1/workspace/invites',
      { method: 'POST', body: JSON.stringify({ email, role }) }
    ),

  acceptInvite: (token: string, name: string, password: string) =>
    apiFetch<AuthResponse>('/api/v1/workspace/invites/accept', {
      method: 'POST',
      body: JSON.stringify({ token, name, password }),
    }),
}
