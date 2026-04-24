package service

import "context"

// AssetService handles business logic for asset records.
type AssetService interface {
	Get(ctx context.Context, workspaceID, assetID string) (*AssetDTO, error)
}
