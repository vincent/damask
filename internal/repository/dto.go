package repository

import "time"

// Asset is the domain representation of an uploaded file.
type Asset struct {
	ID                   string
	WorkspaceID          string
	ProjectID            *string
	FolderID             *string
	DerivedFromAssetID   *string
	OriginalFilename     string
	StorageKey           string
	MimeType             string
	Size                 int64
	Width                *int64
	Height               *int64
	ThumbnailKey         *string
	ThumbnailContentType string
	Metadata             *string
	CurrentVersionID     *string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// Project is the domain representation of a project.
type Project struct {
	ID             string
	WorkspaceID    string
	Name           string
	Description    *string
	Color          *string
	CoverAssetID   *string
	CoverVersionID *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Folder is the domain representation of a folder.
type Folder struct {
	ID          string
	WorkspaceID string
	ProjectID   string
	ParentID    *string
	Name        string
	Slug        *string
	Position    int64
	CreatedAt   time.Time
}

// FolderTree is a folder with asset count and pre-built children list.
type FolderTree struct {
	Folder
	AssetCount int64
	Children   []FolderTree
}

// AssetComment is the domain representation of a comment posted on an asset via a share.
type AssetComment struct {
	ID          string
	AssetID     string
	ShareID     string
	AuthorName  string
	AuthorEmail *string
	Body        string
	CreatedAt   time.Time
}

// AssetStorageKeys holds all storage keys for an asset and its versions + variants.
// Used to clean up storage after a hard delete.
type AssetStorageKeys struct {
	AssetKey    string
	ThumbKey    *string
	VersionKeys []VersionStorageKeys
}

// VersionStorageKeys holds the storage keys for one asset version and its variants.
type VersionStorageKeys struct {
	StorageKey           string
	ThumbnailKey         *string
	VariantKeys          []string
	VariantThumbnailKeys []string
}

// Tag is the domain representation of a tag.
type Tag struct {
	ID          string
	WorkspaceID string
	Name        string
	Color       *string
	GroupName   *string
	AssetCount  int64
	CreatedAt   time.Time
	LastUsedAt  *time.Time
}

// Collection is the domain representation of a collection.
type Collection struct {
	ID          string
	WorkspaceID string
	Name        string
	Description string
	CreatedBy   string
	AssetCount  int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Share is the domain representation of a share link.
type Share struct {
	ID            string
	WorkspaceID   string
	CreatedBy     string
	Label         string
	TargetType    string
	TargetID      string
	PasswordHash  *string
	ExpiresAt     *string
	AllowComments bool
	AllowDownload bool
	ViewCount     int64
	CreatedAt     time.Time
	RevokedAt     *string
}

// AssetVersion is the domain representation of a versioned asset file.
type AssetVersion struct {
	ID           string
	AssetID      string
	WorkspaceID  string
	VersionNum   int64
	StorageKey   string
	ContentHash  string
	MimeType     string
	Size         int64
	Width        *int64
	Height       *int64
	DurationSec  *float64
	ThumbnailKey *string
	Comment      *string
	CreatedBy    *string
	CreatedAt    time.Time
	IsCurrent    bool
	DeletedAt    *string
}

// AssetVersionWithCount is an AssetVersion enriched with its derived variant count.
type AssetVersionWithCount struct {
	AssetVersion
	VariantCount int64
}

// FieldDefinition is the domain representation of a custom field definition.
type FieldDefinition struct {
	ID                 string
	WorkspaceID        string
	CreatedBy          string
	Source             string
	Scope              string
	Name               string
	Key                string
	FieldType          string
	Options            *string
	Required           bool
	Position           int64
	InheritFromProject bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
	DeletedAt          *string
}

// Workspace is the domain representation of a workspace.
type Workspace struct {
	ID                       string
	Name                     string
	IngestToken              *string
	VersionRetentionCount    int64
	EventLogRetentionDays    int64
	DownloadLogRetentionDays int64
	IconAssetID              *string
	IconVersionID            *string
	ExifKeep                 bool
	ExifKeepGps              bool
	LockedTaxonomy           bool
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

// WorkspaceWithRole is a Workspace enriched with the requesting user's role.
type WorkspaceWithRole struct {
	Workspace
	Role string
}

// Member is the domain representation of a workspace member.
type Member struct {
	WorkspaceID string
	UserID      string
	Email       string
	Name        string
	Role        string
	InvitedBy   *string
	JoinedAt    time.Time
}

// Invite is the domain representation of a workspace invite.
type Invite struct {
	ID          string
	WorkspaceID string
	Email       string
	Token       string
	Role        string
	InvitedBy   string
	ExpiresAt   time.Time
	AcceptedAt  *time.Time
	CreatedAt   time.Time
}

// User is the domain representation of a user.
type User struct {
	ID           string
	Email        string
	PasswordHash string
	Name         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	OidcSub      *string
	OidcIssuer   *string
	GoogleUserID *string
	CanvaUserID  *string
	AvatarUrl    *string
	AvatarStorageKey *string
	AuthMethods  string
	PendingEmail *string
	DisplayName  *string
	DeletedAt    *string
}

// ShareComment is the domain representation of a public share comment.
type ShareComment struct {
	ID          string
	ShareID     string
	AssetID     string
	AuthorName  string
	AuthorEmail *string
	Body        string
	CreatedAt   string
}

// PublicAsset is a minimal asset view used by public share endpoints (no workspace isolation needed).
type PublicAsset struct {
	ID               string
	WorkspaceID      string
	ProjectID        *string
	FolderID         *string
	OriginalFilename string
	StorageKey       string
	MimeType         string
	Size             int64
	Width            *int64
	Height           *int64
	ThumbnailKey     *string
	Metadata         *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// PublicAssetFile holds the file-serving data for a shared asset.
type PublicAssetFile struct {
	MimeType         string
	OriginalFilename string
	StorageKey       string
	ContentHash      string
	Size             int64
	VersionCreatedAt string
}

// OAuthConnection is the domain representation of a stored OAuth connection.
type OAuthConnection struct {
	ID             string
	WorkspaceID    string
	CreatedBy      string
	Provider       string
	ProviderUserID *string
	ProviderEmail  *string
	Scopes         string
	AccessToken    string
	RefreshToken   *string
	ExpiresAt      *string
	CreatedAt      string
	UpdatedAt      string
}

// Variant is the domain representation of an asset variant (transformed version).
type Variant struct {
	ID                   string
	WorkspaceID          string
	AssetVersionID       string
	Type                 string
	StorageKey           string
	TransformParams      *string
	Size                 *int64
	Status               string
	ThumbnailKey         *string
	ThumbnailContentType string
	CreatedAt            time.Time
}

// ProjectWithCount is a Project enriched with its asset count.
type ProjectWithCount struct {
	Project
	AssetCount int64
}

// FieldValue is the domain representation of a typed custom field value.
type FieldValue struct {
	FieldID           string
	FieldKey          string
	FieldName         string
	FieldType         string
	FieldSource       string
	FieldOptions      *string
	ValueText         *string
	ValueNumber       *float64
	ValueDate         *string
	ValueBoolean      *int64
	DefinitionDeleted bool
}

// SetFieldValueParams holds the parameters for setting a single field value.
type SetFieldValueParams struct {
	FieldID      string
	ValueText    *string
	ValueNumber  *float64
	ValueDate    *string
	ValueBoolean *int64
	CreatedBy    string
}

// FieldFilter represents a parsed field[key][op]=value query filter for asset listing.
type FieldFilter struct {
	Key      string // field key slug (e.g. "client_name")
	Operator string // eq | lt | lte | gt | gte | contains | starts_with
	Value    string
}

// ListAssetsParams holds filters for listing assets.
// The List method builds a dynamic SQL query from these fields.
type ListAssetsParams struct {
	WorkspaceID string
	// Project / folder filters (mutually exclusive with TagNames/SearchQuery)
	ProjectID    *string
	FolderID     *string // non-nil = filter by folder; use FolderIsRoot for root filter
	FolderIsRoot bool    // true = folder_id IS NULL, requires ProjectID
	CollectionID *string // filter to assets in this collection
	// Tag filter (AND logic)
	TagNames []string
	// FTS search
	SearchQuery string
	// MIME prefix filter
	MimePrefix *string
	// Sort: "created_at" (default), "size", "id", "taken_at"
	SortField string
	SortDesc  bool // for taken_at: true = DESC (NULLs last always)
	// Cursor pagination — Field + Value encode the sort position; ID is tiebreaker
	CursorField string // "created_at" | "size" | "id"
	CursorValue string // stringified cursor value
	CursorID    string // UUID tiebreaker
	Limit       int64
	// ExifFieldID is required when SortField=="taken_at" (pre-looked-up field definition ID)
	ExifFieldID string
}

// ListAssetsByFieldsParams holds the parameters for field-filter-based asset listing.
type ListAssetsByFieldsParams struct {
	WorkspaceID  string
	FieldFilters []FieldFilter
	CursorAt     *string // raw cursor value (created_at string)
	CursorID     *string
	Limit        int64
}

// CreateAssetParams holds the fields needed to insert a new asset row.
type CreateAssetParams struct {
	ID               string
	WorkspaceID      string
	ProjectID        *string
	FolderID         *string
	DerivedFromAssetID *string
	OriginalFilename string
	StorageKey       string
	MimeType         string
	Size             int64
	Width            *int64
	Height           *int64
	ThumbnailKey     *string
	ThumbnailContentType string
	Metadata         *string
}

// UpdateAssetParams holds the fields that can be updated on an existing asset.
type UpdateAssetParams struct {
	ID               string
	WorkspaceID      string
	OriginalFilename *string
	ProjectID        *string
	FolderID         *string
	ThumbnailKey     *string
	Metadata         *string
	CurrentVersionID *string
	Width            *int64
	Height           *int64
}
