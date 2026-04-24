package service

import "time"

// ListAssetsParams holds filters for listing assets via AssetService.List.
type ListAssetsParams struct {
	WorkspaceID string
	ProjectID   *string
	MimePrefix  *string
	CursorAt    *time.Time
	CursorID    *string
	Limit       int64
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
