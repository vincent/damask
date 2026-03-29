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

