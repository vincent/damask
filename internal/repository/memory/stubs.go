// Package memory provides in-memory repository implementations for unit tests.
// This file contains minimal stubs for aggregates that are not yet exercised by
// service-layer unit tests. Flesh out each stub when the corresponding service
// is migrated in Phase 2.
package memory

import (
	"context"

	"damask/server/internal/repository"
)

// FolderRepo ---------------------------------------------------------------

type FolderRepo struct{}

func NewFolderRepo() *FolderRepo { return &FolderRepo{} }

func (r *FolderRepo) GetByID(_ context.Context, _, _ string) (repository.Folder, error) {
	return repository.Folder{}, nil
}
func (r *FolderRepo) ListByProject(_ context.Context, _, _ string) ([]repository.Folder, error) {
	return nil, nil
}
func (r *FolderRepo) Create(_ context.Context, f repository.Folder) (repository.Folder, error) {
	return f, nil
}
func (r *FolderRepo) Update(_ context.Context, f repository.Folder) (repository.Folder, error) {
	return f, nil
}
func (r *FolderRepo) Delete(_ context.Context, _, _ string) error           { return nil }
func (r *FolderRepo) GetChildren(_ context.Context, _, _ string) ([]repository.Folder, error) {
	return nil, nil
}
func (r *FolderRepo) NullifyAssets(_ context.Context, _, _ string) error { return nil }

// TagRepo ------------------------------------------------------------------

type TagRepo struct{}

func NewTagRepo() *TagRepo { return &TagRepo{} }

func (r *TagRepo) GetByName(_ context.Context, _, _ string) (repository.Tag, error) {
	return repository.Tag{}, nil
}
func (r *TagRepo) List(_ context.Context, _ string) ([]repository.Tag, error) { return nil, nil }
func (r *TagRepo) Upsert(_ context.Context, _, _ string) (repository.Tag, error) {
	return repository.Tag{}, nil
}
func (r *TagRepo) UpdateMetadata(_ context.Context, _, _ string, _, _ *string) error { return nil }
func (r *TagRepo) Rename(_ context.Context, _, _, _ string) error                     { return nil }
func (r *TagRepo) Delete(_ context.Context, _ string, _ []string) error { return nil }
func (r *TagRepo) ListForAsset(_ context.Context, _ string) ([]repository.Tag, error) {
	return nil, nil
}
func (r *TagRepo) AddToAsset(_ context.Context, _, _ string) error    { return nil }
func (r *TagRepo) RemoveFromAsset(_ context.Context, _, _, _ string) error { return nil }

// CollectionRepo -----------------------------------------------------------

type CollectionRepo struct{}

func NewCollectionRepo() *CollectionRepo { return &CollectionRepo{} }

func (r *CollectionRepo) GetByID(_ context.Context, _, _ string) (repository.Collection, error) {
	return repository.Collection{}, nil
}
func (r *CollectionRepo) List(_ context.Context, _ string) ([]repository.Collection, error) {
	return nil, nil
}
func (r *CollectionRepo) Create(_ context.Context, c repository.Collection) (repository.Collection, error) {
	return c, nil
}
func (r *CollectionRepo) Update(_ context.Context, c repository.Collection) (repository.Collection, error) {
	return c, nil
}
func (r *CollectionRepo) Delete(_ context.Context, _, _ string) error   { return nil }
func (r *CollectionRepo) AddAsset(_ context.Context, _, _ string) error        { return nil }
func (r *CollectionRepo) RemoveAsset(_ context.Context, _, _ string) error     { return nil }
func (r *CollectionRepo) ListForAsset(_ context.Context, _, _ string) ([]repository.Collection, error) {
	return nil, nil
}
func (r *CollectionRepo) CountAssets(_ context.Context, _ string) (int64, error) { return 0, nil }

// ShareRepo ----------------------------------------------------------------

type ShareRepo struct{}

func NewShareRepo() *ShareRepo { return &ShareRepo{} }

func (r *ShareRepo) GetByID(_ context.Context, _, _ string) (repository.Share, error) {
	return repository.Share{}, nil
}
func (r *ShareRepo) List(_ context.Context, _ string) ([]repository.Share, error) { return nil, nil }
func (r *ShareRepo) Create(_ context.Context, s repository.Share) (repository.Share, error) {
	return s, nil
}
func (r *ShareRepo) Update(_ context.Context, s repository.Share) (repository.Share, error) {
	return s, nil
}
func (r *ShareRepo) Revoke(_ context.Context, _, _ string) error { return nil }

// VersionRepo --------------------------------------------------------------

type VersionRepo struct{}

func NewVersionRepo() *VersionRepo { return &VersionRepo{} }

func (r *VersionRepo) GetByID(_ context.Context, _ string) (repository.AssetVersion, error) {
	return repository.AssetVersion{}, nil
}
func (r *VersionRepo) GetByIDForWorkspace(_ context.Context, _, _ string) (repository.AssetVersion, error) {
	return repository.AssetVersion{}, nil
}
func (r *VersionRepo) GetCurrentByAsset(_ context.Context, _ string) (repository.AssetVersion, error) {
	return repository.AssetVersion{}, nil
}
func (r *VersionRepo) ListByAsset(_ context.Context, _ string) ([]repository.AssetVersion, error) {
	return nil, nil
}
func (r *VersionRepo) Create(_ context.Context, v repository.AssetVersion) (repository.AssetVersion, error) {
	return v, nil
}
func (r *VersionRepo) SoftDelete(_ context.Context, _ string) error             { return nil }
func (r *VersionRepo) Delete(_ context.Context, _ string) error                 { return nil }
func (r *VersionRepo) CountByAsset(_ context.Context, _ string) (int64, error)  { return 0, nil }
func (r *VersionRepo) IsReferencedAsCover(_ context.Context, _ string) (bool, error) {
	return false, nil
}

// FieldRepo ----------------------------------------------------------------

type FieldRepo struct{}

func NewFieldRepo() *FieldRepo { return &FieldRepo{} }

func (r *FieldRepo) GetByID(_ context.Context, _, _ string) (repository.FieldDefinition, error) {
	return repository.FieldDefinition{}, nil
}
func (r *FieldRepo) List(_ context.Context, _, _ string) ([]repository.FieldDefinition, error) {
	return nil, nil
}
func (r *FieldRepo) Create(_ context.Context, f repository.FieldDefinition) (repository.FieldDefinition, error) {
	return f, nil
}
func (r *FieldRepo) Update(_ context.Context, f repository.FieldDefinition) (repository.FieldDefinition, error) {
	return f, nil
}
func (r *FieldRepo) SoftDelete(_ context.Context, _, _ string) error { return nil }
func (r *FieldRepo) CountByWorkspaceAndScope(_ context.Context, _, _ string) (int64, error) {
	return 0, nil
}

// WorkspaceRepo ------------------------------------------------------------

type WorkspaceRepo struct{}

func NewWorkspaceRepo() *WorkspaceRepo { return &WorkspaceRepo{} }

func (r *WorkspaceRepo) GetByID(_ context.Context, _ string) (repository.Workspace, error) {
	return repository.Workspace{}, nil
}
func (r *WorkspaceRepo) Update(_ context.Context, w repository.Workspace) (repository.Workspace, error) {
	return w, nil
}

// UserRepo -----------------------------------------------------------------

type UserRepo struct{}

func NewUserRepo() *UserRepo { return &UserRepo{} }

func (r *UserRepo) GetByID(_ context.Context, _ string) (repository.User, error) {
	return repository.User{}, nil
}
func (r *UserRepo) GetByEmail(_ context.Context, _ string) (repository.User, error) {
	return repository.User{}, nil
}
func (r *UserRepo) Create(_ context.Context, u repository.User) (repository.User, error) {
	return u, nil
}
func (r *UserRepo) Update(_ context.Context, u repository.User) (repository.User, error) {
	return u, nil
}
