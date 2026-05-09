package repository

import (
	"context"
	"time"
)

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
	// RefreshFTS rebuilds the FTS5 index entry for the asset, including its text field values.
	RefreshFTS(ctx context.Context, assetID string) error
	// ListByFields returns assets matching the given field filters (JOIN-per-filter pattern).
	ListByFields(ctx context.Context, params ListAssetsByFieldsParams) ([]Asset, error)
	// CollectStorageKeys returns all storage keys (asset + versions + variants) for an asset.
	CollectStorageKeys(ctx context.Context, workspaceID, assetID string) (AssetStorageKeys, error)
	// HardDelete permanently deletes the asset DB row (assumes storage cleanup is done by caller).
	HardDelete(ctx context.Context, workspaceID, assetID string) error
	// CountVersionsByAsset returns the number of non-deleted versions for an asset.
	CountVersionsByAsset(ctx context.Context, assetID string) (int64, error)
	// CountVariantsByCurrentVersion returns the number of variants on the asset's current version.
	CountVariantsByCurrentVersion(ctx context.Context, assetID string) (int64, error)
	// IsRebuildingVariants reports whether a rebuild_variants job is pending/processing for the given version.
	IsRebuildingVariants(ctx context.Context, versionID string) (bool, error)
	// ListComments returns all share comments posted on the asset.
	ListComments(ctx context.Context, assetID string) ([]AssetComment, error)
	// BatchVersionCounts returns version counts keyed by asset ID.
	BatchVersionCounts(ctx context.Context, assetIDs []string) (map[string]int64, error)
	// BatchVariantCounts returns variant counts (current version) keyed by asset ID.
	BatchVariantCounts(ctx context.Context, assetIDs []string) (map[string]int64, error)
	// SetProject assigns or clears the project for an asset (projectID = nil clears it).
	SetProject(ctx context.Context, workspaceID, assetID string, projectID *string) error
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
	// ListTree returns a recursive tree of folders for a project, each with asset count. Depth ≤ 2.
	ListTree(ctx context.Context, workspaceID, projectID string) ([]FolderTree, error)
}

// TagRepository handles persistence for Tag records.
type TagRepository interface {
	GetByName(ctx context.Context, workspaceID, name string) (Tag, error)
	List(ctx context.Context, workspaceID string, includeSystem bool) ([]Tag, error)
	Upsert(ctx context.Context, workspaceID, name string) (Tag, error)
	EnsureSystemTag(ctx context.Context, workspaceID, name string) error
	UpdateMetadata(ctx context.Context, workspaceID, name string, color, groupName *string) error
	Rename(ctx context.Context, workspaceID, oldName, newName string) error
	Delete(ctx context.Context, workspaceID string, names []string) error
	ListForAsset(ctx context.Context, assetID string) ([]Tag, error)
	AddToAsset(ctx context.Context, assetID, tagID string) error
	RemoveFromAsset(ctx context.Context, workspaceID, assetID, tagName string) error
	// BatchTagsForAssets returns tag names keyed by asset ID for the given set of asset IDs.
	BatchTagsForAssets(ctx context.Context, assetIDs []string) (map[string][]string, error)
	// CountAssets returns the number of asset_tags rows for a given tag ID.
	CountAssets(ctx context.Context, tagID string) (int64, error)
	// ReassignAssets moves all asset_tags rows from fromTagID to toTagID (idempotent).
	ReassignAssets(ctx context.Context, fromTagID, toTagID string) error
	// TouchLastUsed updates last_used_at to now for the given tag (fire-and-forget).
	TouchLastUsed(ctx context.Context, workspaceID, name string) error
	FindAssetBySystemTagInFolder(ctx context.Context, workspaceID, tagName, folderID string) (Asset, error)
	FindAssetBySystemTagInProject(ctx context.Context, workspaceID, tagName, projectID string) (Asset, error)
	FindAssetBySystemTagInWorkspace(ctx context.Context, workspaceID, tagName string) (Asset, error)
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
	// GetPublic returns a share by ID without workspace scoping (for public endpoints).
	GetPublic(ctx context.Context, id string) (Share, error)
	// GetByIDAndWorkspace returns a share only when it belongs to the workspace.
	GetByIDAndWorkspace(ctx context.Context, workspaceID, id string) (Share, error)
	List(ctx context.Context, workspaceID string) ([]Share, error)
	Create(ctx context.Context, s Share) (Share, error)
	Update(ctx context.Context, s Share) (Share, error)
	Revoke(ctx context.Context, workspaceID, id string) error
	IncrementViewCount(ctx context.Context, id string) error
	// Asset listing for public share endpoints (no workspace auth required).
	ListAssetsByTarget(ctx context.Context, targetType, targetID string) ([]PublicAsset, error)
	GetPublicAsset(ctx context.Context, assetID string) (PublicAsset, error)
	GetPublicAssetFile(ctx context.Context, assetID string) (PublicAssetFile, error)
	GetPublicAssetThumb(ctx context.Context, assetID string) (*string, time.Time, error)
	// IsAssetInTarget verifies the asset belongs to the share target.
	IsAssetInTarget(ctx context.Context, targetType, targetID, assetID string) (bool, error)
	// Comment methods.
	CreateComment(ctx context.Context, c ShareComment) (ShareComment, error)
	ListCommentsByShare(ctx context.Context, shareID string) ([]ShareComment, error)
	ListCommentsByShareAndAsset(ctx context.Context, shareID, assetID string) ([]ShareComment, error)
	DeleteComment(ctx context.Context, shareID, commentID string) error
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
	// InheritProjectFields copies inheritable project field values to a newly created asset.
	// It is a no-op when there are no inheritable definitions or the project has no values set.
	InheritProjectFields(ctx context.Context, workspaceID, assetID, projectID, userID string) error
	// GetByKey returns the field definition with the given key in the workspace, or ErrNotFound.
	GetByKey(ctx context.Context, workspaceID, key string) (FieldDefinition, error)
	// ListImageAssetIDs returns IDs of all image assets in the workspace.
	ListImageAssetIDs(ctx context.Context, workspaceID string) ([]string, error)
	// ListMissingExifField returns asset IDs that are image assets but lack a value for fieldID.
	ListMissingExifField(ctx context.Context, workspaceID, fieldID string, limit int64) ([]string, error)
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
	// GetByGoogleID returns the user linked to this Google sub ID.
	GetByGoogleID(ctx context.Context, googleUserID string) (User, error)
	// GetByCanvaID returns the user linked to this Canva user ID.
	GetByCanvaID(ctx context.Context, canvaUserID string) (User, error)
	// GetByOIDC returns the user linked to this OIDC issuer+sub pair.
	GetByOIDC(ctx context.Context, issuer, sub string) (User, error)
	// CreateWithGoogle creates a new user with a Google identity in one query.
	CreateWithGoogle(ctx context.Context, u User) (User, error)
	// CreateWithOIDC creates a new user with an OIDC identity in one query.
	CreateWithOIDC(ctx context.Context, u User) (User, error)
	// CreateWithCanva creates a new user with a Canva identity in one query.
	CreateWithCanva(ctx context.Context, u User) (User, error)
	// LinkGoogle sets google_user_id + avatar_url + auth_methods on an existing user.
	LinkGoogle(ctx context.Context, u User) (User, error)
	// LinkOIDC sets oidc_issuer + oidc_sub + avatar_url + auth_methods on an existing user.
	LinkOIDC(ctx context.Context, u User) (User, error)
	// LinkCanva sets canva_user_id + avatar_url + auth_methods on an existing user.
	LinkCanva(ctx context.Context, u User) (User, error)
	// UnlinkGoogle clears google_user_id and updates auth_methods.
	UnlinkGoogle(ctx context.Context, u User) (User, error)
	// UnlinkOIDC clears oidc_sub/oidc_issuer and updates auth_methods.
	UnlinkOIDC(ctx context.Context, u User) (User, error)
	// UnlinkCanva clears canva_user_id and updates auth_methods.
	UnlinkCanva(ctx context.Context, u User) (User, error)
	// ListWorkspaceIDs returns the workspace IDs the user belongs to (ordered by join date).
	ListWorkspaceIDs(ctx context.Context, userID string) ([]string, error)
	// RunInTx executes fn inside a single database transaction.
	RunInTx(ctx context.Context, fn func(tx UserRepository) error) error
}

// OAuthConnectionRepository handles persistence for oauth_connections rows.
type OAuthConnectionRepository interface {
	List(ctx context.Context, workspaceID string) ([]OAuthConnection, error)
	GetByID(ctx context.Context, workspaceID, id string) (OAuthConnection, error)
	GetByProviderUserID(ctx context.Context, workspaceID, provider, providerUserID string) (OAuthConnection, error)
	Create(ctx context.Context, c OAuthConnection) error
	UpdateTokens(ctx context.Context, id, accessToken string, refreshToken *string, expiresAt *string) error
	Delete(ctx context.Context, workspaceID, id string) error
}
