import { apiFetch } from './client'

export interface TelemetryStatus {
  enabled: boolean
  service_name: string
  env: string
}

export const telemetryApi = {
  getStatus: () => apiFetch<TelemetryStatus>('/api/admin/telemetry'),
}
