# BaDAM — MVP Technical Roadmap

> **35 checkpoints · 6 phases · 11–13 weeks solo / 6–7 weeks duo**

---

## Overview

| Phase | Title | Timeline | Checkpoints |
|-------|-------|----------|-------------|
| Phase 0 | Foundations & scaffolding | Week 1 | 6 |
| Phase 1 | Auth & multi-tenancy | Week 2 | 5 |
| Phase 2 | Asset ingestion & library | Weeks 3–5 | 9 |
| Phase 3 | Projects, tags & organisation | Weeks 6–7 | 5 |
| Phase 3.5 | Folder system | Week 8 | 3 |
| Phase 4 | Variants & transformations | Weeks 9–11 | 7 |

---

## Phase 0 — Foundations & scaffolding - DONE
> Week 1 · 6 checkpoints

### 0.1 — Monorepo & tooling setup
**Tags:** `infra`

Init Git repo with `/server` (Go) and `/web` (SvelteKit) workspaces. Configure `.editorconfig`, `Makefile` with `dev`/`build`/`test` targets, `golangci-lint`, ESLint + Prettier for Svelte. Add `.env.example` and a root README skeleton.

---

### 0.2 — Go project skeleton
**Tags:** `go` `infra`

`go mod init` with module name. Establish `/cmd/server` entry point, `/internal/{api,storage,db,transform,queue,config}` package layout. Wire a minimal fiber HTTP server that returns `200` on `/healthz`. Confirm it compiles to a single static binary.

---

### 0.3 — SvelteKit project skeleton
**Tags:** `svelte` `infra`

`npm create svelte@latest` with TypeScript, ESLint, Prettier. Establish `/lib/api` (typed fetch client), `/lib/stores`, and `/routes` layout. Confirm dev server starts and HMR works.

---

### 0.4 — Database layer — SQLite + sqlc
**Tags:** `db` `go`

Add `go-sqlite3` (CGO-free alternative: `modernc.org/sqlite`). Write initial SQL schema: `workspaces`, `users`, `projects`, `assets`, `tags`, `asset_tags`, `variants`, `jobs`. Run `sqlc generate` to produce typed query functions. All tables carry `workspace_id` for multi-tenancy.

---

### 0.5 — Migration system
**Tags:** `db` `go`

Integrate `golang-migrate` with embedded SQL files. Write a `Migrate()` call at server startup. Document how to add future migrations. Test up/down on a fresh DB.

---

### 0.6 — Config & env management
**Tags:** `go` `infra`

Load config from env vars with sane defaults (`PORT`, `DB_PATH`, `STORAGE_PATH`, `JWT_SECRET`). Define a `Config` struct passed through the app via dependency injection — no globals.

---

## Phase 1 — Auth & multi-tenancy - DONE
> Week 2 · 5 checkpoints

### 1.1 — Paseto token auth
**Tags:** `auth` `go`

Implement `POST /auth/register` and `POST /auth/login`. Hash passwords with bcrypt. Issue Paseto v4 local tokens containing `user_id` and `workspace_id`. Token expiry 7d, refresh via `POST /auth/refresh`.

---

### 1.2 — Auth middleware
**Tags:** `auth` `go`

Write a fiber middleware that validates the Paseto token on every protected route and injects the claims into the request context. All DB queries downstream automatically scope to `workspace_id` from context.

---

### 1.3 — Workspace provisioning
**Tags:** `auth` `go` `db`

On first register, auto-create a workspace and assign the user as owner. Add `GET /workspace/me` endpoint. Implement workspace invite: `POST /workspace/invites` → generates a short-lived token; `POST /workspace/invites/accept` → creates the user and links them to the workspace.

---

### 1.4 — Frontend auth flow
**Tags:** `svelte` `auth`

Build `/login` and `/register` pages in SvelteKit. Persist the Paseto token in an httpOnly cookie via a thin `+server.ts` proxy (avoids localStorage). Add an auth store and a route guard (`+layout.ts` load function that redirects to `/login` when unauthenticated).

---

### 1.5 — User roles (minimal)
**Tags:** `auth` `go` `db`

Add a `role` column to `workspace_members`: `owner`, `editor`, `viewer`. Enforce in middleware: DELETE endpoints require `owner`, POST/PUT require `editor`. No UI for role management yet — just backend enforcement.

---

## Phase 2 — Asset ingestion & library - DONE
> Weeks 3–5 · 9 checkpoints

### 2.1 — Storage abstraction layer
**Tags:** `go` `infra`

Define a `Storage` interface: `Put(key, reader)`, `Get(key) reader`, `Delete(key)`, `List(prefix) []string`. Implement `LocalStorage` backed by the filesystem. Asset keys follow the pattern `workspace_id/asset_id/filename`. No S3 yet — just make the interface solid.

---

### 2.2 — Asset upload endpoint
**Tags:** `go` `core`

`POST /assets` — accept `multipart/form-data`. Extract file, detect MIME type (`mime.TypeByExtension` + magic bytes fallback). Write to Storage. Insert asset row: `id` (UUID), `workspace_id`, `project_id` (nullable), `original_filename`, `storage_key`, `mime_type`, `size`, `width`, `height` (for images). Return the asset object.

---

### 2.3 — Thumbnail generation on ingest
**Tags:** `go` `media`

After upload, enqueue a job to generate a 400×400 WebP thumbnail using `github.com/disintegration/imaging` or a shell call to ffmpeg (for video). Store thumbnail at `workspace_id/asset_id/thumb.webp`. Update asset row with `thumbnail_key`. This must be async — upload returns immediately.

---

### 2.4 — Asset metadata extraction
**Tags:** `go` `media` `core`

On ingest, extract: image dimensions (`image.Decode`), video duration + dimensions (`ffprobe` JSON), EXIF data for photos (strip GPS by default, privacy-first). Persist in an `asset_metadata` JSONB column.

---

### 2.5 — Asset list & detail endpoints
**Tags:** `go` `core`

- `GET /assets` — paginated (cursor-based, not offset), filterable by `project_id`, `tag`, `mime_type` prefix, `created_at` range.
- `GET /assets/:id` — full asset object + tags + variants.
- `GET /assets/:id/file` — stream the original file.
- `GET /assets/:id/thumb` — stream the thumbnail.

---

### 2.6 — Library grid view
**Tags:** `svelte` `core`

Build `/library` route in SvelteKit. Virtual-scrolled grid of asset cards (thumbnail + name + type badge). Infinite scroll via `IntersectionObserver` hitting the cursor-paginated API. Skeleton loaders while fetching. Click opens lightbox.

---

### 2.7 — Drag-and-drop upload UI
**Tags:** `svelte` `core`

Full-page drop zone with upload queue. Show per-file progress bars (XHR with `onprogress`). Handle multiple files. After upload, optimistically insert card into the grid with a 'processing' overlay until the thumbnail job completes (poll `GET /assets/:id` every 2s or use SSE).

---

### 2.8 — Lightbox / asset detail panel
**Tags:** `svelte` `core`

Slide-in panel (not a new route) showing full preview (image, video player, PDF first-page render, generic icon for others), metadata table, tags, project badge, variants list. Action buttons: download original, delete, create variant.

---

### 2.9 — Search
**Tags:** `go` `svelte` `db` `core`

`GET /assets?q=` — full-text search over asset name, tags, project name using SQLite FTS5 virtual table. Debounced search input in the header updates the library grid reactively. No external search engine needed for MVP.

---

## Phase 3 — Projects, tags & organisation - DONE
> Weeks 6–7 · 5 checkpoints

### 3.1 — Projects CRUD
**Tags:** `go` `db` `core`

`POST /projects`, `GET /projects`, `GET /projects/:id`, `PUT /projects/:id`, `DELETE /projects/:id`. Projects have: `name`, `description`, `cover_asset_id` (nullable), `color` (hex string for sidebar indicator). DELETE cascades to asset `project_id → null` (assets are not deleted).

---

### 3.2 — Tag system
**Tags:** `go` `db` `core`

Tags are workspace-scoped strings, auto-created on first use. `POST /assets/:id/tags {name}`, `DELETE /assets/:id/tags/:name`. `GET /tags` — list all workspace tags with asset counts. Enforce max 20 tags per asset client-side. SQLite FTS5 index covers tag names.

---

### 3.3 — Project sidebar & navigation
**Tags:** `svelte` `core`

Left sidebar listing projects with color dot and asset count. Click filters the library to that project. 'All assets' default view. Add/rename/delete project from sidebar context menu. Keyboard shortcut (`Cmd+K`) opens a command palette to jump between projects.

---

### 3.4 — Tag filter UI
**Tags:** `svelte` `core`

Tag filter bar below the search input: click a tag to filter, Shift-click to multi-select (AND logic). Active tags shown as dismissible chips. Tags auto-suggested in the asset detail panel with typeahead.

---

### 3.4.5 — Bulk actions
**Tags:** `go` `svelte` `core`

Shift-click or checkbox to multi-select assets in the grid. Bulk action bar appears: assign to project, add tag, delete. Batch API endpoints: `POST /assets/bulk/tag`, `POST /assets/bulk/project`, `DELETE /assets/bulk`.

---

## Phase 3.5 — Folder system
> Week 8 · 3 checkpoints

> **Design decision:** folders are modelled with an adjacency list in SQL (each folder holds a nullable `parent_id`), which supports infinite depth natively. The application layer enforces a **2-level maximum** for MVP (root folders + one level of subfolders). Lifting this limit later is a single-line validation change — no schema migration needed.

### 3.5.1 — Folders table & API
**Tags:** `go` `db` `core`

Add the `folders` table:

```sql
CREATE TABLE folders (
  id           TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id),
  project_id   TEXT NOT NULL REFERENCES projects(id),
  parent_id    TEXT REFERENCES folders(id),   -- NULL = root folder
  name         TEXT NOT NULL,
  position     INTEGER NOT NULL DEFAULT 0,    -- manual ordering
  created_at   TEXT NOT NULL DEFAULT (datetime('now')),
  UNIQUE(project_id, parent_id, name)
);
```

Add `folder_id TEXT REFERENCES folders(id)` to the `assets` table (nullable — assets without a folder sit at project root).

Implement endpoints:
- `POST /projects/:id/folders` — create folder, enforce max depth 2 by walking `parent_id` chain before insert
- `GET /projects/:id/folders` — return full tree via recursive CTE (see below), max depth guard in query
- `PUT /folders/:id` — rename or reorder (`name`, `position`, `parent_id`)
- `DELETE /folders/:id` — moves all child assets to `folder_id = NULL` (project root), then deletes folder and its children recursively

Recursive CTE for tree fetch:

```sql
WITH RECURSIVE tree AS (
  SELECT *, 0 AS depth FROM folders
  WHERE project_id = ? AND parent_id IS NULL

  UNION ALL

  SELECT f.*, t.depth + 1 FROM folders f
  JOIN tree t ON f.parent_id = t.id
  WHERE t.depth < 2   -- depth limit lives here; bump to lift restriction
)
SELECT * FROM tree ORDER BY depth, position;
```

Update `GET /assets` to accept `folder_id` as a filter param. Assets at project root are returned when `folder_id=root` is passed (i.e. `WHERE folder_id IS NULL AND project_id = ?`).

---

### 3.5.2 — Folder tree UI (sidebar)
**Tags:** `svelte` `core`

Extend the project sidebar to render a collapsible folder tree beneath each project entry. Each project node expands to show its root folders; each root folder expands to show its one level of subfolders. UI behaviours:

- Click a folder → filters the library grid to that folder's assets
- Chevron toggle → expand/collapse without navigating
- Right-click context menu → rename, delete, create subfolder (disabled at depth 2)
- Folder shows asset count badge
- 'All assets in project' option at the project level bypasses folder filter

Folder state (open/closed) is persisted in a Svelte store and survives page refresh via `localStorage`.

---

### 3.5.3 — Drag assets into folders
**Tags:** `svelte` `core`

Make asset cards in the library grid draggable (`draggable="true"` + HTML5 drag events). Folder entries in the sidebar act as drop targets — highlight on `dragover`, commit on `drop` via `PATCH /assets/:id {folder_id}`. Show a toast confirmation on success. Also support drag-to-root by making the project name row a drop target that sets `folder_id = null`.

Bulk move: when multiple assets are selected and one is dragged to a folder, all selected assets move together.

---

## Phase 4 — Variants & transformations - DONE
> Weeks 9–11 · 7 checkpoints

### 4.1 — Job queue (in-process)
**Tags:** `go` `infra`

Implement a simple in-process job queue: a buffered channel of `Job` structs, a configurable worker pool (default 4 goroutines), SQLite-backed persistence (`jobs` table: `id`, `type`, `payload JSON`, `status`, `attempts`, `error`, `created_at`, `updated_at`). On startup, re-queue any jobs stuck in `processing` from a previous crash.

---

### 4.2 — Image resize & format conversion
**Tags:** `go` `media`

Transform worker: use `github.com/disintegration/imaging` for resize/crop/rotate. Support output formats: JPEG, PNG, WebP, AVIF (via cgo libavif or ffmpeg). Accept params: `width`, `height`, `fit` (cover/contain/fill), `quality`, `format`. Store result via Storage, insert variant row with `transform_params` JSON.

---

### 4.3 — Video thumbnail extraction
**Tags:** `go` `media`

`ffmpeg -ss {timestamp} -frames:v 1` to extract a frame as JPEG/WebP. Accept params: `timestamp` (seconds or percent). Run ffmpeg as `exec.Command` with a timeout. Store result as a variant.

---

### 4.4 — Video transcoding
**Tags:** `go` `media`

`ffmpeg -c:v libx264 -c:a aac -movflags +faststart` for MP4. Support: format conversion (MOV→MP4, WebM), resolution downscale (1080p→720p→480p), strip audio. This is the heaviest job — enforce a max concurrent transcoding limit of 2.

---

### 4.5 — Background removal
**Tags:** `go` `media`

Integrate Remove.bg API as optional transformation (requires API key in settings). `POST /assets/:id/variants {type:'bg-remove'}`. Fall back gracefully if key is not configured: show 'configure API key' prompt. Store the result PNG as a variant. Later: swap for local RMBG-1.4 model.

---

### 4.6 — Variant management API
**Tags:** `go` `core`

- `GET /assets/:id/variants` — list all variants with type, params, size, created_at, download URL.
- `DELETE /assets/:id/variants/:vid`
- `POST /assets/:id/variants` — create new variant with `{type, params}`.

Variant types: `resize`, `convert`, `thumbnail`, `bg_remove`, `crop`.

---

### 4.7 — Variant creation UI
**Tags:** `svelte` `media`

'Create variant' panel in the asset lightbox. Tabbed by type:
- **Resize** — width/height inputs + fit toggle
- **Convert** — format selector + quality slider
- **Crop** — interactive crop box overlay on a canvas preview
- **Background remove** — one-click, shows spinner

Shows live preview when params change (debounced, calls a preview endpoint that returns a small WebP).

---

### 4.8 — Transform preview endpoint
**Tags:** `go` `media`

`GET /assets/:id/preview?w=&h=&fit=&format=&q=` — runs the transform in-memory (no storage write) and streams back a small WebP ≤800px. Used by the variant UI for live preview. Cache in memory (LRU, max 100 entries) to avoid redundant work while the user is dragging sliders.

---

### Phase 5 - Design

### 5.1 — Apply basic mockups from MP - DONE

### 5.2 — Add status bar - DONE

### 5.3 — Add content zoom - DONE

---

### Phase 8 — More Transforms - DONE
**Tags:** `go` `media`

Add a Watermark variant

---

### Phase 50 - Ingress

### 50.1 - Manual variant
**Tags:** `go` `media`

Allow to manually upload a file as a new variant.

---





## Architecture reference

```
badam/
├── server/              ← Go binary (API + file server + job runner)
│   ├── cmd/server/      ← Entry point
│   ├── internal/
│   │   ├── api/         ← Fiber route handlers
│   │   ├── storage/     ← LocalStorage + Storage interface
│   │   ├── transform/   ← ffmpeg + imaging pipeline
│   │   ├── queue/       ← In-process job queue
│   │   ├── db/          ← sqlc generated queries + migrations
│   │   └── config/      ← Env-based config struct
│
├── web/                 ← SvelteKit app
│   ├── src/
│   │   ├── lib/
│   │   │   ├── api/     ← Typed fetch client
│   │   │   └── stores/  ← Asset state, upload progress, auth
│   │   └── routes/
│   │       ├── library/ ← Main grid view
│   │       ├── project/ ← Project detail
│   │       └── asset/   ← Lightbox + variants panel
│
└── desktop/             ← Tauri shell (post-MVP)
```

## Key dependencies

| Layer | Package | Purpose |
|-------|---------|---------|
| HTTP | `github.com/gofiber/fiber/v2` | HTTP server |
| DB driver | `modernc.org/sqlite` | CGO-free SQLite |
| Query gen | `github.com/sqlc-dev/sqlc` | Type-safe SQL |
| Migrations | `github.com/golang-migrate/migrate` | Schema versioning |
| Auth | `github.com/o1ecc8b9/go-paseto` | Paseto v4 tokens |
| Imaging | `github.com/disintegration/imaging` | Image resize/crop |
| EXIF | `github.com/rwcarlsen/goexif` | Metadata extraction |
| Frontend | `SvelteKit` + `TypeScript` | UI framework |
| UI base | `shadcn-svelte` | Component library |

## Design decisions

- **Local-first, remote-optional** — the `Storage` interface abstracts filesystem vs S3. Swap at config level, no code changes.
- **Multi-tenancy from day 1** — `workspace_id` on every table, enforced in middleware before any handler runs.
- **Async transforms always** — even thumbnail generation goes through the job queue. Uploads are always fast.
- **Cursor pagination only** — offset pagination breaks with large datasets and concurrent uploads. Cursors from day 1.
- **Single binary** — the Go server compiles to one executable. Zero runtime dependencies for self-hosters.
- **SQLite in MVP, Postgres-ready** — sqlc queries are portable. Turso (libSQL) bridges local→cloud without schema changes.
- **No Redis, no message broker** — the in-process queue backed by SQLite handles MVP load. Interface is swappable for Asynq or River when needed.

## Post-MVP expansion hooks

| Feature | What's already in place |
|---------|------------------------|
| S3 / R2 remote sync | `Storage` interface — add `S3Storage` implementation |
| Tauri desktop app | Go server can be embedded as a sidecar |
| AI auto-tagging | Job queue + variant system — add a new job type |
| Subtitle generation | Transform pipeline — add ffmpeg subtitle extraction |
| Version history | `variants` table — extend with `parent_asset_id` |
| Local bg removal | Replace Remove.bg call with RMBG-1.4 inference |
| Figma / Adobe plugin | REST API is already public — build the plugin against it |
| Team billing | `workspace_members` + role system already exists |
| Deeper folder nesting | Change `WHERE t.depth < 2` to desired limit in the recursive CTE |

Sharing v2 — password protection + expiry + client review mode. Ships fast, drives word of mouth, zero infrastructure changes.
S3/R2 storage backend — unlocks teams and remote access, uses existing interface.
Desktop app (Tauri) — makes the local-first promise real, strong launch moment.
2–3 targeted transforms — Whisper subtitles, watermark, smart crop.
Workflows — revisit only when users are asking for it by name.

