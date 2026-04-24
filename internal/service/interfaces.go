package service

import (
	"context"
	"io"
)

// AssetService handles business logic for asset records.
type AssetService interface {
	Get(ctx context.Context, workspaceID, assetID string) (*AssetDTO, error)
	List(ctx context.Context, params ListAssetsParams) ([]*AssetDTO, error)
	Move(ctx context.Context, workspaceID, assetID string, p MoveAssetParams) (*AssetDTO, error)
	Rename(ctx context.Context, workspaceID, assetID, newStem string) (*AssetDTO, error)
	Delete(ctx context.Context, workspaceID, assetID string) error
}

// VariantService handles business logic for asset variant records.
type VariantService interface {
	List(ctx context.Context, workspaceID, assetID string) ([]*VariantDTO, error)
	Get(ctx context.Context, workspaceID, id string) (*VariantDTO, error)
	Delete(ctx context.Context, workspaceID, assetID, variantID string) error
}

// WorkspaceService handles business logic for workspace settings.
type WorkspaceService interface {
	Get(ctx context.Context, workspaceID string) (*WorkspaceDTO, error)
	Update(ctx context.Context, workspaceID string, p UpdateWorkspaceParams) (*WorkspaceDTO, error)
}

// VersionService handles business logic for asset version records.
type VersionService interface {
	List(ctx context.Context, assetID string) ([]*VersionDTO, error)
	Get(ctx context.Context, workspaceID, id string) (*VersionDTO, error)
	Delete(ctx context.Context, workspaceID, assetID, versionID string) error
}

// ShareService handles business logic for share link records.
type ShareService interface {
	List(ctx context.Context, workspaceID string) ([]*ShareDTO, error)
	Get(ctx context.Context, workspaceID, id string) (*ShareDTO, error)
	Create(ctx context.Context, workspaceID string, p CreateShareParams) (*ShareDTO, error)
	Update(ctx context.Context, workspaceID, id string, p UpdateShareParams) (*ShareDTO, error)
	Revoke(ctx context.Context, workspaceID, id string) error
}

// FieldService handles business logic for custom field definitions.
type FieldService interface {
	List(ctx context.Context, workspaceID, scope string) ([]*FieldDefinitionDTO, error)
	Get(ctx context.Context, workspaceID, id string) (*FieldDefinitionDTO, error)
	Create(ctx context.Context, workspaceID string, p CreateFieldDefinitionParams) (*FieldDefinitionDTO, error)
	Update(ctx context.Context, workspaceID, id string, p UpdateFieldDefinitionParams) (*FieldDefinitionDTO, error)
	Delete(ctx context.Context, workspaceID, id string) error
}

// CollectionService handles business logic for collection records.
type CollectionService interface {
	List(ctx context.Context, workspaceID string) ([]*CollectionDTO, error)
	Get(ctx context.Context, workspaceID, id string) (*CollectionDTO, error)
	Create(ctx context.Context, workspaceID string, p CreateCollectionParams) (*CollectionDTO, error)
	Update(ctx context.Context, workspaceID, id string, p UpdateCollectionParams) (*CollectionDTO, error)
	Delete(ctx context.Context, workspaceID, id string) error
	AddAsset(ctx context.Context, workspaceID, collectionID, assetID string) error
	RemoveAsset(ctx context.Context, workspaceID, collectionID, assetID string) error
}

// FolderService handles business logic for folder records.
type FolderService interface {
	Create(ctx context.Context, workspaceID, projectID string, p CreateFolderParams) (*FolderDTO, error)
	Get(ctx context.Context, workspaceID, id string) (*FolderDTO, error)
	List(ctx context.Context, workspaceID, projectID string) ([]*FolderDTO, error)
	Update(ctx context.Context, workspaceID, id string, p UpdateFolderParams) (*FolderDTO, error)
	Delete(ctx context.Context, workspaceID, id string) error
}

// TagService handles business logic for tag records.
type TagService interface {
	List(ctx context.Context, workspaceID string) ([]*TagDTO, error)
	Create(ctx context.Context, workspaceID string, p CreateTagParams) (*TagDTO, error)
	Patch(ctx context.Context, workspaceID, currentName string, p PatchTagParams) (*TagDTO, error)
	Delete(ctx context.Context, workspaceID string, names []string) error
	ListForAsset(ctx context.Context, assetID string) ([]*TagDTO, error)
	AddToAsset(ctx context.Context, workspaceID, assetID, tagName string) (*TagDTO, error)
	RemoveFromAsset(ctx context.Context, workspaceID, assetID, tagName string) error
	UpsertForAsset(ctx context.Context, workspaceID, assetID, tagName string) error
}

// ProjectService handles business logic for project records.
type ProjectService interface {
	Create(ctx context.Context, workspaceID string, p CreateProjectParams) (*ProjectDTO, error)
	Get(ctx context.Context, workspaceID, id string) (*ProjectDTO, error)
	List(ctx context.Context, workspaceID string) ([]*ProjectDTO, error)
	Update(ctx context.Context, workspaceID, id string, p UpdateProjectParams) (*ProjectDTO, error)
	Delete(ctx context.Context, workspaceID, id string) error
}

// AuditLogService handles read access to the audit event log.
type AuditLogService interface {
	ListAssetEvents(ctx context.Context, p ListAssetEventsParams) (*AuditEventListDTO, error)
	ListProjectEvents(ctx context.Context, p ListProjectEventsParams) (*AuditEventListDTO, error)
	ListWorkspaceActivity(ctx context.Context, p ListWorkspaceActivityParams) (*ActivityListDTO, error)
	ExportActivity(ctx context.Context, p ExportActivityParams) (csv string, err error)
}

// UploadService handles asset ingestion from an io.Reader.
type UploadService interface {
	Ingest(ctx context.Context, workspaceID string, r io.Reader, meta UploadMeta) (*UploadedAssetDTO, error)
}

// StackService handles business logic for stack export and merge.
type StackService interface {
	ExportZip(ctx context.Context, workspaceID string, p ExportZipParams, w io.Writer) error
	EnqueueMerge(ctx context.Context, workspaceID, userID string, p MergeParams) (jobID string, err error)
}

// IngressService handles business logic for ingress sources, rules, and log entries.
type IngressService interface {
	// Sources
	ListSources(ctx context.Context, workspaceID string) ([]*IngressSourceDTO, error)
	GetSource(ctx context.Context, workspaceID, id string) (*IngressSourceDTO, error)
	CreateSource(ctx context.Context, workspaceID, userID string, p CreateIngressSourceParams) (*IngressSourceDTO, error)
	UpdateSource(ctx context.Context, workspaceID, id string, p UpdateIngressSourceParams) (*IngressSourceDTO, error)
	DeleteSource(ctx context.Context, workspaceID, id string) error
	TestSource(ctx context.Context, workspaceID, id string) error
	TriggerPoll(ctx context.Context, workspaceID, id string) (jobID string, err error)
	// Rules
	ListRules(ctx context.Context, workspaceID, sourceID string) ([]*IngressRuleDTO, error)
	CreateRule(ctx context.Context, workspaceID, sourceID string, p CreateIngressRuleParams) (*IngressRuleDTO, error)
	UpdateRule(ctx context.Context, workspaceID, sourceID, ruleID string, p UpdateIngressRuleParams) (*IngressRuleDTO, error)
	DeleteRule(ctx context.Context, workspaceID, sourceID, ruleID string) error
	ReorderRules(ctx context.Context, workspaceID, sourceID string, entries []ReorderRuleEntry) ([]*IngressRuleDTO, error)
	// Log
	ListLog(ctx context.Context, workspaceID, statusFilter string, limit, offset int64) ([]*IngressLogEntryDTO, error)
	ListSourceLog(ctx context.Context, workspaceID, sourceID string, limit, offset int64) ([]*IngressLogEntryDTO, error)
	DeleteLogEntry(ctx context.Context, workspaceID, entryID string) error
	RetryLogEntry(ctx context.Context, workspaceID, entryID string) (jobID string, err error)
}
