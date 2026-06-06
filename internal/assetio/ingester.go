// Package assetio handles asset ingestion and I/O operations.
package assetio

import "context"

// FieldInheritanceFunc is called after asset creation to copy project field values.
type FieldInheritanceFunc func(ctx context.Context, workspaceID, assetID, projectID, userID string)

// IngestFileOpts holds optional destination fields for Ingester.IngestFile.
type IngestFileOpts struct {
	ProjectID     *string
	FolderID      *string
	UserID        string
	InheritFields FieldInheritanceFunc
	// OriginalName overrides the filename derived from filePath.
	OriginalName string
}

// AssetSummary is the minimal asset data returned by Ingester.IngestFile.
type AssetSummary struct {
	ID               string
	WorkspaceID      string
	StorageKey       string
	MimeType         string
	OriginalFilename string
}

// Ingester handles low-level asset creation from a file path.
type Ingester interface {
	IngestFile(ctx context.Context, workspaceID, filePath string, opts IngestFileOpts) (AssetSummary, error)
}
