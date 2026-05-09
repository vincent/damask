package reposqlc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"damask/server/internal/apperr"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
)

type tagRepo struct {
	q     *dbgen.Queries
	sqlDB *sql.DB
}

// NewTagRepo returns a repository.TagRepository backed by sqlc-generated queries.
func NewTagRepo(q *dbgen.Queries, sqlDB *sql.DB) repository.TagRepository {
	return &tagRepo{q: q, sqlDB: sqlDB}
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

func (r *tagRepo) List(ctx context.Context, workspaceID string, includeSystem bool) ([]repository.Tag, error) {
	rows, err := r.q.ListTagsWithCount(ctx, dbgen.ListTagsWithCountParams{
		WorkspaceID:   workspaceID,
		IncludeSystem: includeSystem,
	})
	if err != nil {
		return nil, err
	}
	out := make([]repository.Tag, len(rows))
	for i, row := range rows {
		out[i] = repository.Tag{
			ID:          row.ID,
			WorkspaceID: row.WorkspaceID,
			Name:        row.Name,
			Color:       row.Color,
			GroupName:   row.GroupName,
			AssetCount:  row.AssetCount,
			CreatedAt:   row.CreatedAt,
			LastUsedAt:  row.LastUsedAt,
		}
	}
	return out, nil
}

func (r *tagRepo) EnsureSystemTag(ctx context.Context, workspaceID, name string) error {
	return r.q.EnsureSystemTag(ctx, dbgen.EnsureSystemTagParams{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		Name:        name,
	})
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

func (r *tagRepo) UpdateMetadata(ctx context.Context, workspaceID, name string, color, groupName *string) error {
	return r.q.UpdateTagMetadata(ctx, dbgen.UpdateTagMetadataParams{
		WorkspaceID: workspaceID,
		Name:        name,
		Color:       color,
		GroupName:   groupName,
	})
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

func (r *tagRepo) RemoveFromAsset(ctx context.Context, workspaceID, assetID, tagName string) error {
	return r.q.RemoveTagFromAsset(ctx, dbgen.RemoveTagFromAssetParams{
		AssetID:     assetID,
		WorkspaceID: workspaceID,
		Name:        tagName,
	})
}

func (r *tagRepo) BatchTagsForAssets(ctx context.Context, assetIDs []string) (map[string][]string, error) {
	if len(assetIDs) == 0 {
		return map[string][]string{}, nil
	}
	placeholders := make([]string, len(assetIDs))
	args := make([]any, len(assetIDs))
	for i, id := range assetIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	q := fmt.Sprintf(
		`SELECT at.asset_id, t.name FROM asset_tags at JOIN tags t ON t.id = at.tag_id WHERE at.asset_id IN (%s)`,
		strings.Join(placeholders, ","),
	)
	rows, err := r.sqlDB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string][]string, len(assetIDs))
	for rows.Next() {
		var assetID, name string
		if err := rows.Scan(&assetID, &name); err != nil {
			return nil, err
		}
		out[assetID] = append(out[assetID], name)
	}
	return out, rows.Err()
}

func (r *tagRepo) CountAssets(ctx context.Context, tagID string) (int64, error) {
	return r.q.CountTagAssets(ctx, tagID)
}

func (r *tagRepo) ReassignAssets(ctx context.Context, fromTagID, toTagID string) error {
	return r.q.ReassignTagAssets(ctx, dbgen.ReassignTagAssetsParams{
		TagID:   toTagID,
		TagID_2: fromTagID,
	})
}

func (r *tagRepo) TouchLastUsed(ctx context.Context, workspaceID, name string) error {
	return r.q.TouchTagLastUsed(ctx, dbgen.TouchTagLastUsedParams{
		WorkspaceID: workspaceID,
		Name:        name,
	})
}

func (r *tagRepo) FindAssetBySystemTagInFolder(ctx context.Context, workspaceID, tagName, folderID string) (repository.Asset, error) {
	row, err := r.q.FindAssetBySystemTagInFolder(ctx, dbgen.FindAssetBySystemTagInFolderParams{
		WorkspaceID: workspaceID,
		Name:        tagName,
		FolderID:    &folderID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Asset{}, apperr.ErrNotFound
		}
		return repository.Asset{}, err
	}
	return toAsset(row), nil
}

func (r *tagRepo) FindAssetBySystemTagInProject(ctx context.Context, workspaceID, tagName, projectID string) (repository.Asset, error) {
	row, err := r.q.FindAssetBySystemTagInProject(ctx, dbgen.FindAssetBySystemTagInProjectParams{
		WorkspaceID: workspaceID,
		Name:        tagName,
		ProjectID:   &projectID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Asset{}, apperr.ErrNotFound
		}
		return repository.Asset{}, err
	}
	return toAsset(row), nil
}

func (r *tagRepo) FindAssetBySystemTagInWorkspace(ctx context.Context, workspaceID, tagName string) (repository.Asset, error) {
	row, err := r.q.FindAssetBySystemTagInWorkspace(ctx, dbgen.FindAssetBySystemTagInWorkspaceParams{
		WorkspaceID: workspaceID,
		Name:        tagName,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Asset{}, apperr.ErrNotFound
		}
		return repository.Asset{}, err
	}
	return toAsset(row), nil
}

func (r *tagRepo) RunInTx(ctx context.Context, fn func(tx repository.TagRepository) error) error {
	tx, err := r.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	txRepo := &tagRepo{q: r.q.WithTx(tx), sqlDB: r.sqlDB}
	if err := fn(txRepo); err != nil {
		return err
	}
	return tx.Commit()
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
