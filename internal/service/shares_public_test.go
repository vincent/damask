package service_test

import (
	"testing"
	"time"

	"damask/server/internal/repository"
	"damask/server/internal/service"
)

func TestIsShareExpiredDomain(t *testing.T) {
	t.Parallel()

	// nil ExpiresAt — never expires
	if service.IsShareExpiredDomain(repository.Share{}) {
		t.Error("nil ExpiresAt should not be expired")
	}

	// future RFC3339
	future := time.Now().Add(24 * time.Hour).UTC().Format("2006-01-02T15:04:05Z")
	if service.IsShareExpiredDomain(repository.Share{ExpiresAt: &future}) {
		t.Error("future date should not be expired")
	}

	// past RFC3339
	past := time.Now().Add(-24 * time.Hour).UTC().Format("2006-01-02T15:04:05Z")
	if !service.IsShareExpiredDomain(repository.Share{ExpiresAt: &past}) {
		t.Error("past RFC3339 date should be expired")
	}

	// past space-delimited format
	pastSpace := time.Now().Add(-24 * time.Hour).UTC().Format("2006-01-02 15:04:05")
	if !service.IsShareExpiredDomain(repository.Share{ExpiresAt: &pastSpace}) {
		t.Error("past space-delimited date should be expired")
	}

	// malformed date
	bad := "not-a-date"
	if service.IsShareExpiredDomain(repository.Share{ExpiresAt: &bad}) {
		t.Error("malformed date should not be expired")
	}
}

func TestToPublicAssetDTO(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)
	w := int64(1920)
	h := int64(1080)
	thumb := "thumb/key"
	meta := `{"foo":"bar"}`
	proj := "proj_1"
	folder := "folder_1"

	in := repository.PublicAsset{
		ID:               "ast_1",
		WorkspaceID:      "ws_1",
		ProjectID:        &proj,
		FolderID:         &folder,
		OriginalFilename: "photo.jpg",
		StorageKey:       "files/photo.jpg",
		MimeType:         "image/jpeg",
		Size:             1024,
		Width:            &w,
		Height:           &h,
		ThumbnailKey:     &thumb,
		Metadata:         &meta,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	got := service.ToPublicAssetDTO(in)
	if got.ID != in.ID || got.WorkspaceID != in.WorkspaceID || got.ProjectID != in.ProjectID {
		t.Errorf("ID/WorkspaceID/ProjectID mismatch: %+v", got)
	}
	if got.FolderID != in.FolderID || got.OriginalFilename != in.OriginalFilename {
		t.Errorf("FolderID/OriginalFilename mismatch: %+v", got)
	}
	if got.StorageKey != in.StorageKey || got.MimeType != in.MimeType || got.Size != in.Size {
		t.Errorf("StorageKey/MimeType/Size mismatch: %+v", got)
	}
	if got.Width != in.Width || got.Height != in.Height || got.ThumbnailKey != in.ThumbnailKey {
		t.Errorf("Width/Height/ThumbnailKey mismatch: %+v", got)
	}
	if got.Metadata != in.Metadata {
		t.Errorf("Metadata mismatch: %+v", got)
	}
}

func TestToCommentDTO(t *testing.T) {
	t.Parallel()

	email := "user@example.com"
	in := repository.ShareComment{
		ID:          "cmt_1",
		ShareID:     "share_1",
		AssetID:     "ast_1",
		AuthorName:  "Alice",
		AuthorEmail: &email,
		Body:        "Nice photo!",
		CreatedAt:   "2024-01-01T00:00:00Z",
	}

	got := service.ToCommentDTO(in)
	if got.ID != in.ID || got.ShareID != in.ShareID || got.AssetID != in.AssetID {
		t.Errorf("ID/ShareID/AssetID mismatch: %+v", got)
	}
	if got.AuthorName != in.AuthorName || got.AuthorEmail != in.AuthorEmail {
		t.Errorf("AuthorName/AuthorEmail mismatch: %+v", got)
	}
	if got.Body != in.Body || got.CreatedAt != in.CreatedAt {
		t.Errorf("Body/CreatedAt mismatch: %+v", got)
	}
}
