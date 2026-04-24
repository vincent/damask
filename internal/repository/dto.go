package repository

import "time"

// Asset is the domain representation of an uploaded file.
type Asset struct {
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
	CurrentVersionID *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
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

// FieldDefinition is the domain representation of a custom field definition.
type FieldDefinition struct {
	ID                 string
	WorkspaceID        string
	CreatedBy          string
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
	CreatedAt                time.Time
	UpdatedAt                time.Time
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
	AvatarUrl    *string
	AuthMethods  string
}

// Variant is the domain representation of an asset variant (transformed version).
type Variant struct {
	ID              string
	WorkspaceID     string
	AssetVersionID  string
	Type            string
	StorageKey      string
	TransformParams *string
	Size            *int64
	CreatedAt       time.Time
}

// ProjectWithCount is a Project enriched with its asset count.
type ProjectWithCount struct {
	Project
	AssetCount int64
}

// ListAssetsParams holds filters for listing assets.
// ProjectID and MimePrefix accept nil to mean "all".
// CursorAt and CursorID implement keyset pagination (created_at + id).
type ListAssetsParams struct {
	WorkspaceID string
	ProjectID   interface{} // *string or nil
	MimePrefix  interface{} // *string or nil
	CursorAt    interface{} // *time.Time or nil
	CursorID    *string
	Limit       int64
}

// CreateAssetParams holds the fields needed to insert a new asset row.
type CreateAssetParams struct {
	ID               string
	WorkspaceID      string
	ProjectID        *string
	OriginalFilename string
	StorageKey       string
	MimeType         string
	Size             int64
	Width            *int64
	Height           *int64
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
