package reposqlc

import (
	"context"
	"database/sql"
	"errors"

	"damask/server/internal/apperr"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
)

type fieldRepo struct {
	q *dbgen.Queries
}

// NewFieldRepo returns a repository.FieldRepository backed by sqlc-generated queries.
func NewFieldRepo(q *dbgen.Queries) repository.FieldRepository {
	return &fieldRepo{q: q}
}

func (r *fieldRepo) GetByID(ctx context.Context, workspaceID, id string) (repository.FieldDefinition, error) {
	row, err := r.q.GetFieldDefinitionByID(ctx, dbgen.GetFieldDefinitionByIDParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.FieldDefinition{}, apperr.ErrNotFound
		}
		return repository.FieldDefinition{}, err
	}
	return toField(row), nil
}

func (r *fieldRepo) List(ctx context.Context, workspaceID, scope string) ([]repository.FieldDefinition, error) {
	rows, err := r.q.ListFieldDefinitions(ctx, dbgen.ListFieldDefinitionsParams{
		WorkspaceID: workspaceID,
		Scope:       scope,
	})
	if err != nil {
		return nil, err
	}
	out := make([]repository.FieldDefinition, len(rows))
	for i, row := range rows {
		out[i] = toField(row)
	}
	return out, nil
}

func (r *fieldRepo) Create(ctx context.Context, f repository.FieldDefinition) (repository.FieldDefinition, error) {
	row, err := r.q.CreateFieldDefinition(ctx, dbgen.CreateFieldDefinitionParams{
		ID:                 f.ID,
		WorkspaceID:        f.WorkspaceID,
		CreatedBy:          f.CreatedBy,
		Scope:              f.Scope,
		Name:               f.Name,
		Key:                f.Key,
		FieldType:          f.FieldType,
		Options:            f.Options,
		Required:           boolToInt64(f.Required),
		Position:           f.Position,
		InheritFromProject: boolToInt64(f.InheritFromProject),
	})
	if err != nil {
		return repository.FieldDefinition{}, err
	}
	return toField(row), nil
}

func (r *fieldRepo) Update(ctx context.Context, f repository.FieldDefinition) (repository.FieldDefinition, error) {
	req := boolToInt64(f.Required)
	ifp := boolToInt64(f.InheritFromProject)
	row, err := r.q.UpdateFieldDefinition(ctx, dbgen.UpdateFieldDefinitionParams{
		ID:                 f.ID,
		WorkspaceID:        f.WorkspaceID,
		Name:               &f.Name,
		Options:            f.Options,
		Required:           &req,
		Position:           &f.Position,
		InheritFromProject: &ifp,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.FieldDefinition{}, apperr.ErrNotFound
		}
		return repository.FieldDefinition{}, err
	}
	return toField(row), nil
}

func (r *fieldRepo) SoftDelete(ctx context.Context, workspaceID, id string) error {
	return r.q.SoftDeleteFieldDefinition(ctx, dbgen.SoftDeleteFieldDefinitionParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
}

func (r *fieldRepo) CountByWorkspaceAndScope(ctx context.Context, workspaceID, scope string) (int64, error) {
	return r.q.CountFieldDefinitions(ctx, dbgen.CountFieldDefinitionsParams{
		WorkspaceID: workspaceID,
		Scope:       scope,
	})
}

func toField(f dbgen.FieldDefinition) repository.FieldDefinition {
	return repository.FieldDefinition{
		ID:                 f.ID,
		WorkspaceID:        f.WorkspaceID,
		CreatedBy:          f.CreatedBy,
		Scope:              f.Scope,
		Name:               f.Name,
		Key:                f.Key,
		FieldType:          f.FieldType,
		Options:            f.Options,
		Required:           f.Required != 0,
		Position:           f.Position,
		InheritFromProject: f.InheritFromProject != 0,
		CreatedAt:          parseCreatedAt(f.CreatedAt),
		UpdatedAt:          parseCreatedAt(f.UpdatedAt),
		DeletedAt:          f.DeletedAt,
	}
}

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
