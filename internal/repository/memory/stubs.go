// Package memory provides in-memory repository implementations for unit tests.
// This file contains minimal stubs for aggregates that are not yet exercised by
// service-layer unit tests. Flesh out each stub when the corresponding service
// is migrated in Phase 2.
package memory

import (
	"context"
	"time"

	"damask/server/internal/apperr"
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
func (r *FolderRepo) Delete(_ context.Context, _, _ string) error { return nil }
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
func (r *TagRepo) List(_ context.Context, _ string, _ bool) ([]repository.Tag, error) {
	return nil, nil
}
func (r *TagRepo) Upsert(_ context.Context, _, _ string) (repository.Tag, error) {
	return repository.Tag{}, nil
}
func (r *TagRepo) EnsureSystemTag(_ context.Context, _, _ string) error              { return nil }
func (r *TagRepo) UpdateMetadata(_ context.Context, _, _ string, _, _ *string) error { return nil }
func (r *TagRepo) Rename(_ context.Context, _, _, _ string) error                    { return nil }
func (r *TagRepo) Delete(_ context.Context, _ string, _ []string) error              { return nil }
func (r *TagRepo) ListForAsset(_ context.Context, _ string) ([]repository.Tag, error) {
	return nil, nil
}
func (r *TagRepo) AddToAsset(_ context.Context, _, _ string) error { return nil }
func (r *TagRepo) BatchTagsForAssets(_ context.Context, _ []string) (map[string][]string, error) {
	return map[string][]string{}, nil
}
func (r *TagRepo) RemoveFromAsset(_ context.Context, _, _, _ string) error { return nil }
func (r *TagRepo) CountAssets(_ context.Context, _ string) (int64, error)  { return 0, nil }
func (r *TagRepo) ReassignAssets(_ context.Context, _, _ string) error     { return nil }
func (r *TagRepo) TouchLastUsed(_ context.Context, _, _ string) error      { return nil }
func (r *TagRepo) FindAssetBySystemTagInFolder(_ context.Context, _, _, _ string) (repository.Asset, error) {
	return repository.Asset{}, apperr.ErrNotFound
}
func (r *TagRepo) FindAssetBySystemTagInProject(_ context.Context, _, _, _ string) (repository.Asset, error) {
	return repository.Asset{}, apperr.ErrNotFound
}
func (r *TagRepo) FindAssetBySystemTagInWorkspace(_ context.Context, _, _ string) (repository.Asset, error) {
	return repository.Asset{}, apperr.ErrNotFound
}
func (r *TagRepo) RunInTx(_ context.Context, fn func(repository.TagRepository) error) error {
	return fn(r)
}

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
func (r *CollectionRepo) Delete(_ context.Context, _, _ string) error      { return nil }
func (r *CollectionRepo) AddAsset(_ context.Context, _, _ string) error    { return nil }
func (r *CollectionRepo) RemoveAsset(_ context.Context, _, _ string) error { return nil }
func (r *CollectionRepo) ListForAsset(_ context.Context, _, _ string) ([]repository.Collection, error) {
	return nil, nil
}
func (r *CollectionRepo) CountAssets(_ context.Context, _ string) (int64, error) { return 0, nil }
func (r *CollectionRepo) ListAssetIDs(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

// ShareRepo ----------------------------------------------------------------

type ShareRepo struct{}

func NewShareRepo() *ShareRepo { return &ShareRepo{} }

func (r *ShareRepo) GetByID(_ context.Context, _, _ string) (repository.Share, error) {
	return repository.Share{}, nil
}
func (r *ShareRepo) GetPublic(_ context.Context, _ string) (repository.Share, error) {
	return repository.Share{}, nil
}
func (r *ShareRepo) GetByIDAndWorkspace(_ context.Context, _, _ string) (repository.Share, error) {
	return repository.Share{}, nil
}
func (r *ShareRepo) List(_ context.Context, _ string) ([]repository.Share, error) { return nil, nil }
func (r *ShareRepo) Create(_ context.Context, s repository.Share) (repository.Share, error) {
	return s, nil
}
func (r *ShareRepo) Update(_ context.Context, s repository.Share) (repository.Share, error) {
	return s, nil
}
func (r *ShareRepo) Revoke(_ context.Context, _, _ string) error          { return nil }
func (r *ShareRepo) IncrementViewCount(_ context.Context, _ string) error { return nil }
func (r *ShareRepo) ListAssetsByTarget(_ context.Context, _, _ string) ([]repository.PublicAsset, error) {
	return nil, nil
}
func (r *ShareRepo) GetPublicAsset(_ context.Context, _ string) (repository.PublicAsset, error) {
	return repository.PublicAsset{}, nil
}
func (r *ShareRepo) GetPublicAssetFile(_ context.Context, _ string) (repository.PublicAssetFile, error) {
	return repository.PublicAssetFile{}, nil
}
func (r *ShareRepo) GetPublicAssetThumb(_ context.Context, _ string) (*string, time.Time, error) {
	return nil, time.Time{}, nil
}
func (r *ShareRepo) IsAssetInTarget(_ context.Context, _, _, _ string) (bool, error) {
	return true, nil
}
func (r *ShareRepo) CreateComment(_ context.Context, c repository.ShareComment) (repository.ShareComment, error) {
	return c, nil
}
func (r *ShareRepo) ListCommentsByShare(_ context.Context, _ string) ([]repository.ShareComment, error) {
	return nil, nil
}
func (r *ShareRepo) ListCommentsByShareAndAsset(_ context.Context, _, _ string) ([]repository.ShareComment, error) {
	return nil, nil
}
func (r *ShareRepo) DeleteComment(_ context.Context, _, _ string) error { return nil }

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
func (r *VersionRepo) GetFirstByAsset(_ context.Context, _ string) (repository.AssetVersion, error) {
	return repository.AssetVersion{}, nil
}
func (r *VersionRepo) ListByAsset(_ context.Context, _ string) ([]repository.AssetVersion, error) {
	return nil, nil
}
func (r *VersionRepo) Create(_ context.Context, v repository.AssetVersion) (repository.AssetVersion, error) {
	return v, nil
}
func (r *VersionRepo) SoftDelete(_ context.Context, _ string) error            { return nil }
func (r *VersionRepo) Delete(_ context.Context, _ string) error                { return nil }
func (r *VersionRepo) CountByAsset(_ context.Context, _ string) (int64, error) { return 0, nil }
func (r *VersionRepo) IsReferencedAsCover(_ context.Context, _ string) (bool, error) {
	return false, nil
}
func (r *VersionRepo) GetByHash(_ context.Context, _, _ string) (repository.AssetVersion, error) {
	return repository.AssetVersion{}, apperr.ErrNotFound
}
func (r *VersionRepo) NextVersionNum(_ context.Context, _ string) (int64, error)      { return 1, nil }
func (r *VersionRepo) SetCurrent(_ context.Context, _, _ string) error                { return nil }
func (r *VersionRepo) SetAssetThumbnail(_ context.Context, _ string, _ *string) error { return nil }
func (r *VersionRepo) ListWithVariantCount(_ context.Context, _ string) ([]repository.AssetVersionWithCount, error) {
	return nil, nil
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
func (r *FieldRepo) CountAssetValues(_ context.Context, _ string) (int64, error)   { return 0, nil }
func (r *FieldRepo) CountProjectValues(_ context.Context, _ string) (int64, error) { return 0, nil }
func (r *FieldRepo) UpdatePosition(_ context.Context, _, _ string, _ int64) error  { return nil }
func (r *FieldRepo) InheritProjectFields(_ context.Context, _, _, _, _ string) error {
	return nil
}

// WorkspaceRepo ------------------------------------------------------------

type WorkspaceRepo struct{}

func NewWorkspaceRepo() *WorkspaceRepo { return &WorkspaceRepo{} }

func (r *WorkspaceRepo) GetByID(_ context.Context, _ string) (repository.Workspace, error) {
	return repository.Workspace{}, nil
}
func (r *WorkspaceRepo) Create(_ context.Context, w repository.Workspace) (repository.Workspace, error) {
	return w, nil
}
func (r *WorkspaceRepo) Update(_ context.Context, w repository.Workspace) (repository.Workspace, error) {
	return w, nil
}
func (r *WorkspaceRepo) UpdateLockedTaxonomy(_ context.Context, _ string, _ bool) (repository.Workspace, error) {
	return repository.Workspace{}, nil
}
func (r *WorkspaceRepo) GetImageRouterKey(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (r *WorkspaceRepo) SetImageRouterKey(_ context.Context, _, _ string) error { return nil }
func (r *WorkspaceRepo) ClearImageRouterKey(_ context.Context, _ string) error  { return nil }
func (r *WorkspaceRepo) CountAssets(_ context.Context, _ string) (int64, error) { return 0, nil }
func (r *WorkspaceRepo) GetMember(_ context.Context, _, _ string) (repository.Member, error) {
	return repository.Member{}, nil
}
func (r *WorkspaceRepo) ListMembers(_ context.Context, _ string) ([]repository.Member, error) {
	return nil, nil
}
func (r *WorkspaceRepo) CountMembers(_ context.Context, _ string) (int64, error)   { return 0, nil }
func (r *WorkspaceRepo) CreateMember(_ context.Context, _ repository.Member) error { return nil }
func (r *WorkspaceRepo) DeleteMember(_ context.Context, _, _ string) error         { return nil }
func (r *WorkspaceRepo) UpdateMemberRole(_ context.Context, _, _, _ string) error  { return nil }
func (r *WorkspaceRepo) CreateInvite(_ context.Context, inv repository.Invite) (repository.Invite, error) {
	return inv, nil
}
func (r *WorkspaceRepo) ListPendingInvites(_ context.Context, _ string) ([]repository.Invite, error) {
	return nil, nil
}
func (r *WorkspaceRepo) GetInviteByToken(_ context.Context, _ string) (repository.Invite, error) {
	return repository.Invite{}, nil
}
func (r *WorkspaceRepo) DeleteInvite(_ context.Context, _, _ string) error { return nil }
func (r *WorkspaceRepo) AcceptInvite(_ context.Context, _ string) error    { return nil }
func (r *WorkspaceRepo) ListByUserID(_ context.Context, _ string) ([]repository.WorkspaceWithRole, error) {
	return nil, nil
}
func (r *WorkspaceRepo) RunInTx(_ context.Context, fn func(repository.WorkspaceRepository) error) error {
	return fn(r)
}

func (r *WorkspaceRepo) RunRegistrationTx(
	_ context.Context,
	fn func(context.Context, repository.UserRepository, repository.WorkspaceRepository) error,
) error {
	return fn(context.Background(), &UserRepo{}, r)
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
func (r *UserRepo) UpdateProfile(_ context.Context, _ string, _ string) (repository.User, error) {
	return repository.User{}, nil
}
func (r *UserRepo) UpdateAvatarKey(_ context.Context, _ string, _ string) (repository.User, error) {
	return repository.User{}, nil
}
func (r *UserRepo) ClearAvatarKey(_ context.Context, _ string) (repository.User, error) {
	return repository.User{}, nil
}
func (r *UserRepo) SetPassword(_ context.Context, _, _ string) error {
	return nil
}
func (r *UserRepo) SetAuthMethods(_ context.Context, _ string, _ string) (repository.User, error) {
	return repository.User{}, nil
}
func (r *UserRepo) SetPendingEmail(_ context.Context, _ string, _ string) error {
	return nil
}
func (r *UserRepo) ClearPendingEmail(_ context.Context, _ string) error {
	return nil
}
func (r *UserRepo) ConfirmEmailChange(_ context.Context, _ string, _ string) (repository.User, error) {
	return repository.User{}, nil
}
func (r *UserRepo) SoftDelete(_ context.Context, _ string) error {
	return nil
}
func (r *UserRepo) AnonymizeDeletedUser(_ context.Context, _ string) error {
	return nil
}
func (r *UserRepo) HardDelete(_ context.Context, _ string) error {
	return nil
}
func (r *UserRepo) GetByGoogleID(_ context.Context, _ string) (repository.User, error) {
	return repository.User{}, apperr.ErrNotFound
}
func (r *UserRepo) GetByCanvaID(_ context.Context, _ string) (repository.User, error) {
	return repository.User{}, apperr.ErrNotFound
}
func (r *UserRepo) GetByOIDC(_ context.Context, _, _ string) (repository.User, error) {
	return repository.User{}, apperr.ErrNotFound
}
func (r *UserRepo) CreateWithGoogle(_ context.Context, u repository.User) (repository.User, error) {
	return u, nil
}
func (r *UserRepo) CreateWithOIDC(_ context.Context, u repository.User) (repository.User, error) {
	return u, nil
}
func (r *UserRepo) CreateWithCanva(_ context.Context, u repository.User) (repository.User, error) {
	return u, nil
}
func (r *UserRepo) LinkGoogle(_ context.Context, u repository.User) (repository.User, error) {
	return u, nil
}
func (r *UserRepo) LinkOIDC(_ context.Context, u repository.User) (repository.User, error) {
	return u, nil
}
func (r *UserRepo) LinkCanva(_ context.Context, u repository.User) (repository.User, error) {
	return u, nil
}
func (r *UserRepo) UnlinkGoogle(_ context.Context, u repository.User) (repository.User, error) {
	return u, nil
}
func (r *UserRepo) UnlinkOIDC(_ context.Context, u repository.User) (repository.User, error) {
	return u, nil
}
func (r *UserRepo) UnlinkCanva(_ context.Context, u repository.User) (repository.User, error) {
	return u, nil
}
func (r *UserRepo) ListWorkspaceIDs(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}
func (r *UserRepo) RunInTx(_ context.Context, fn func(repository.UserRepository) error) error {
	return fn(r)
}

// OAuthRepo ----------------------------------------------------------------

type OAuthRepo struct{}

func NewOAuthRepo() *OAuthRepo { return &OAuthRepo{} }

func (r *OAuthRepo) List(_ context.Context, _ string) ([]repository.OAuthConnection, error) {
	return nil, nil
}
func (r *OAuthRepo) GetByID(_ context.Context, _, _ string) (repository.OAuthConnection, error) {
	return repository.OAuthConnection{}, apperr.ErrNotFound
}
func (r *OAuthRepo) GetByProviderUserID(_ context.Context, _, _, _ string) (repository.OAuthConnection, error) {
	return repository.OAuthConnection{}, apperr.ErrNotFound
}
func (r *OAuthRepo) Create(_ context.Context, _ repository.OAuthConnection) error { return nil }
func (r *OAuthRepo) UpdateTokens(_ context.Context, _, _ string, _ *string, _ *string) error {
	return nil
}
func (r *OAuthRepo) Delete(_ context.Context, _, _ string) error { return nil }
