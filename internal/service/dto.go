package service

import "time"

// ListAssetsParams holds filters for listing assets via AssetService.List.
type ListAssetsParams struct {
	WorkspaceID string
	// Filters
	ProjectID    *string
	FolderID     *string // non-nil = filter by folder_id; use FolderIsRoot for root
	FolderIsRoot bool    // true = folder_id IS NULL (requires ProjectID)
	CollectionID *string
	TagNames     []string
	SearchQuery  string
	MimePrefix   *string
	// Sort: "created_at" (default), "size", "id", "taken_at"
	SortField string
	SortDesc  bool
	// Cursor (opaque; parsed by handler from cursor query param)
	CursorField string
	CursorValue string
	CursorID    string
	Limit       int64
}

// FieldFilter is a typed field[key][op]=value filter for asset listing.
type FieldFilter struct {
	Key      string
	Operator string // eq | lt | lte | gt | gte | contains | starts_with
	Value    string
}

// ListAssetsByFieldsParams holds parameters for field-filter-based asset listing.
type ListAssetsByFieldsParams struct {
	WorkspaceID  string
	FieldFilters []FieldFilter
	CursorAt     *string // raw cursor value (created_at string)
	CursorID     *string
	Limit        int64
}

// MoveAssetParams holds the destination for AssetService.Move.
// Nil fields mean "keep existing value". An empty-string pointer clears the field.
type MoveAssetParams struct {
	FolderID  *string
	ProjectID *string
}

// AssetDTO is the output of AssetService methods.
type AssetDTO struct {
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
