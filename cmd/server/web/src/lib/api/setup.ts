import { apiFetch } from './client'
import type {
  DepStatus,
  EnvParams,
  OwnerParams,
  SetupStatus,
  StorageParams,
} from './models'

export const setupApi = {
  status: () => apiFetch<SetupStatus>('/api/setup/status'),

  validateStorage: (params: StorageParams) =>
    apiFetch<{ ok: boolean; reason?: string }>('/api/setup/validate-storage', {
      method: 'POST',
      body: JSON.stringify(params),
    }),

  deps: () => apiFetch<DepStatus[]>('/api/setup/deps'),

  writeConfig: (params: EnvParams) =>
    apiFetch<void>('/api/setup/config', {
      method: 'POST',
      body: JSON.stringify(params),
    }),

  createOwner: (params: OwnerParams) =>
    apiFetch<void>('/api/setup/owner', {
      method: 'POST',
      body: JSON.stringify(params),
    }),
}
