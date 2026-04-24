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
	CountByIDs(ctx context.Context, workspaceID string, ids []string) (int64, error)
}

// CreateVariantParams is the input for VariantService.Create.
type CreateVariantParams struct {
	ID              string
	WorkspaceID     string
	AssetVersionID  string
	Type            string
	StorageKey      string
	TransformParams *string
	Size            *int64
}

// VariantService handles business logic for asset variant records.
type VariantService interface {
	List(ctx context.Context, workspaceID, assetID string) ([]*VariantDTO, error)
	Get(ctx context.Context, workspaceID, id string) (*VariantDTO, error)
	Create(ctx context.Context, p CreateVariantParams) (*VariantDTO, error)
	Delete(ctx context.Context, workspaceID, assetID, variantID string) error
}

// WorkspaceService handles business logic for workspace settings, members, and invites.
type WorkspaceService interface {
	Get(ctx context.Context, workspaceID string) (*WorkspaceDTO, error)
	Update(ctx context.Context, workspaceID string, p UpdateWorkspaceParams) (*WorkspaceDTO, error)
	Me(ctx context.Context, workspaceID, userID string) (*WorkspaceMeDTO, error)
	ListForUser(ctx context.Context, userID string) ([]WorkspaceWithRoleDTO, error)
	CountAssets(ctx context.Context, workspaceID string) (int64, error)
	// Members
	GetMember(ctx context.Context, workspaceID, userID string) (*MemberDTO, error)
	ListMembers(ctx context.Context, workspaceID string) ([]MemberDTO, error)
	RemoveMember(ctx context.Context, workspaceID, callerID, targetUserID string) error
	UpdateMemberRole(ctx context.Context, workspaceID, callerID, targetUserID string, role string) error
	// Invites
	CreateInvite(ctx context.Context, workspaceID, callerID string, p CreateInviteParams) (*InviteDTO, error)
	ListInvites(ctx context.Context, workspaceID string) ([]InviteDTO, error)
	DeleteInvite(ctx context.Context, workspaceID, inviteID string) error
	AcceptInvite(ctx context.Context, p AcceptInviteParams) (*AcceptInviteResult, error)
}

// VersionWithCountDTO is a VersionDTO enriched with its derived variant count.
type VersionWithCountDTO struct {
	VersionDTO
	VariantCount int64
}

// VersionService handles business logic for asset version records.
type VersionService interface {
	List(ctx context.Context, assetID string) ([]*VersionDTO, error)
	ListWithVariantCount(ctx context.Context, assetID string) ([]*VersionWithCountDTO, error)
	Get(ctx context.Context, workspaceID, id string) (*VersionDTO, error)
	GetCurrentByAsset(ctx context.Context, assetID string) (*VersionDTO, error)
	// GetByHash returns a version matching assetID + contentHash, or ErrNotFound.
	GetByHash(ctx context.Context, assetID, contentHash string) (*VersionDTO, error)
	// NextVersionNum returns the next version number for the asset.
	NextVersionNum(ctx context.Context, assetID string) (int64, error)
	// Create persists a new version row (is_current should be false; call SetCurrent after).
	Create(ctx context.Context, v *VersionDTO) (*VersionDTO, error)
	// SetCurrent atomically promotes versionID to current.
	SetCurrent(ctx context.Context, assetID, versionID string) error
	// SetAssetThumbnail updates assets.thumbnail_key.
	SetAssetThumbnail(ctx context.Context, assetID string, key *string) error
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
	GetStats(ctx context.Context, workspaceID, id string) (FieldStatsDTO, error)
	Reorder(ctx context.Context, workspaceID string, items []ReorderFieldItem) error
}

// FieldStatsDTO holds usage counts for a field definition.
type FieldStatsDTO struct {
	AssetCount   int64
	ProjectCount int64
}

// ReorderFieldItem is one entry in a reorder request.
type ReorderFieldItem struct {
	ID       string
	Position int64
}

// FieldValueDTO is the service-layer representation of a typed custom field value.
type FieldValueDTO struct {
	FieldID           string
	FieldKey          string
	FieldName         string
	FieldType         string
	FieldOptions      *string
	Value             interface{}
	DefinitionDeleted bool
}

// SetFieldValueInput is one entry in a patch-fields request.
type SetFieldValueInput struct {
	FieldID string
	Value   interface{}
}

// AssetFieldService handles business logic for asset field values.
type AssetFieldService interface {
	GetValues(ctx context.Context, workspaceID, assetID string) ([]*FieldValueDTO, error)
	SetValues(ctx context.Context, workspaceID, assetID, userID string, inputs []SetFieldValueInput) ([]*FieldValueDTO, error)
	BulkSetValues(ctx context.Context, workspaceID, userID string, assetIDs []string, inputs []SetFieldValueInput) (int64, error)
}

// ProjectFieldService handles business logic for project field values.
type ProjectFieldService interface {
	GetValues(ctx context.Context, workspaceID, projectID string) ([]*FieldValueDTO, error)
	SetValues(ctx context.Context, workspaceID, projectID, userID string, inputs []SetFieldValueInput) ([]*FieldValueDTO, error)
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
	ListForAsset(ctx context.Context, workspaceID, assetID string) ([]*CollectionDTO, error)
	ListAssets(ctx context.Context, workspaceID, collectionID string) ([]*AssetDTO, error)
}

// FolderService handles business logic for folder records.
type FolderService interface {
	Create(ctx context.Context, workspaceID, projectID string, p CreateFolderParams) (*FolderDTO, error)
	Get(ctx context.Context, workspaceID, id string) (*FolderDTO, error)
	List(ctx context.Context, workspaceID, projectID string) ([]*FolderDTO, error)
	Update(ctx context.Context, workspaceID, id string, p UpdateFolderParams) (*FolderDTO, error)
	Delete(ctx context.Context, workspaceID, id string) error
}

// BulkDeleteTagsResult carries the counts returned by TagService.BulkDelete.
type BulkDeleteTagsResult struct {
	Deleted           int
	RemovedFromAssets int64
}

// MergeTagsResult carries the outcome of TagService.Merge.
type MergeTagsResult struct {
	MergedAssets int64
	Target       *TagDTO
}

// TagService handles business logic for tag records.
type TagService interface {
	List(ctx context.Context, workspaceID string) ([]*TagDTO, error)
	Create(ctx context.Context, workspaceID string, p CreateTagParams) (*TagDTO, error)
	Patch(ctx context.Context, workspaceID, currentName string, p PatchTagParams) (*TagDTO, error)
	Delete(ctx context.Context, workspaceID string, names []string) error
	// BulkDelete deletes the named tags atomically and returns counts.
	BulkDelete(ctx context.Context, workspaceID string, names []string) (BulkDeleteTagsResult, error)
	// Merge reassigns all assets from sources to target (creating it if needed), then deletes sources.
	Merge(ctx context.Context, workspaceID string, sources []string, target string) (MergeTagsResult, error)
	// TouchLastUsed updates last_used_at for the named tag (fire-and-forget).
	TouchLastUsed(ctx context.Context, workspaceID, name string) error
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

// UserService handles business logic for user registration and login.
type UserService interface {
	Register(ctx context.Context, p RegisterUserParams) (*RegisterUserResult, error)
	Login(ctx context.Context, p LoginUserParams) (*LoginUserResult, error)
	GetByID(ctx context.Context, userID string) (*UserDTO, error)
	// CreateWorkspace creates a new workspace owned by userID inside a transaction.
	CreateWorkspace(ctx context.Context, userID, name string) (*WorkspaceDTO, error)
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
