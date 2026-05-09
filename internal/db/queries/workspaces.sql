-- name: CreateWorkspace :one
INSERT INTO workspaces (id, name, created_at, updated_at)
VALUES (?, ?, datetime('now'), datetime('now'))
RETURNING *;

-- name: GetWorkspaceByID :one
SELECT * FROM workspaces WHERE id = ? LIMIT 1;

-- name: ListWorkspacesWithRetention :many
SELECT * FROM workspaces WHERE version_retention_count > 0;

-- name: GetWorkspaceByIconAsset :one
SELECT * FROM workspaces WHERE icon_asset_id = ? AND id = ? LIMIT 1;

-- name: UpdateWorkspaceVersionRetention :exec
UPDATE workspaces SET version_retention_count = ?, updated_at = datetime('now') WHERE id = ?;

-- name: UpdateWorkspaceExifSettings :exec
UPDATE workspaces SET exif_keep = ?, exif_keep_gps = ?, updated_at = datetime('now') WHERE id = ?;

-- name: UpdateWorkspaceLockedTaxonomy :exec
UPDATE workspaces SET locked_taxonomy = ?, updated_at = datetime('now') WHERE id = ?;

-- name: CountWorkspaceAssets :one
SELECT COUNT(*) FROM assets WHERE workspace_id = ?;
