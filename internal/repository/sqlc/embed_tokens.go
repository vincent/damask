package reposqlc

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"damask/server/internal/apperr"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
)

type embedTokenRepo struct {
	q *dbgen.Queries
}

// NewEmbedTokenRepo returns a repository.EmbedTokenRepository backed by sqlc-generated queries.
func NewEmbedTokenRepo(q *dbgen.Queries) repository.EmbedTokenRepository {
	return &embedTokenRepo{q: q}
}

func (r *embedTokenRepo) Create(
	ctx context.Context,
	params repository.CreateEmbedTokenParams,
) (repository.EmbedToken, error) {
	row, err := r.q.CreateEmbedToken(ctx, dbgen.CreateEmbedTokenParams{
		ID:          params.ID,
		WorkspaceID: params.WorkspaceID,
		AssetID:     params.AssetID,
		CreatedBy:   params.CreatedBy,
		Label:       params.Label,
	})
	if err != nil {
		if isUniqueConstraintErr(err) {
			return repository.EmbedToken{}, apperr.ErrConflict
		}
		return repository.EmbedToken{}, err
	}
	return toEmbedToken(row), nil
}

func (r *embedTokenRepo) GetByID(ctx context.Context, id string) (repository.EmbedToken, error) {
	row, err := r.q.GetEmbedTokenByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.EmbedToken{}, apperr.ErrNotFound
		}
		return repository.EmbedToken{}, err
	}
	return toEmbedToken(row), nil
}

func (r *embedTokenRepo) GetActiveByAssetID(
	ctx context.Context,
	workspaceID, assetID string,
) (repository.EmbedToken, error) {
	row, err := r.q.GetActiveEmbedTokenByAssetID(ctx, dbgen.GetActiveEmbedTokenByAssetIDParams{
		WorkspaceID: workspaceID,
		AssetID:     assetID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.EmbedToken{}, apperr.ErrNotFound
		}
		return repository.EmbedToken{}, err
	}
	return toEmbedToken(row), nil
}

func (r *embedTokenRepo) Revoke(ctx context.Context, workspaceID, id string) error {
	rows, err := r.q.RevokeEmbedToken(ctx, dbgen.RevokeEmbedTokenParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return apperr.ErrNotFound
	}
	return nil
}

func toEmbedToken(t dbgen.AssetEmbedToken) repository.EmbedToken {
	return repository.EmbedToken{
		ID:          t.ID,
		WorkspaceID: t.WorkspaceID,
		AssetID:     t.AssetID,
		CreatedBy:   t.CreatedBy,
		Label:       t.Label,
		CreatedAt:   t.CreatedAt,
		RevokedAt:   t.RevokedAt,
	}
}

// isUniqueConstraintErr reports whether err is a SQLite unique-constraint violation.
func isUniqueConstraintErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
