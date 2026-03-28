-- name: CreateAsset :one
INSERT INTO assets (id, workspace_id, project_id, original_filename, storage_key, mime_type, size, width, height, metadata)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetAssetByID :one
SELECT * FROM assets WHERE id = ? AND workspace_id = ?;

-- name: ListAssets :many
SELECT * FROM assets
WHERE workspace_id = sqlc.arg('workspace_id')
  AND (sqlc.narg('project_id') IS NULL OR project_id = sqlc.narg('project_id'))
  AND (sqlc.narg('mime_prefix') IS NULL OR mime_type LIKE sqlc.narg('mime_prefix'))
  AND (
    sqlc.narg('cursor_at') IS NULL
    OR created_at < sqlc.narg('cursor_at')
    OR (created_at = sqlc.narg('cursor_at') AND id < sqlc.narg('cursor_id'))
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit');

-- name: UpdateAssetThumbnail :exec
UPDATE assets SET thumbnail_key = ?, updated_at = datetime('now') WHERE id = ?;

-- name: UpdateAssetDimensions :exec
UPDATE assets SET width = ?, height = ?, metadata = ?, updated_at = datetime('now') WHERE id = ?;

-- name: UpdateAssetProject :exec
UPDATE assets SET project_id = ?, updated_at = datetime('now') WHERE id = ? AND workspace_id = ?;

-- name: UpdateAssetFolder :exec
UPDATE assets SET folder_id = ?, updated_at = datetime('now') WHERE id = ? AND workspace_id = ?;

-- name: DeleteAsset :exec
DELETE FROM assets WHERE id = ? AND workspace_id = ?;
