-- name: CreateJob :one
INSERT INTO jobs (id, workspace_id, type, payload, status)
VALUES (?, ?, ?, ?, 'pending')
RETURNING *;

-- name: ClaimNextJob :one
UPDATE jobs
SET status = 'processing', attempts = attempts + 1, updated_at = datetime('now')
WHERE id = (
    SELECT id FROM jobs WHERE status = 'pending' ORDER BY created_at ASC LIMIT 1
)
RETURNING *;

-- name: CompleteJob :exec
UPDATE jobs SET status = 'done', updated_at = datetime('now') WHERE id = ?;

-- name: FailJob :exec
UPDATE jobs SET status = 'failed', error = ?, updated_at = datetime('now') WHERE id = ?;

-- name: RequeueStalledJobs :exec
UPDATE jobs SET status = 'pending', updated_at = datetime('now')
WHERE status = 'processing';

-- name: CountPendingJobs :one
SELECT COUNT(*) FROM jobs WHERE status = 'pending';

-- name: GetJobByID :one
SELECT * FROM jobs WHERE id = ?;

-- name: CompleteJobWithResult :exec
UPDATE jobs SET status = 'done', result = ?, updated_at = datetime('now') WHERE id = ?;

-- name: CreateJobForWorkspace :one
INSERT INTO jobs (id, workspace_id, type, payload, status)
VALUES (?, ?, ?, ?, 'pending')
RETURNING *;
