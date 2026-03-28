-- name: CreateUser :one
INSERT INTO users (id, workspace_id, email, password_hash, name, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = ? LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ? LIMIT 1;
