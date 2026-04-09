-- name: CreateMember :exec
INSERT INTO workspace_members (workspace_id, user_id, role, invited_by, created_at)
VALUES (?, ?, ?, ?, datetime('now'));

-- name: GetMember :one
SELECT * FROM workspace_members
WHERE workspace_id = ? AND user_id = ?
LIMIT 1;

-- name: GetMemberByUserID :one
SELECT * FROM workspace_members
WHERE user_id = ?
ORDER BY created_at ASC
LIMIT 1;

-- name: ListWorkspacesByUserID :many
SELECT w.id, w.name, w.created_at, w.updated_at, wm.role
FROM workspaces w
JOIN workspace_members wm ON wm.workspace_id = w.id
WHERE wm.user_id = ?
ORDER BY wm.created_at ASC;

