package reposqlc

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"damask/server/internal/apperr"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
)

type tagRepo struct {
	q *dbgen.Queries
}

// NewTagRepo returns a repository.TagRepository backed by sqlc-generated queries.
func NewTagRepo(q *dbgen.Queries) repository.TagRepository {
	return &tagRepo{q: q}
}

func (r *tagRepo) GetByName(ctx context.Context, workspaceID, name string) (repository.Tag, error) {
	row, err := r.q.GetTagByWorkspaceAndName(ctx, dbgen.GetTagByWorkspaceAndNameParams{
		WorkspaceID: workspaceID,
		Name:        name,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Tag{}, apperr.ErrNotFound
		}
		return repository.Tag{}, err
	}
	return toTag(row), nil
}

func (r *tagRepo) List(ctx context.Context, workspaceID string) ([]repository.Tag, error) {
	rows, err := r.q.ListTagsInWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]repository.Tag, len(rows))
	for i, row := range rows {
		out[i] = toTag(row)
	}
	return out, nil
}

func (r *tagRepo) Upsert(ctx context.Context, workspaceID, name string) (repository.Tag, error) {
	row, err := r.q.GetOrCreateTag(ctx, dbgen.GetOrCreateTagParams{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		Name:        name,
	})
	if err != nil {
		return repository.Tag{}, err
	}
	return toTag(row), nil
}

func (r *tagRepo) Rename(ctx context.Context, workspaceID, oldName, newName string) error {
	return r.q.UpdateTagName(ctx, dbgen.UpdateTagNameParams{
		WorkspaceID: workspaceID,
		Name_2:      oldName,
		Name:        newName,
	})
}

func (r *tagRepo) Delete(ctx context.Context, workspaceID string, names []string) error {
	for _, name := range names {
		if err := r.q.DeleteTag(ctx, dbgen.DeleteTagParams{
			WorkspaceID: workspaceID,
			Name:        name,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *tagRepo) ListForAsset(ctx context.Context, assetID string) ([]repository.Tag, error) {
	rows, err := r.q.GetTagsForAsset(ctx, assetID)
	if err != nil {
		return nil, err
	}
	out := make([]repository.Tag, len(rows))
	for i, row := range rows {
		out[i] = repository.Tag{
			ID:          row.ID,
			WorkspaceID: row.WorkspaceID,
			Name:        row.Name,
		}
	}
	return out, nil
}

func (r *tagRepo) AddToAsset(ctx context.Context, assetID, tagID string) error {
	return r.q.AddTagToAsset(ctx, dbgen.AddTagToAssetParams{
		AssetID: assetID,
		TagID:   tagID,
	})
}

func (r *tagRepo) RemoveFromAsset(ctx context.Context, assetID, tagName string) error {
	// GetTagByWorkspaceAndName requires workspaceID but interface only provides assetID + name.
	// The sqlc RemoveTagFromAsset takes (assetID, workspaceID, name) -- workspaceID is empty
	// for the base interface; callers that need workspace scoping should use the direct query.
	// This wrapper looks up the member's workspace by querying RemoveTagFromAsset with the
	// asset-scoped variant that matches asset + name across any workspace.
	return r.q.RemoveTagFromAsset(ctx, dbgen.RemoveTagFromAssetParams{
		AssetID: assetID,
		Name:    tagName,
		// WorkspaceID is intentionally empty here; the SQL WHERE uses asset_id + name match.
	})
}

func toTag(t dbgen.Tag) repository.Tag {
	return repository.Tag{
		ID:          t.ID,
		WorkspaceID: t.WorkspaceID,
		Name:        t.Name,
		Color:       t.Color,
		GroupName:   t.GroupName,
		CreatedAt:   t.CreatedAt,
		LastUsedAt:  t.LastUsedAt,
	}
}
