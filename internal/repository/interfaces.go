package repository

import "context"

// AssetRepository handles persistence for Asset records.
type AssetRepository interface {
	GetByID(ctx context.Context, workspaceID, id string) (Asset, error)
	List(ctx context.Context, params ListAssetsParams) ([]Asset, error)
	Create(ctx context.Context, params CreateAssetParams) (Asset, error)
	Update(ctx context.Context, params UpdateAssetParams) (Asset, error)
	SoftDelete(ctx context.Context, workspaceID, id string) error
}

// ProjectRepository handles persistence for Project records.
type ProjectRepository interface {
	GetByID(ctx context.Context, workspaceID, id string) (Project, error)
	List(ctx context.Context, workspaceID string) ([]Project, error)
	Create(ctx context.Context, p Project) (Project, error)
	Update(ctx context.Context, p Project) (Project, error)
	Delete(ctx context.Context, workspaceID, id string) error
}

// FolderRepository handles persistence for Folder records.
type FolderRepository interface {
	GetByID(ctx context.Context, workspaceID, id string) (Folder, error)
	ListByProject(ctx context.Context, workspaceID, projectID string) ([]Folder, error)
	Create(ctx context.Context, f Folder) (Folder, error)
	Update(ctx context.Context, f Folder) (Folder, error)
	Delete(ctx context.Context, workspaceID, id string) error
}

// TagRepository handles persistence for Tag records.
type TagRepository interface {
	GetByName(ctx context.Context, workspaceID, name string) (Tag, error)
	List(ctx context.Context, workspaceID string) ([]Tag, error)
	Upsert(ctx context.Context, workspaceID, name string) (Tag, error)
	Rename(ctx context.Context, workspaceID, oldName, newName string) error
	Delete(ctx context.Context, workspaceID string, names []string) error
	ListForAsset(ctx context.Context, assetID string) ([]Tag, error)
	AddToAsset(ctx context.Context, assetID, tagID string) error
	RemoveFromAsset(ctx context.Context, assetID, tagName string) error
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
	ListByAsset(ctx context.Context, assetID string) ([]AssetVersion, error)
	Create(ctx context.Context, v AssetVersion) (AssetVersion, error)
	Delete(ctx context.Context, id string) error
	CountByAsset(ctx context.Context, assetID string) (int64, error)
}

// FieldRepository handles persistence for FieldDefinition records.
type FieldRepository interface {
	GetByID(ctx context.Context, workspaceID, id string) (FieldDefinition, error)
	List(ctx context.Context, workspaceID, scope string) ([]FieldDefinition, error)
	Create(ctx context.Context, f FieldDefinition) (FieldDefinition, error)
	Update(ctx context.Context, f FieldDefinition) (FieldDefinition, error)
	SoftDelete(ctx context.Context, workspaceID, id string) error
	CountByWorkspaceAndScope(ctx context.Context, workspaceID, scope string) (int64, error)
}

// WorkspaceRepository handles persistence for Workspace records.
type WorkspaceRepository interface {
	GetByID(ctx context.Context, id string) (Workspace, error)
	Update(ctx context.Context, w Workspace) (Workspace, error)
}

// UserRepository handles persistence for User records.
type UserRepository interface {
	GetByID(ctx context.Context, id string) (User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
	Create(ctx context.Context, u User) (User, error)
	Update(ctx context.Context, u User) (User, error)
}
