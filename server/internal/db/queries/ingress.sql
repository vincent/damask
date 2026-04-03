-- ============================================================
-- ingress_sources
-- ============================================================

-- name: CreateIngressSource :one
INSERT INTO ingress_sources (
    id, workspace_id, created_by, type, label, config, public_token,
    dest_folder_id, dest_project_id, enabled, poll_interval_min
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetIngressSource :one
SELECT * FROM ingress_sources
WHERE id = ? AND workspace_id = ?;

-- name: ListIngressSources :many
SELECT * FROM ingress_sources
WHERE workspace_id = ?
ORDER BY created_at DESC;

-- name: UpdateIngressSource :one
UPDATE ingress_sources SET
    label             = ?,
    config            = ?,
    dest_folder_id    = ?,
    dest_project_id   = ?,
    enabled           = ?,
    poll_interval_min = ?,
    updated_at        = datetime('now')
WHERE id = ? AND workspace_id = ?
RETURNING *;

-- name: DeleteIngressSource :exec
DELETE FROM ingress_sources WHERE id = ? AND workspace_id = ?;

-- name: MarkIngressSourcePolled :exec
UPDATE ingress_sources
SET last_polled_at = datetime('now'),
    last_error     = ?,
    updated_at     = datetime('now')
WHERE id = ?;

-- name: ListDueIngressSources :many
SELECT * FROM ingress_sources
WHERE enabled = 1
  AND (
      last_polled_at IS NULL
      OR datetime(last_polled_at, '+' || poll_interval_min || ' minutes') <= datetime('now')
  )
ORDER BY last_polled_at ASC
LIMIT 20;

-- name: GetIngressSourceByPublicToken :one
SELECT * FROM ingress_sources WHERE public_token = ?;

-- name: SetWorkspaceIngestToken :exec
UPDATE workspaces SET ingest_token = ? WHERE id = ?;

-- name: GetWorkspaceByIngestToken :one
SELECT * FROM workspaces WHERE ingest_token = ?;

-- ============================================================
-- ingress_log
-- ============================================================

-- name: InsertIngressLogEntry :one
INSERT OR IGNORE INTO ingress_log (id, source_id, remote_id, filename, status)
VALUES (?, ?, ?, ?, 'pending')
RETURNING *;

-- name: GetIngressLogEntry :one
SELECT * FROM ingress_log WHERE id = ?;

-- name: UpdateIngressLogEntry :exec
UPDATE ingress_log
SET status = ?, asset_id = ?, error = ?, imported_at = datetime('now')
WHERE id = ?;

-- name: ListIngressSourceLog :many
SELECT * FROM ingress_log
WHERE source_id = ?
ORDER BY imported_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: ListWorkspaceIngressLog :many
SELECT l.* FROM ingress_log l
JOIN ingress_sources s ON s.id = l.source_id
WHERE s.workspace_id = sqlc.arg('workspace_id')
  AND (sqlc.narg('status') IS NULL OR l.status = sqlc.narg('status'))
ORDER BY l.imported_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: DeleteIngressLogEntry :exec
DELETE FROM ingress_log WHERE id = ?;

-- ============================================================
-- ingress_rules
-- ============================================================

-- name: CreateIngressRule :one
INSERT INTO ingress_rules (id, source_id, position, field, operator, value, action)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: ListIngressRules :many
SELECT * FROM ingress_rules
WHERE source_id = ?
ORDER BY position ASC;

-- name: UpdateIngressRule :one
UPDATE ingress_rules
SET position = ?, field = ?, operator = ?, value = ?, action = ?
WHERE id = ?
RETURNING *;

-- name: GetIngressRule :one
SELECT * FROM ingress_rules WHERE id = ?;

-- name: DeleteIngressRule :exec
DELETE FROM ingress_rules WHERE id = ?;
