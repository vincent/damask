-- name: CreateVariant :one
INSERT INTO variants (id, asset_id, workspace_id, type, storage_key, transform_params, size)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: ListVariants :many
SELECT * FROM variants WHERE asset_id = ? AND workspace_id = ? ORDER BY created_at DESC;

-- name: GetVariantByID :one
SELECT * FROM variants WHERE id = ? AND workspace_id = ?;

-- name: DeleteVariant :exec
DELETE FROM variants WHERE id = ? AND workspace_id = ?;

-- name: DeleteVariantsByAsset :exec
DELETE FROM variants WHERE asset_id = ? AND workspace_id = ?;
