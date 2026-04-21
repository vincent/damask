export interface Config {
  demo: boolean
  mailHost: string
}

export interface MenuItem {
  id: string | null
  label: string
  url?: string
  color?: string
  count?: number
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
  version_retention_count: number
  exif_keep: boolean
  exif_keep_gps: boolean
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
  total_asset_count: number
}

// ---- Assets ----

export interface Asset {
  id: string
  workspace_id: string
  project_id: string | null
  folder_id: string | null
  original_filename: string
  mime_type: string
  size: number
  width: number | null
  height: number | null
  thumbnail_key: string | null
  metadata: string | null
  tags: string[]
  version_count: number
  variant_count: number
  variants_rebuilding: boolean
  created_at: string
  updated_at: string
}

export interface AssetVersionCreatedBy {
  id: string
  name: string
}

export interface AssetVersion {
  id: string
  version_num: number
  mime_type: string
  size: number
  width: number | null
  height: number | null
  duration_sec: number | null
  thumbnail_url: string | null
  comment: string | null
  created_by: AssetVersionCreatedBy | null
  created_at: string
  is_current: boolean
  variant_count: number
}

export interface UploadVersionResponse {
  version: AssetVersion
  asset: Asset
}

export interface RestoreVersionResponse {
  version: AssetVersion
  asset: Asset
}

export interface PublicShare {
  id: string
  label: string
  allow_comments: boolean
  allow_download: boolean
  expires_at: string | null
  has_password: boolean
}

export interface PublicAsset {
  id: string
  original_filename: string
  mime_type: string
  size: number
  created_at: string
}

export interface ShareComment {
  id: string
  asset_id: string | null
  author_name: string
  author_email: string | null
  body: string
  created_at: string
}

// ---- Projects ----

export interface Project {
  id: string
  workspace_id: string
  name: string
  description: string | null
  color: string | null
  cover_asset_id: string | null
  asset_count: number
  created_at: string
  updated_at: string
}

// ---- Folders ----

export interface Folder {
  id: string
  workspace_id: string
  project_id: string
  parent_id: string | null
  name: string
  slug: string | null
  position: number
  asset_count: number
  children: Folder[]
  created_at: string
}

// ---- Tags ----

export interface Tag {
  id: string
  name: string
  asset_count: number
  color?: string | null
  group_name?: string | null
  created_at: string
  last_used_at?: string | null
}

export interface DuplicateTagPair {
  a: string
  b: string
  score: number
}

export interface MergeTagsResult {
  merged_assets: number
  target: Tag
}

export interface BulkDeleteTagsResult {
  deleted: number
  removed_from_assets: number
}

// ---- Variants ----

export interface Variant {
  id: string
  asset_version_id: string
  type: string
  transform_params: string | null
  size: number | null
  storage_key: string
  download_url: string
  created_at: string
}

export interface ListVariantsResponse {
  variants: Variant[]
  rebuilding: boolean
}

export interface CreateVariantResponse {
  job_id: string
  status: string
  message: string
}

export interface ResizeParams {
  width?: number
  height?: number
  fit?: 'contain' | 'cover' | 'fill'
  quality?: number
  format?: 'jpeg' | 'png' | 'tiff'
}

export interface ConvertParams {
  format: 'jpeg' | 'png' | 'tiff'
  quality?: number
}

export interface CropParams {
  x: number
  y: number
  width: number
  height: number
  quality?: number
  format?: 'jpeg' | 'png'
}

export interface VideoThumbnailParams {
  timestamp?: number
}

export interface TranscodeParams {
  format?: 'mp4' | 'webm'
  resolution?: '1080p' | '720p' | '480p'
  strip_audio?: boolean
}
// ---- Shares ----

export interface Share {
  id: string
  workspace_id: string
  created_by: string
  label: string
  target_type: 'collection' | 'asset' | 'project'
  target_id: string
  has_password: boolean
  expires_at: string | null
  allow_comments: boolean
  allow_download: boolean
  view_count: number
  created_at: string
  revoked_at: string | null
  is_expired: boolean
  public_url: string
}

export interface CreateShareParams {
  label?: string
  target_type: 'collection' | 'asset' | 'project'
  target_id: string
  password?: string
  expires_in_days?: number | null
  allow_comments?: boolean
  allow_download?: boolean
}

export interface UpdateShareParams {
  label?: string
  password?: string
  clear_password?: boolean
  expires_at?: string
  clear_expiry?: boolean
  allow_comments?: boolean
  allow_download?: boolean
}

// ---- Ingress ----

export type IngressSourceType = 'email_api' | 'imap' | 'sftp' | 'dav' | 's3'

export interface IngressSource {
  id: string
  workspace_id: string
  public_token: string
  created_by: string
  type: IngressSourceType
  label: string
  config: Record<string, unknown>
  dest_folder_id: string | null
  dest_project_id: string | null
  enabled: boolean
  poll_interval_min: number
  last_polled_at: string | null
  last_error: string | null
  created_at: string
  updated_at: string
}

export interface IngressRule {
  id: string
  source_id: string
  position: number
  field: string
  operator: string
  value: string
  action: string
}

export interface IngressLogEntry {
  id: string
  source_id: string
  remote_id: string
  filename: string
  asset_id: string | null
  status: 'pending' | 'imported' | 'skipped' | 'failed'
  error: string | null
  imported_at: string
}

export interface CreateIngressSourceParams {
  type: IngressSourceType
  label: string
  config: Record<string, unknown>
  dest_project_id?: string | null
  dest_folder_id?: string | null
  poll_interval_min?: number
  enabled?: boolean
}

export interface UpdateIngressSourceParams {
  label?: string
  config?: Record<string, unknown>
  dest_project_id?: string | null
  dest_folder_id?: string | null
  poll_interval_min?: number
  enabled?: boolean
}

export interface CreateIngressRuleParams {
  position: number
  field: string
  operator: string
  value: string
  action: string
}

// ---- Custom Fields ----

export type FieldType = 'text' | 'number' | 'date' | 'boolean' | 'select' | 'url'
export type FieldScope = 'asset' | 'project'

export interface FieldDefinition {
  id: string
  workspace_id: string
  scope: FieldScope
  name: string
  key: string
  field_type: FieldType
  options: string | null  // JSON array string for select type
  required: boolean
  position: number
  inherit_from_project: boolean
  created_at: string
  updated_at: string
  deleted_at?: string | null
}

export interface FieldDefinitionStats {
  asset_count: number
  project_count: number
}

export interface AssetFieldValue {
  field_id: string
  key: string
  name: string
  field_type: FieldType
  value: string | number | boolean | null
  definition_deleted: boolean
}

export interface AssetFieldsResponse {
  fields: AssetFieldValue[]
}

export type FieldFilterOp = 'eq' | 'lt' | 'lte' | 'gt' | 'gte' | 'contains' | 'starts_with'

export interface FieldFilter {
  key: string
  op: FieldFilterOp
  value: string
}

export interface ProjectFieldValue {
  field_id: string
  key: string
  name: string
  field_type: FieldType
  value: string | number | boolean | null
  definition_deleted: boolean
}

export interface ProjectFieldsResponse {
  fields: ProjectFieldValue[]
}

// ---- Event Log ----

export interface EventActor {
  type: 'user' | 'system'
  id: string | null
  name: string | null
}

export interface AuditEvent {
  id: string
  event_type: string
  actor: EventActor
  payload: Record<string, unknown>
  created_at: string
  human_readable: string
}

export interface ActivityEvent extends AuditEvent {
  entity_type: 'asset' | 'project'
  entity_id: string
}

export interface AuditLogResponse {
  events: AuditEvent[]
  next_cursor: string | null
  has_more: boolean
}

export interface ActivityFeedResponse {
  events: ActivityEvent[]
  next_cursor: string | null
  has_more: boolean
}
export interface WorkspaceMember {
  user_id: string
  name: string
  email: string
  role: string
  joined_at: string
}

export interface WorkspaceInvite {
  id: string
  email: string
  role: string
  expires_at: string
  created_at: string
}

export interface Collection {
  id: string
  workspace_id: string
  name: string
  description: string
  created_by: string
  asset_count: number
  created_at: string
  updated_at: string
}
