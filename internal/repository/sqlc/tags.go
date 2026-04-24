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

type sqlExecer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type tagRepo struct {
	q     *dbgen.Queries
	sqlDB *sql.DB    // held for RunInTx (BeginTx); nil inside a tx
	exec  sqlExecer  // *sql.DB or *sql.Tx for raw queries
}

// NewTagRepo returns a repository.TagRepository backed by sqlc-generated queries.
func NewTagRepo(q *dbgen.Queries, sqlDB *sql.DB) repository.TagRepository {
	return &tagRepo{q: q, sqlDB: sqlDB, exec: sqlDB}
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
	rows, err := r.q.ListTagsWithCount(ctx, workspaceID)
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

func (r *tagRepo) CountAssets(ctx context.Context, tagID string) (int64, error) {
	var count int64
	err := r.exec.QueryRowContext(ctx, `SELECT COUNT(*) FROM asset_tags WHERE tag_id = ?`, tagID).Scan(&count)
	return count, err
}

func (r *tagRepo) ReassignAssets(ctx context.Context, fromTagID, toTagID string) error {
	_, err := r.exec.ExecContext(ctx,
		`INSERT OR IGNORE INTO asset_tags (asset_id, tag_id) SELECT asset_id, ? FROM asset_tags WHERE tag_id = ?`,
		toTagID, fromTagID,
	)
	return err
}

func (r *tagRepo) TouchLastUsed(ctx context.Context, workspaceID, name string) error {
	return r.q.TouchTagLastUsed(ctx, dbgen.TouchTagLastUsedParams{
		WorkspaceID: workspaceID,
		Name:        name,
	})
}

func (r *tagRepo) RunInTx(ctx context.Context, fn func(tx repository.TagRepository) error) error {
	tx, err := r.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	txRepo := &tagRepo{q: r.q.WithTx(tx), sqlDB: r.sqlDB, exec: tx}
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
