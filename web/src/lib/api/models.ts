
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

// ---- Assets ----

export interface Asset {
  id: string
  workspace_id: string
  project_id: string | null
  original_filename: string
  mime_type: string
  size: number
  width: number | null
  height: number | null
  thumbnail_key: string | null
  metadata: string | null
  tags: string[]
  created_at: string
  updated_at: string
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
}
// ---- Variants ----

export interface Variant {
  id: string
  asset_id: string
  type: string
  transform_params: string | null
  size: number | null
  storage_key: string
  download_url: string
  created_at: string
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