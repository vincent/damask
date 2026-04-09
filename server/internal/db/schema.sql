CREATE TABLE workspaces (
    id                          TEXT PRIMARY KEY,
    name                        TEXT NOT NULL,
    ingest_token                TEXT UNIQUE,
    version_retention_count     INTEGER NOT NULL DEFAULT 0,
    event_log_retention_days    INTEGER NOT NULL DEFAULT 365,
    download_log_retention_days INTEGER NOT NULL DEFAULT 30,
    icon_asset_id               TEXT,
    icon_version_id             TEXT,
    created_at                  DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at                  DATETIME NOT NULL DEFAULT (datetime('now'))
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
    id               TEXT PRIMARY KEY,
    workspace_id     TEXT NOT NULL REFERENCES workspaces(id),
    name             TEXT NOT NULL,
    description      TEXT,
    color            TEXT,
    cover_asset_id   TEXT,
    cover_version_id TEXT,
    created_at       DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at       DATETIME NOT NULL DEFAULT (datetime('now'))
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
    id                  TEXT PRIMARY KEY,
    workspace_id        TEXT NOT NULL REFERENCES workspaces(id),
    project_id          TEXT REFERENCES projects(id),
    folder_id           TEXT REFERENCES folders(id),
    original_filename   TEXT NOT NULL,
    storage_key         TEXT NOT NULL,
    mime_type           TEXT NOT NULL,
    size                INTEGER NOT NULL,
    width               INTEGER,
    height              INTEGER,
    thumbnail_key       TEXT,
    metadata            TEXT, -- JSON
    current_version_id  TEXT,  -- FK added after asset_versions is created
    created_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at          DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE asset_versions (
  id            TEXT PRIMARY KEY,
  asset_id      TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
  workspace_id  TEXT NOT NULL REFERENCES workspaces(id),
  version_num   INTEGER NOT NULL,
  storage_key   TEXT NOT NULL,
  content_hash  TEXT NOT NULL,
  mime_type     TEXT NOT NULL,
  size          INTEGER NOT NULL,
  width         INTEGER,
  height        INTEGER,
  duration_sec  REAL,
  thumbnail_key TEXT,
  comment       TEXT,
  created_by    TEXT REFERENCES users(id),
  created_at    TEXT NOT NULL DEFAULT (datetime('now')),
  is_current    INTEGER NOT NULL DEFAULT 0,
  deleted_at    TEXT,
  UNIQUE(asset_id, version_num)
);

CREATE INDEX idx_versions_asset     ON asset_versions(asset_id, is_current);
CREATE INDEX idx_versions_workspace ON asset_versions(workspace_id);
CREATE INDEX idx_versions_hash      ON asset_versions(content_hash);
CREATE INDEX idx_versions_created   ON asset_versions(asset_id, created_at DESC);

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

-- variants: bound to asset_versions (versioned, VV-M3)
CREATE TABLE variants (
    id               TEXT PRIMARY KEY,
    workspace_id     TEXT NOT NULL REFERENCES workspaces(id),
    asset_version_id TEXT NOT NULL REFERENCES asset_versions(id) ON DELETE CASCADE,
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
CREATE INDEX idx_variants_version   ON variants(asset_version_id);
CREATE INDEX idx_variants_workspace ON variants(workspace_id);
CREATE INDEX idx_variants_type      ON variants(asset_version_id, type);
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

-- Custom fields (migration 000008)
CREATE TABLE field_definitions (
  id            TEXT PRIMARY KEY,
  workspace_id  TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  created_by    TEXT NOT NULL REFERENCES users(id),
  scope         TEXT NOT NULL CHECK(scope IN ('asset', 'project')),
  name          TEXT NOT NULL,
  key           TEXT NOT NULL,
  field_type    TEXT NOT NULL CHECK(field_type IN ('text','number','date','boolean','select','url')),
  options       TEXT,
  required      INTEGER NOT NULL DEFAULT 0,
  position      INTEGER NOT NULL DEFAULT 0,
  inherit_from_project INTEGER NOT NULL DEFAULT 0,
  created_at    TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at    TEXT NOT NULL DEFAULT (datetime('now')),
  deleted_at    TEXT,
  UNIQUE(workspace_id, scope, key)
);

CREATE INDEX idx_field_defs_workspace ON field_definitions(workspace_id, scope);
CREATE INDEX idx_field_defs_active    ON field_definitions(workspace_id, deleted_at);

CREATE TABLE asset_field_values (
  id            TEXT PRIMARY KEY,
  asset_id      TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
  field_id      TEXT NOT NULL REFERENCES field_definitions(id),
  value_text    TEXT,
  value_number  REAL,
  value_date    TEXT,
  value_boolean INTEGER,
  created_by    TEXT NOT NULL REFERENCES users(id),
  created_at    TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at    TEXT NOT NULL DEFAULT (datetime('now')),
  UNIQUE(asset_id, field_id)
);

CREATE INDEX idx_afv_asset    ON asset_field_values(asset_id);
CREATE INDEX idx_afv_field    ON asset_field_values(field_id);
CREATE INDEX idx_afv_text     ON asset_field_values(field_id, value_text);
CREATE INDEX idx_afv_number   ON asset_field_values(field_id, value_number);
CREATE INDEX idx_afv_date     ON asset_field_values(field_id, value_date);
CREATE INDEX idx_afv_boolean  ON asset_field_values(field_id, value_boolean);

CREATE TABLE project_field_values (
  id            TEXT PRIMARY KEY,
  project_id    TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  field_id      TEXT NOT NULL REFERENCES field_definitions(id),
  value_text    TEXT,
  value_number  REAL,
  value_date    TEXT,
  value_boolean INTEGER,
  created_by    TEXT NOT NULL REFERENCES users(id),
  created_at    TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at    TEXT NOT NULL DEFAULT (datetime('now')),
  UNIQUE(project_id, field_id)
);

CREATE INDEX idx_pfv_project  ON project_field_values(project_id);
CREATE INDEX idx_pfv_field    ON project_field_values(field_id);
CREATE INDEX idx_pfv_text     ON project_field_values(field_id, value_text);
CREATE INDEX idx_pfv_number   ON project_field_values(field_id, value_number);
CREATE INDEX idx_pfv_date     ON project_field_values(field_id, value_date);

-- Event log (migration 000011)
CREATE TABLE asset_events (
  id           TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id),
  asset_id     TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
  user_id      TEXT REFERENCES users(id),
  actor_type   TEXT NOT NULL DEFAULT 'user' CHECK(actor_type IN ('user','system')),
  event_type   TEXT NOT NULL,
  payload      TEXT NOT NULL DEFAULT '{}',
  created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_aevents_asset     ON asset_events(asset_id, created_at DESC);
CREATE INDEX idx_aevents_workspace ON asset_events(workspace_id, created_at DESC);
CREATE INDEX idx_aevents_user      ON asset_events(user_id, created_at DESC);
CREATE INDEX idx_aevents_type      ON asset_events(workspace_id, event_type);

CREATE TABLE project_events (
  id           TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id),
  project_id   TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  user_id      TEXT REFERENCES users(id),
  actor_type   TEXT NOT NULL DEFAULT 'user' CHECK(actor_type IN ('user','system')),
  event_type   TEXT NOT NULL,
  payload      TEXT NOT NULL DEFAULT '{}',
  created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_pevents_project   ON project_events(project_id, created_at DESC);
CREATE INDEX idx_pevents_workspace ON project_events(workspace_id, created_at DESC);
CREATE INDEX idx_pevents_user      ON project_events(user_id, created_at DESC);
