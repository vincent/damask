package reposqlc

import (
	"context"
	"database/sql"
	"errors"

	"damask/server/internal/apperr"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
)

type variantRepo struct {
	q *dbgen.Queries
}

// NewVariantRepo returns a repository.VariantRepository backed by sqlc-generated queries.
func NewVariantRepo(q *dbgen.Queries) repository.VariantRepository {
	return &variantRepo{q: q}
}

func (r *variantRepo) GetByID(ctx context.Context, workspaceID, id string) (repository.Variant, error) {
	row, err := r.q.GetVariantByID(ctx, dbgen.GetVariantByIDParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Variant{}, apperr.ErrNotFound
		}
		return repository.Variant{}, err
	}
	return toVariant(row), nil
}

func (r *variantRepo) ListByAsset(ctx context.Context, _ string, assetID string) ([]repository.Variant, error) {
	rows, err := r.q.ListVariantsByAssetCurrentVersion(ctx, assetID)
	if err != nil {
		return nil, err
	}
	out := make([]repository.Variant, len(rows))
	for i, row := range rows {
		out[i] = toVariant(row)
	}
	return out, nil
}

func (r *variantRepo) Create(ctx context.Context, v repository.Variant) (repository.Variant, error) {
	row, err := r.q.CreateVariant(ctx, dbgen.CreateVariantParams{
		ID:              v.ID,
		WorkspaceID:     v.WorkspaceID,
		AssetVersionID:  v.AssetVersionID,
		Type:            v.Type,
		StorageKey:      v.StorageKey,
		TransformParams: v.TransformParams,
		Size:            v.Size,
	})
	if err != nil {
		return repository.Variant{}, err
	}
	return toVariant(row), nil
}

func (r *variantRepo) Delete(ctx context.Context, workspaceID, id string) error {
	return r.q.DeleteVariant(ctx, dbgen.DeleteVariantParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
}

func toVariant(v dbgen.Variant) repository.Variant {
	return repository.Variant{
		ID:              v.ID,
		WorkspaceID:     v.WorkspaceID,
		AssetVersionID:  v.AssetVersionID,
		Type:            v.Type,
		StorageKey:      v.StorageKey,
		TransformParams: v.TransformParams,
		Size:            v.Size,
		CreatedAt:       v.CreatedAt,
	}
}
