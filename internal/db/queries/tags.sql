-- name: GetOrCreateTag :one
INSERT INTO tags (id, workspace_id, name)
VALUES (?, ?, ?)
ON CONFLICT (workspace_id, name) DO UPDATE SET name = name
RETURNING *;

-- name: GetTagByWorkspaceAndName :one
SELECT * FROM tags WHERE workspace_id = ? AND name = ?;

-- name: ListTagsWithCount :many
SELECT t.id, t.workspace_id, t.name, t.color, t.group_name, t.created_at, t.last_used_at,
       COUNT(at.asset_id) AS asset_count
FROM tags t
LEFT JOIN asset_tags at ON at.tag_id = t.id
WHERE t.workspace_id = ?
GROUP BY t.id
ORDER BY t.name ASC;

-- name: GetTagsForAsset :many
SELECT t.id, t.workspace_id, t.name
FROM tags t
JOIN asset_tags at ON at.tag_id = t.id
WHERE at.asset_id = ?;

-- name: AddTagToAsset :exec
INSERT OR IGNORE INTO asset_tags (asset_id, tag_id) VALUES (?, ?);

-- name: RemoveTagFromAsset :exec
DELETE FROM asset_tags
WHERE asset_id = ?
  AND tag_id = (SELECT id FROM tags WHERE workspace_id = ? AND name = ?);

-- name: TouchTagLastUsed :exec
UPDATE tags SET last_used_at = datetime('now') WHERE workspace_id = ? AND name = ?;

-- name: CreateTag :one
INSERT INTO tags (id, workspace_id, name, color, group_name)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateTagName :exec
UPDATE tags SET name = ? WHERE workspace_id = ? AND name = ?;

-- name: UpdateTagMetadata :exec
UPDATE tags SET color = ?, group_name = ? WHERE workspace_id = ? AND name = ?;

-- name: DeleteTag :exec
DELETE FROM tags WHERE workspace_id = ? AND name = ?;

-- name: ListTagsInWorkspace :many
SELECT * FROM tags WHERE workspace_id = ? ORDER BY name ASC;
