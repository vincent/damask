-- name: CreateWorkspace :one
INSERT INTO workspaces (id, name, created_at, updated_at)
VALUES (?, ?, datetime('now'), datetime('now'))
RETURNING *;

-- name: GetWorkspaceByID :one
SELECT * FROM workspaces WHERE id = ? LIMIT 1;
