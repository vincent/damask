import { apiFetch } from './client'

export interface WorkflowGraphPosition {
  x: number
  y: number
}

export interface WorkflowGraphNode {
  id: string
  type: string
  config: Record<string, unknown>
  position: WorkflowGraphPosition
}

export interface WorkflowGraphEdge {
  id?: string
  from_node: string
  from_port: string
  to_node: string
  to_port: string
}

export interface WorkflowGraph {
  nodes: WorkflowGraphNode[]
  edges: WorkflowGraphEdge[]
}

export interface Workflow {
  id: string
  workspace_id: string
  name: string
  description: string
  enabled: boolean
  trigger_type: string
  graph: string
  notify_on_failure_email: string
  last_run_at: string | null
  created_at: string
  updated_at: string
}

export interface WorkflowRunStep {
  node_id: string
  node_type: string
  status: string
  attempt: number
  input_ctx: Record<string, unknown>
  output_ctx: Record<string, unknown>
  error: string | null
  started_at: string | null
  completed_at: string | null
}

export interface WorkflowRun {
  id: string
  workflow_id: string
  status: string
  trigger_data: Record<string, unknown>
  error: string | null
  started_at: string | null
  completed_at: string | null
  steps: WorkflowRunStep[]
  created_at: string
}

export interface WorkflowNodePort {
  id: string
  label: string
}

export interface WorkflowNodeSchema {
  type: string
  label: string
  category: string
  description: string
  inputs: WorkflowNodePort[]
  outputs: WorkflowNodePort[]
  config_schema: WorkflowNodeConfigSchema
}

export interface WorkflowNodeConfigSchema {
  type?: string
  title?: string
  format?: string
  placeholder?: string
  required?: string[]
  enum?: string[]
  items?: WorkflowNodeConfigSchema
  properties?: Record<string, WorkflowNodeConfigSchema>
  additionalProperties?: boolean
}

export interface WorkflowTemplate {
  id: string
  name: string
  description: string
  trigger_type: string
  graph: string
}

export interface CreateWorkflowBody {
  name: string
  description: string
  graph: string
  notify_on_failure_email?: string
}

export interface UpdateWorkflowBody {
  name: string
  description: string
  graph: string
  notify_on_failure_email?: string
}

export interface WorkflowEvent {
  type: string
  asset_id: string
  workflow_id?: string
  run_id?: string
  node_id?: string
  status?: string
  error?: string
}

export const workflowsApi = {
  list: () => apiFetch<Workflow[]>('/api/v1/workflows'),
  get: (id: string) => apiFetch<Workflow>(`/api/v1/workflows/${id}`),
  create: (body: CreateWorkflowBody) =>
    apiFetch<Workflow>('/api/v1/workflows', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  update: (id: string, body: UpdateWorkflowBody) =>
    apiFetch<Workflow>(`/api/v1/workflows/${id}`, {
      method: 'PUT',
      body: JSON.stringify(body),
    }),
  setEnabled: (id: string, enabled: boolean) =>
    apiFetch<void>(`/api/v1/workflows/${id}/enabled`, {
      method: 'PATCH',
      body: JSON.stringify({ enabled }),
    }),
  delete: (id: string) =>
    apiFetch<void>(`/api/v1/workflows/${id}`, {
      method: 'DELETE',
    }),
  triggerManual: (id: string) =>
    apiFetch<{ run_id: string; status: string }>(
      `/api/v1/workflows/${id}/runs`,
      {
        method: 'POST',
      }
    ),
  listRuns: (id: string, cursor?: string) =>
    apiFetch<WorkflowRun[]>(
      `/api/v1/workflows/${id}/runs${cursor ? `?cursor=${encodeURIComponent(cursor)}` : ''}`
    ),
  getRun: (id: string, runId: string) =>
    apiFetch<WorkflowRun>(`/api/v1/workflows/${id}/runs/${runId}`),
  getNodeSchemas: () =>
    apiFetch<WorkflowNodeSchema[]>('/api/v1/workflows/node-schemas'),
  getTemplates: () =>
    apiFetch<WorkflowTemplate[]>('/api/v1/workflows/templates'),
  getWebhookToken: (id: string) =>
    apiFetch<{ token: string }>(`/api/v1/workflows/${id}/webhook-token`),
  regenerateWebhook: (id: string) =>
    apiFetch<{ token: string }>(
      `/api/v1/workflows/${id}/webhook-token/regenerate`,
      {
        method: 'POST',
      }
    ),
}
