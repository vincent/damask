package reposqlc

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"damask/server/internal/apperr"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
)

type shareRepo struct {
	q *dbgen.Queries
}

// NewShareRepo returns a repository.ShareRepository backed by sqlc-generated queries.
func NewShareRepo(q *dbgen.Queries) repository.ShareRepository {
	return &shareRepo{q: q}
}

func (r *shareRepo) GetByID(ctx context.Context, workspaceID, id string) (repository.Share, error) {
	row, err := r.q.GetShareByIDAndWorkspace(ctx, dbgen.GetShareByIDAndWorkspaceParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Share{}, apperr.ErrNotFound
		}
		return repository.Share{}, err
	}
	return toShare(row), nil
}

func (r *shareRepo) List(ctx context.Context, workspaceID string) ([]repository.Share, error) {
	rows, err := r.q.ListSharesByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]repository.Share, len(rows))
	for i, row := range rows {
		out[i] = toShare(row)
	}
	return out, nil
}

func (r *shareRepo) Create(ctx context.Context, s repository.Share) (repository.Share, error) {
	row, err := r.q.CreateShare(ctx, dbgen.CreateShareParams{
		ID:            s.ID,
		WorkspaceID:   s.WorkspaceID,
		CreatedBy:     s.CreatedBy,
		Label:         s.Label,
		TargetType:    s.TargetType,
		TargetID:      s.TargetID,
		PasswordHash:  s.PasswordHash,
		ExpiresAt:     s.ExpiresAt,
		AllowComments: boolToInt(s.AllowComments),
		AllowDownload: boolToInt(s.AllowDownload),
	})
	if err != nil {
		return repository.Share{}, err
	}
	return toShare(row), nil
}

func (r *shareRepo) Update(ctx context.Context, s repository.Share) (repository.Share, error) {
	row, err := r.q.UpdateShare(ctx, dbgen.UpdateShareParams{
		ID:            s.ID,
		WorkspaceID:   s.WorkspaceID,
		Label:         s.Label,
		PasswordHash:  s.PasswordHash,
		ExpiresAt:     s.ExpiresAt,
		AllowComments: boolToInt(s.AllowComments),
		AllowDownload: boolToInt(s.AllowDownload),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Share{}, apperr.ErrNotFound
		}
		return repository.Share{}, err
	}
	return toShare(row), nil
}

func (r *shareRepo) Revoke(ctx context.Context, workspaceID, id string) error {
	return r.q.RevokeShare(ctx, dbgen.RevokeShareParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
}

func toShare(s dbgen.Share) repository.Share {
	return repository.Share{
		ID:            s.ID,
		WorkspaceID:   s.WorkspaceID,
		CreatedBy:     s.CreatedBy,
		Label:         s.Label,
		TargetType:    s.TargetType,
		TargetID:      s.TargetID,
		PasswordHash:  s.PasswordHash,
		ExpiresAt:     s.ExpiresAt,
		AllowComments: s.AllowComments != 0,
		AllowDownload: s.AllowDownload != 0,
		ViewCount:     s.ViewCount,
		CreatedAt:     parseShareTime(s.CreatedAt),
		RevokedAt:     s.RevokedAt,
	}
}

func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func parseShareTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, _ = time.Parse("2006-01-02 15:04:05", s)
	}
	return t
}
