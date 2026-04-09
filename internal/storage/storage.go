package storage

import (
	"fmt"
	"io"
)

// Storage is the interface that wraps basic asset storage operations.
// LocalStorage is the default implementation. Swap for S3Storage at config level.
type Storage interface {
	Put(key string, r io.Reader) error
	Get(key string) (io.ReadCloser, error)
	Delete(key string) error
	List(prefix string) ([]string, error)
}

// VersionedVariantKey returns the canonical storage path for a variant that
// belongs to a specific asset version.
//
// Format: {workspaceID}/{assetID}/v{versionNum}/variants/{variantType}/{paramsHash}{ext}
//
// paramsHash is the first 8 hex characters of SHA-256(canonical JSON of transform_params).
// ext should include the leading dot, e.g. ".jpg". Pass "" for formats without an extension.
func VersionedVariantKey(workspaceID, assetID string, versionNum int64, variantType, paramsHash, ext string) string {
	return fmt.Sprintf("%s/%s/v%d/variants/%s/%s%s",
		workspaceID, assetID, versionNum, variantType, paramsHash, ext)
}
