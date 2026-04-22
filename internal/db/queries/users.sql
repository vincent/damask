-- name: CreateUser :one
INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = ? LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ? LIMIT 1;

-- name: GetUserByOIDC :one
SELECT * FROM users WHERE oidc_issuer = ? AND oidc_sub = ? LIMIT 1;

-- name: GetUserByGoogleID :one
SELECT * FROM users WHERE google_user_id = ? LIMIT 1;

-- name: GetUserByCanvaID :one
SELECT * FROM users WHERE canva_user_id = ? LIMIT 1;

-- name: LinkOIDC :one
UPDATE users SET oidc_issuer = ?, oidc_sub = ?, avatar_url = COALESCE(?, avatar_url),
    auth_methods = ?, updated_at = datetime('now')
WHERE id = ? RETURNING *;

-- name: LinkGoogle :one
UPDATE users SET google_user_id = ?, avatar_url = COALESCE(?, avatar_url),
    auth_methods = ?, updated_at = datetime('now')
WHERE id = ? RETURNING *;

-- name: LinkCanva :one
UPDATE users SET canva_user_id = ?, avatar_url = COALESCE(?, avatar_url),
    auth_methods = ?, updated_at = datetime('now')
WHERE id = ? RETURNING *;

-- name: UnlinkOIDC :one
UPDATE users SET oidc_issuer = NULL, oidc_sub = NULL,
    auth_methods = ?, updated_at = datetime('now')
WHERE id = ? RETURNING *;

-- name: UnlinkGoogle :one
UPDATE users SET google_user_id = NULL,
    auth_methods = ?, updated_at = datetime('now')
WHERE id = ? RETURNING *;

-- name: UnlinkCanva :one
UPDATE users SET canva_user_id = NULL,
    auth_methods = ?, updated_at = datetime('now')
WHERE id = ? RETURNING *;

-- name: CreateUserWithOIDC :one
INSERT INTO users (id, email, password_hash, name, oidc_issuer, oidc_sub, avatar_url, auth_methods, created_at, updated_at)
VALUES (?, ?, '', ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
RETURNING *;

-- name: CreateUserWithGoogle :one
INSERT INTO users (id, email, password_hash, name, google_user_id, avatar_url, auth_methods, created_at, updated_at)
VALUES (?, ?, '', ?, ?, ?, ?, datetime('now'), datetime('now'))
RETURNING *;

-- name: CreateUserWithCanva :one
INSERT INTO users (id, email, password_hash, name, canva_user_id, avatar_url, auth_methods, created_at, updated_at)
VALUES (?, ?, '', ?, ?, ?, ?, datetime('now'), datetime('now'))
RETURNING *;
