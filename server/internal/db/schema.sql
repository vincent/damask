CREATE TABLE workspaces (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    ingest_token TEXT UNIQUE,
    created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE users (
    id            TEXT PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    name          TEXT NOT NULL,
    created_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at    DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE workspace_members (
    workspace_id TEXT NOT NULL REFERENCES workspaces(id),
    user_id      TEXT NOT NULL REFERENCES users(id),
    role         TEXT NOT NULL CHECK(role IN ('owner', 'editor', 'viewer')),
    invited_by   TEXT REFERENCES users(id),
    created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (workspace_id, user_id)
);

CREATE TABLE workspace_invites (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id),
    email        TEXT NOT NULL,
    token        TEXT NOT NULL UNIQUE,
    role         TEXT NOT NULL CHECK(role IN ('editor', 'viewer')) DEFAULT 'editor',
    invited_by   TEXT NOT NULL REFERENCES users(id),
    expires_at   DATETIME NOT NULL,
    accepted_at  DATETIME,
    created_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE projects (
    id            TEXT PRIMARY KEY,
    workspace_id  TEXT NOT NULL REFERENCES workspaces(id),
    name          TEXT NOT NULL,
    description   TEXT,
    color         TEXT,
    cover_asset_id TEXT,
    created_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at    DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE folders (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id),
    project_id   TEXT NOT NULL REFERENCES projects(id),
    parent_id    TEXT REFERENCES folders(id),
    name         TEXT NOT NULL,
    position     INTEGER NOT NULL DEFAULT 0,
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(project_id, parent_id, name)
);

CREATE TABLE assets (
    id                TEXT PRIMARY KEY,
    workspace_id      TEXT NOT NULL REFERENCES workspaces(id),
    project_id        TEXT REFERENCES projects(id),
    folder_id         TEXT REFERENCES folders(id),
    original_filename TEXT NOT NULL,
    storage_key       TEXT NOT NULL,
    mime_type         TEXT NOT NULL,
    size              INTEGER NOT NULL,
    width             INTEGER,
    height            INTEGER,
    thumbnail_key     TEXT,
    metadata          TEXT, -- JSON
    created_at        DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at        DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE tags (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id),
    name         TEXT NOT NULL,
    UNIQUE(workspace_id, name)
);

CREATE TABLE asset_tags (
    asset_id TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    tag_id   TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (asset_id, tag_id)
);

CREATE TABLE variants (
    id               TEXT PRIMARY KEY,
    asset_id         TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    workspace_id     TEXT NOT NULL REFERENCES workspaces(id),
    type             TEXT NOT NULL,
    storage_key      TEXT NOT NULL,
    transform_params TEXT, -- JSON
    size             INTEGER,
    created_at       DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE jobs (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id),
    type         TEXT NOT NULL,
    payload      TEXT NOT NULL, -- JSON
    status       TEXT NOT NULL CHECK(status IN ('pending', 'processing', 'done', 'failed')) DEFAULT 'pending',
    attempts     INTEGER NOT NULL DEFAULT 0,
    error        TEXT,
    created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Indexes for common queries
CREATE INDEX idx_assets_workspace ON assets(workspace_id);
CREATE INDEX idx_assets_project ON assets(project_id);
CREATE INDEX idx_folders_project ON folders(project_id);
CREATE INDEX idx_assets_folder ON assets(folder_id);
CREATE INDEX idx_tags_workspace ON tags(workspace_id);
CREATE INDEX idx_variants_asset ON variants(asset_id);
CREATE INDEX idx_jobs_status ON jobs(status);

-- FTS5 virtual table for asset search (migration 000002)
CREATE VIRTUAL TABLE IF NOT EXISTS assets_fts USING fts5(
    original_filename,
    content='assets',
    content_rowid='rowid'
);

-- Shares (migration 000005)
CREATE TABLE shares (
  id              TEXT PRIMARY KEY,
  workspace_id    TEXT NOT NULL REFERENCES workspaces(id),
  created_by      TEXT NOT NULL REFERENCES users(id),
  label           TEXT NOT NULL DEFAULT '',
  target_type     TEXT NOT NULL CHECK(target_type IN ('collection', 'asset', 'project')),
  target_id       TEXT NOT NULL,
  password_hash   TEXT,
  expires_at      TEXT,
  allow_comments  INTEGER NOT NULL DEFAULT 0,
  allow_download  INTEGER NOT NULL DEFAULT 1,
  view_count      INTEGER NOT NULL DEFAULT 0,
  created_at      TEXT NOT NULL DEFAULT (datetime('now')),
  revoked_at      TEXT
);

CREATE INDEX idx_shares_workspace ON shares(workspace_id);
CREATE INDEX idx_shares_target ON shares(target_type, target_id);

CREATE TABLE share_comments (
  id           TEXT PRIMARY KEY,
  share_id     TEXT NOT NULL REFERENCES shares(id) ON DELETE CASCADE,
  asset_id     TEXT NOT NULL,
  author_name  TEXT NOT NULL,
  author_email TEXT,
  body         TEXT NOT NULL,
  created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_comments_share ON share_comments(share_id);
CREATE INDEX idx_comments_asset ON share_comments(asset_id);

-- Ingress (migration 000006)
CREATE TABLE ingress_sources (
    id                TEXT PRIMARY KEY,
    workspace_id      TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    created_by        TEXT NOT NULL REFERENCES users(id),
    type              TEXT NOT NULL,
    label             TEXT NOT NULL DEFAULT '',
    config            TEXT NOT NULL DEFAULT '',
    public_token      TEXT NOT NULL DEFAULT '',
    dest_folder_id    TEXT REFERENCES folders(id),
    dest_project_id   TEXT REFERENCES projects(id),
    enabled           INTEGER NOT NULL DEFAULT 1,
    poll_interval_min INTEGER NOT NULL DEFAULT 15,
    last_polled_at    DATETIME,
    last_error        TEXT,
    created_at        DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at        DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_ingress_sources_workspace ON ingress_sources(workspace_id);
CREATE INDEX idx_ingress_sources_due       ON ingress_sources(enabled, last_polled_at);
CREATE UNIQUE INDEX idx_ingress_sources_public_token
    ON ingress_sources(public_token) WHERE public_token != '';

CREATE TABLE ingress_log (
    id          TEXT PRIMARY KEY,
    source_id   TEXT NOT NULL REFERENCES ingress_sources(id) ON DELETE CASCADE,
    remote_id   TEXT NOT NULL,
    filename    TEXT NOT NULL,
    asset_id    TEXT REFERENCES assets(id) ON DELETE SET NULL,
    status      TEXT NOT NULL DEFAULT 'pending'
                    CHECK(status IN ('pending', 'imported', 'skipped', 'error')),
    error       TEXT,
    imported_at DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE(source_id, remote_id)
);

CREATE INDEX idx_ingress_log_source ON ingress_log(source_id, imported_at);
CREATE INDEX idx_ingress_log_status ON ingress_log(status);

CREATE TABLE ingress_rules (
    id        TEXT PRIMARY KEY,
    source_id TEXT NOT NULL REFERENCES ingress_sources(id) ON DELETE CASCADE,
    position  INTEGER NOT NULL DEFAULT 0,
    field     TEXT NOT NULL,
    operator  TEXT NOT NULL,
    value     TEXT NOT NULL,
    action    TEXT NOT NULL
);

CREATE INDEX idx_ingress_rules_source ON ingress_rules(source_id, position);
