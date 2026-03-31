# Sharing v2 — Technical Roadmap for Claude Code

## Context

Damask DAM. Stack: Go (Fiber) backend, SvelteKit frontend, SQLite (sqlc), local filesystem storage behind a `Storage` interface. Auth uses Paseto v4 tokens. All DB rows are scoped by `workspace_id`.

This feature adds three sharing capabilities, in order of implementation:
1. Password-protected share links
2. Expiring share links
3. Client review mode (view + comment, no account required)

---

## Database migrations

### Migration 001 — shares table

```sql
CREATE TABLE shares (
  id              TEXT PRIMARY KEY,           -- UUID, also used as the public token
  workspace_id    TEXT NOT NULL REFERENCES workspaces(id),
  created_by      TEXT NOT NULL REFERENCES users(id),
  label           TEXT NOT NULL DEFAULT '',   -- optional human name ("Nike Q3 delivery")
  target_type     TEXT NOT NULL,              -- 'collection' | 'asset' | 'project'
  target_id       TEXT NOT NULL,              -- id of the collection, asset, or project
  password_hash   TEXT,                       -- bcrypt hash, NULL = no password
  expires_at      TEXT,                       -- ISO datetime, NULL = never
  allow_comments  INTEGER NOT NULL DEFAULT 0, -- 1 = client review mode enabled
  allow_download  INTEGER NOT NULL DEFAULT 1,
  view_count      INTEGER NOT NULL DEFAULT 0,
  created_at      TEXT NOT NULL DEFAULT (datetime('now')),
  revoked_at      TEXT                        -- NULL = active
);

CREATE INDEX idx_shares_workspace ON shares(workspace_id);
CREATE INDEX idx_shares_target ON shares(target_type, target_id);
```

### Migration 002 — share_comments table

```sql
CREATE TABLE share_comments (
  id           TEXT PRIMARY KEY,
  share_id     TEXT NOT NULL REFERENCES shares(id) ON DELETE CASCADE,
  asset_id     TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
  author_name  TEXT NOT NULL,   -- free text, no account required
  author_email TEXT,            -- optional, for notification hooks later
  body         TEXT NOT NULL,
  created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_comments_share ON share_comments(share_id);
CREATE INDEX idx_comments_asset ON share_comments(asset_id);
```

---

## Backend

### S-1 — sqlc queries

Add sqlc queries for:
- `CreateShare(params) Share`
- `GetShareByID(id) Share`
- `ListSharesByWorkspace(workspace_id) []Share`
- `UpdateShare(id, params) Share`
- `RevokeShare(id) error` — sets `revoked_at = now()`
- `IncrementShareViewCount(id) error`
- `CreateComment(params) ShareComment`
- `ListCommentsByShare(share_id) []ShareComment`
- `ListCommentsByAsset(share_id, asset_id) []ShareComment`

Re-run `sqlc generate` after adding queries.

---

### S-2 — share creation endpoint

`POST /shares` — authenticated, workspace-scoped.

Request body:
```json
{
  "label": "Nike Q3 delivery",
  "target_type": "project",
  "target_id": "proj_abc123",
  "password": "secret123",       // optional, plain — hashed server-side
  "expires_in_days": 14,         // optional, null = never
  "allow_comments": true,
  "allow_download": true
}
```

Logic:
- Validate `target_type` is one of `collection | asset | project`
- Validate `target_id` belongs to the caller's `workspace_id`
- If `password` provided: bcrypt hash it (cost 10), store in `password_hash`
- If `expires_in_days` provided: set `expires_at = now() + N days`
- Generate UUID for `id` — this UUID **is** the public token (no separate token field needed)
- Return full share object including the public URL: `{base_url}/s/{share.id}`

---

### S-3 — share management endpoints

All authenticated, workspace-scoped.

- `GET /shares` — list all shares for the workspace, include `is_expired` computed field
- `GET /shares/:id` — single share detail
- `PUT /shares/:id` — update `label`, `password`, `expires_at`, `allow_comments`, `allow_download`
- `DELETE /shares/:id` — sets `revoked_at`, does not hard-delete (preserves audit trail)

---

### S-4 — public share access endpoint

`POST /shared/:id/access` — unauthenticated route. Validates the share and issues a short-lived share session.

Request body:
```json
{ "password": "secret123" }  // omit if no password set
```

Logic:
- Load share by `id`
- Return 404 if not found or `revoked_at` is set
- Return 410 Gone if `expires_at` is past
- If `password_hash` is set: bcrypt compare — return 401 if mismatch
- On success: increment `view_count`, return a **share session token** (signed Paseto token, 24h expiry, payload: `{ share_id, target_type, target_id, allow_comments, allow_download }`)

The share session token is stored client-side (cookie or memory) and sent on all subsequent `/shared/` requests.

---

### S-5 — public share content endpoints

All require a valid share session token (separate middleware from the workspace auth middleware).

```
GET /shared/:id/assets          — list assets in the share target
GET /shared/:id/assets/:aid     — single asset detail
GET /shared/:id/assets/:aid/file     — stream file (check allow_download)
GET /shared/:id/assets/:aid/thumb    — stream thumbnail (always allowed)
```

Middleware for these routes:
- Validate share session Paseto token
- Confirm `share_id` in token matches `:id` in path
- Re-check expiry and revocation on every request (share can be revoked mid-session)

---

### S-6 — comment endpoints (public)

Require valid share session token with `allow_comments: true`.

`POST /shared/:id/comments`
```json
{
  "asset_id": "ast_xyz",
  "author_name": "Sarah (Nike)",
  "author_email": "sarah@nike.com",
  "body": "Love this one, can we get a version without the logo?"
}
```

`GET /shared/:id/comments` — all comments on this share, grouped by `asset_id`
`GET /shared/:id/assets/:aid/comments` — comments on a specific asset

---

### S-7 — comment endpoints (authenticated, workspace owner view)

`GET /shares/:id/comments` — all comments received on a share (workspace owner view, includes author info)
`DELETE /shares/:id/comments/:cid` — moderation: delete a comment

---

## Frontend — authenticated side

### S-8 — share management UI

New route: `/settings/shares` (or sidebar section under each project).

Per share, show: label, target, created date, expiry status, view count, comment count, revoke button, copy link button.

"Create share" flow — modal or slide-in panel:
- Target picker (current project / specific asset / all assets in project)
- Password toggle + input
- Expiry picker (7d / 14d / 30d / never / custom)
- Comment toggle
- Download toggle
- On submit: call `POST /shares`, show the generated link with a one-click copy button

Share list shows a status pill: `active` / `expires in N days` / `expired` / `revoked`.

---

### S-9 — share link button in asset lightbox and project header

Add a "Share" button to:
- The asset detail panel (lightbox) → creates an asset-scoped share
- The project header → creates a project-scoped share

On click: open the share creation modal pre-filled with the correct target. If a share already exists for this target, show the existing link with options to copy, edit, or revoke.

---

## Frontend — public share view

This is a separate SvelteKit layout with no sidebar, no auth, no workspace context. Routes under `/s/[shareId]/`.

### S-10 — share landing / password gate

Route: `/s/[shareId]`

On load: call `GET /shared/:id` (lightweight endpoint returning `{ requires_password, expires_at, label, allow_download, allow_comments }` — no assets, no token).

If `requires_password`: show a minimal password form. On submit call `POST /shared/:id/access`, store the returned share session token in a `httpOnly` session cookie via a `+server.ts` proxy. Redirect to `/s/[shareId]/view`.

If no password: call `POST /shared/:id/access` automatically on page load, redirect immediately.

Show a clear expiry notice if `expires_at` is within 3 days.

---

### S-11 — share gallery view

Route: `/s/[shareId]/view`

Grid of asset thumbnails, same virtual-scroll pattern as the main library. No sidebar, no tags, no project navigation. Header shows the share `label` and a download-all button (if `allow_download`).

Click an asset → opens the asset review panel (see S-12).

Branding: show "Powered by Damask" in the footer (small, unobtrusive). This is the organic acquisition touchpoint.

---

### S-12 — asset review panel (client review mode)

Slide-in panel on asset click. Shows:
- Full asset preview (image, video player)
- Download button (if `allow_download`)
- Comment thread for this asset (if `allow_comments`)

Comment thread:
- List of existing comments (author name + timestamp + body)
- "Leave a comment" form: name (required), email (optional), message (required)
- On submit: `POST /shared/:id/comments`, optimistically append to thread
- No account, no login — name field is the identity

If `allow_comments` is false, comment section is hidden entirely.

---

## Checklist summary

### Database
- [x] Migration 001: `shares` table
- [x] Migration 002: `share_comments` table

### Backend
- [x] S-1: sqlc queries for shares and comments
- [x] S-2: `POST /shares` — create share
- [x] S-3: `GET|PUT|DELETE /shares/:id` — manage shares
- [x] S-4: `POST /shared/:id/access` — public access + share session token
- [x] S-5: `GET /shared/:id/assets` + file/thumb streaming — public content
- [x] S-6: `POST|GET /shared/:id/comments` — public comments
- [x] S-7: `GET|DELETE /shares/:id/comments` — owner moderation

### Frontend (authenticated)
- [x] S-8: `/settings/shares` management page
- [x] S-9: Share button in asset lightbox + project header

### Frontend (public)
- [x] S-10: `/s/[shareId]` — password gate + auto-redirect
- [x] S-11: `/s/[shareId]/view` — public gallery
- [x] S-12: Asset review panel with comment thread

---

## Key implementation notes for Claude Code

- The share `id` (UUID) doubles as the public token — no separate signing needed for the URL itself. The Paseto share session token is what protects content endpoints after access is granted.
- All `/s/` public routes use a **separate middleware** from the workspace Paseto middleware. Never mix the two — a share session token must never grant workspace access.
- `revoked_at` is a soft delete. Never hard-delete shares — the URL may have been sent to a client and you want `410 Gone` rather than `404 Not Found` on revoked links.
- Re-check expiry/revocation **on every request** to `/shared/` endpoints, not just at session creation. A share can be revoked while a client is actively browsing.
- The public share view (`/s/`) should be a completely separate SvelteKit layout with `+layout.svelte` that has no sidebar, no auth store dependency, no workspace context. Treat it as a separate mini-app within the same codebase.
- Password hashing: bcrypt cost 10 is fine. Do not store plain passwords anywhere, including logs.
- `allow_download: false` means the file streaming endpoint returns 403 — but thumbnails always stream (you can't review assets you can't see).