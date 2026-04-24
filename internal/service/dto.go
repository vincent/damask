package service

import "time"

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
