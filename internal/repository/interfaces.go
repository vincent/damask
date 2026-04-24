package repository

import "context"

// AssetRepository handles persistence for Asset records.
type AssetRepository interface {
	GetByID(ctx context.Context, workspaceID, id string) (Asset, error)
	List(ctx context.Context, params ListAssetsParams) ([]Asset, error)
	Create(ctx context.Context, params CreateAssetParams) (Asset, error)
	Update(ctx context.Context, params UpdateAssetParams) (Asset, error)
	SoftDelete(ctx context.Context, workspaceID, id string) error
	// IsProjectCover reports whether the asset is set as the cover of any project.
	IsProjectCover(ctx context.Context, workspaceID, assetID string) (bool, error)
	// IsWorkspaceIcon reports whether the asset is set as the workspace icon.
	IsWorkspaceIcon(ctx context.Context, workspaceID, assetID string) (bool, error)
	// CountByIDs returns how many of the given IDs belong to the workspace.
	CountByIDs(ctx context.Context, workspaceID string, ids []string) (int64, error)
}

// ProjectRepository handles persistence for Project records.
type ProjectRepository interface {
	GetByID(ctx context.Context, workspaceID, id string) (Project, error)
	// List returns all projects with their asset counts.
	List(ctx context.Context, workspaceID string) ([]ProjectWithCount, error)
	Create(ctx context.Context, p Project) (Project, error)
	Update(ctx context.Context, p Project) (Project, error)
	// Delete removes the project row. The caller is responsible for calling
	// NullifyAssets inside the same transaction before calling Delete.
	Delete(ctx context.Context, workspaceID, id string) error
	// NullifyAssets sets project_id = NULL on all assets in the project.
	// Must be called inside a transaction before Delete.
	NullifyAssets(ctx context.Context, workspaceID, projectID string) error
}

// FolderRepository handles persistence for Folder records.
type FolderRepository interface {
	GetByID(ctx context.Context, workspaceID, id string) (Folder, error)
	ListByProject(ctx context.Context, workspaceID, projectID string) ([]Folder, error)
	Create(ctx context.Context, f Folder) (Folder, error)
	Update(ctx context.Context, f Folder) (Folder, error)
	Delete(ctx context.Context, workspaceID, id string) error
	// GetChildren returns direct children of the given folder.
	GetChildren(ctx context.Context, workspaceID, parentID string) ([]Folder, error)
	// NullifyAssets sets folder_id = NULL on all assets in the folder.
	NullifyAssets(ctx context.Context, workspaceID, folderID string) error
}

// TagRepository handles persistence for Tag records.
type TagRepository interface {
	GetByName(ctx context.Context, workspaceID, name string) (Tag, error)
	List(ctx context.Context, workspaceID string) ([]Tag, error)
	Upsert(ctx context.Context, workspaceID, name string) (Tag, error)
	UpdateMetadata(ctx context.Context, workspaceID, name string, color, groupName *string) error
	Rename(ctx context.Context, workspaceID, oldName, newName string) error
	Delete(ctx context.Context, workspaceID string, names []string) error
	ListForAsset(ctx context.Context, assetID string) ([]Tag, error)
	AddToAsset(ctx context.Context, assetID, tagID string) error
	RemoveFromAsset(ctx context.Context, workspaceID, assetID, tagName string) error
	// CountAssets returns the number of asset_tags rows for a given tag ID.
	CountAssets(ctx context.Context, tagID string) (int64, error)
	// ReassignAssets moves all asset_tags rows from fromTagID to toTagID (idempotent).
	ReassignAssets(ctx context.Context, fromTagID, toTagID string) error
	// TouchLastUsed updates last_used_at to now for the given tag (fire-and-forget).
	TouchLastUsed(ctx context.Context, workspaceID, name string) error
	// RunInTx executes fn inside a single database transaction.
	RunInTx(ctx context.Context, fn func(tx TagRepository) error) error
}

// CollectionRepository handles persistence for Collection records.
type CollectionRepository interface {
	GetByID(ctx context.Context, workspaceID, id string) (Collection, error)
	List(ctx context.Context, workspaceID string) ([]Collection, error)
	Create(ctx context.Context, c Collection) (Collection, error)
	Update(ctx context.Context, c Collection) (Collection, error)
	Delete(ctx context.Context, workspaceID, id string) error
	AddAsset(ctx context.Context, collectionID, assetID string) error
	RemoveAsset(ctx context.Context, collectionID, assetID string) error
	// ListForAsset returns all collections (with asset counts) that contain the asset.
	ListForAsset(ctx context.Context, workspaceID, assetID string) ([]Collection, error)
	// CountAssets returns the number of assets in the collection.
	CountAssets(ctx context.Context, collectionID string) (int64, error)
	// ListAssetIDs returns the asset IDs in the collection.
	ListAssetIDs(ctx context.Context, collectionID string) ([]string, error)
}

// ShareRepository handles persistence for Share records.
type ShareRepository interface {
	GetByID(ctx context.Context, workspaceID, id string) (Share, error)
	List(ctx context.Context, workspaceID string) ([]Share, error)
	Create(ctx context.Context, s Share) (Share, error)
	Update(ctx context.Context, s Share) (Share, error)
	Revoke(ctx context.Context, workspaceID, id string) error
}

// VersionRepository handles persistence for AssetVersion records.
type VersionRepository interface {
	GetByID(ctx context.Context, id string) (AssetVersion, error)
	// GetByIDForWorkspace returns the version only if it belongs to the workspace.
	GetByIDForWorkspace(ctx context.Context, workspaceID, id string) (AssetVersion, error)
	// GetCurrentByAsset returns the current (active) version for an asset.
	GetCurrentByAsset(ctx context.Context, assetID string) (AssetVersion, error)
	ListByAsset(ctx context.Context, assetID string) ([]AssetVersion, error)
	Create(ctx context.Context, v AssetVersion) (AssetVersion, error)
	// SoftDelete marks the version as deleted without removing the storage file.
	SoftDelete(ctx context.Context, id string) error
	// Delete hard-deletes the version row (used by retention job).
	Delete(ctx context.Context, id string) error
	CountByAsset(ctx context.Context, assetID string) (int64, error)
	// IsReferencedAsCover returns true when the version is set as a project cover or workspace icon.
	IsReferencedAsCover(ctx context.Context, versionID string) (bool, error)
	// GetByHash returns a version matching assetID + contentHash, or ErrNotFound.
	GetByHash(ctx context.Context, assetID, contentHash string) (AssetVersion, error)
	// NextVersionNum returns MAX(version_num)+1 for the asset (1 if no versions exist).
	NextVersionNum(ctx context.Context, assetID string) (int64, error)
	// SetCurrent atomically promotes versionID to current for assetID.
	SetCurrent(ctx context.Context, assetID, versionID string) error
	// SetAssetThumbnail updates assets.thumbnail_key for the given asset.
	SetAssetThumbnail(ctx context.Context, assetID string, key *string) error
	// ListWithVariantCount returns versions with per-version variant counts.
	ListWithVariantCount(ctx context.Context, assetID string) ([]AssetVersionWithCount, error)
}

// FieldRepository handles persistence for FieldDefinition records.
type FieldRepository interface {
	GetByID(ctx context.Context, workspaceID, id string) (FieldDefinition, error)
	List(ctx context.Context, workspaceID, scope string) ([]FieldDefinition, error)
	Create(ctx context.Context, f FieldDefinition) (FieldDefinition, error)
	Update(ctx context.Context, f FieldDefinition) (FieldDefinition, error)
	SoftDelete(ctx context.Context, workspaceID, id string) error
	CountByWorkspaceAndScope(ctx context.Context, workspaceID, scope string) (int64, error)
	CountAssetValues(ctx context.Context, fieldID string) (int64, error)
	CountProjectValues(ctx context.Context, fieldID string) (int64, error)
	UpdatePosition(ctx context.Context, workspaceID, id string, position int64) error
}

// AssetFieldRepository handles persistence for asset field values.
type AssetFieldRepository interface {
	GetValues(ctx context.Context, assetID string) ([]FieldValue, error)
	DeleteValue(ctx context.Context, assetID, fieldID string) error
	UpsertValue(ctx context.Context, assetID string, p SetFieldValueParams) error
	RunInTx(ctx context.Context, fn func(tx AssetFieldRepository) error) error
}

// ProjectFieldRepository handles persistence for project field values.
type ProjectFieldRepository interface {
	GetValues(ctx context.Context, projectID string) ([]FieldValue, error)
	DeleteValue(ctx context.Context, projectID, fieldID string) error
	UpsertValue(ctx context.Context, projectID string, p SetFieldValueParams) error
}

// VariantRepository handles persistence for asset variant records.
type VariantRepository interface {
	GetByID(ctx context.Context, workspaceID, id string) (Variant, error)
	ListByAsset(ctx context.Context, workspaceID, assetID string) ([]Variant, error)
	Create(ctx context.Context, v Variant) (Variant, error)
	Delete(ctx context.Context, workspaceID, id string) error
}

// WorkspaceRepository handles persistence for Workspace records.
type WorkspaceRepository interface {
	GetByID(ctx context.Context, id string) (Workspace, error)
	Create(ctx context.Context, w Workspace) (Workspace, error)
	Update(ctx context.Context, w Workspace) (Workspace, error)
	CountAssets(ctx context.Context, workspaceID string) (int64, error)
	// Member methods
	GetMember(ctx context.Context, workspaceID, userID string) (Member, error)
	ListMembers(ctx context.Context, workspaceID string) ([]Member, error)
	CountMembers(ctx context.Context, workspaceID string) (int64, error)
	CreateMember(ctx context.Context, m Member) error
	DeleteMember(ctx context.Context, workspaceID, userID string) error
	UpdateMemberRole(ctx context.Context, workspaceID, userID, role string) error
	// Invite methods
	CreateInvite(ctx context.Context, inv Invite) (Invite, error)
	ListPendingInvites(ctx context.Context, workspaceID string) ([]Invite, error)
	GetInviteByToken(ctx context.Context, token string) (Invite, error)
	DeleteInvite(ctx context.Context, workspaceID, inviteID string) error
	AcceptInvite(ctx context.Context, inviteID string) error
	// Workspace list for user
	ListByUserID(ctx context.Context, userID string) ([]WorkspaceWithRole, error)
	// RunInTx executes fn inside a single database transaction.
	// The WorkspaceRepository passed to fn is scoped to that transaction.
	RunInTx(ctx context.Context, fn func(tx WorkspaceRepository) error) error
	// RunRegistrationTx executes fn with tx-scoped UserRepository and WorkspaceRepository
	// sharing the same underlying database transaction. Used only by UserService.Register.
	RunRegistrationTx(ctx context.Context, fn func(ctx context.Context, txUsers UserRepository, txWorkspaces WorkspaceRepository) error) error
}

// UserRepository handles persistence for User records.
type UserRepository interface {
	GetByID(ctx context.Context, id string) (User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
	Create(ctx context.Context, u User) (User, error)
	Update(ctx context.Context, u User) (User, error)
	// RunInTx executes fn inside a single database transaction.
	RunInTx(ctx context.Context, fn func(tx UserRepository) error) error
}
