import { apiFetch } from './client'
import type { CreateShareParams, Share, UpdateShareParams } from './models'

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
