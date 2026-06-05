package service

import (
	"context"
	"time"

	cache "github.com/go-pkgz/expirable-cache/v3"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/ingress"
)

// ErrStorageLimitReached is the same sentinel as ingress.ErrStorageLimitReached so
// callers in both packages can use errors.Is consistently.
var ErrStorageLimitReached = ingress.ErrStorageLimitReached

// AssetTypeBucket breaks usage down by asset type.
type AssetTypeBucket struct {
	Image    int64 `json:"image"`
	Video    int64 `json:"video"`
	Audio    int64 `json:"audio"`
	Document int64 `json:"document"`
	Other    int64 `json:"other"`
}

// ProjectStorageUsage is per-project usage within a workspace.
type ProjectStorageUsage struct {
	ProjectID     *string         `json:"project_id"`
	ProjectName   string          `json:"project_name"`
	VersionsBytes int64           `json:"versions_bytes"`
	VariantsBytes int64           `json:"variants_bytes"`
	TotalBytes    int64           `json:"total_bytes"`
	FolderCount   int64           `json:"folder_count"`
	ByType        AssetTypeBucket `json:"by_type"`
}

// FolderStorageUsage is per-folder usage within a project.
type FolderStorageUsage struct {
	FolderID      *string `json:"folder_id"`
	FolderName    string  `json:"folder_name"`
	VersionsBytes int64   `json:"versions_bytes"`
	VariantsBytes int64   `json:"variants_bytes"`
	TotalBytes    int64   `json:"total_bytes"`
}

// WorkspaceStorageUsage is the full cached usage breakdown for a workspace.
type WorkspaceStorageUsage struct {
	VersionsBytes int64                 `json:"versions_bytes"`
	VariantsBytes int64                 `json:"variants_bytes"`
	TotalBytes    int64                 `json:"total_bytes"`
	LimitBytes    *int64                `json:"limit_bytes"`
	ByProject     []ProjectStorageUsage `json:"by_project"`
	ByType        AssetTypeBucket       `json:"by_type"`
	ComputedAt    time.Time             `json:"computed_at"`
}

// StorageInvalidator is the narrow interface injected into mutation services.
// They only need to evict their workspace from the usage cache — nothing more.
type StorageInvalidator interface {
	Invalidate(workspaceID string)
}

// StorageService tracks and enforces workspace storage usage.
type StorageService interface {
	StorageInvalidator
	GetUsage(ctx context.Context, workspaceID string) (*WorkspaceStorageUsage, error)
	GetFolderUsage(ctx context.Context, workspaceID, projectID string) ([]FolderStorageUsage, error)
	CheckLimit(ctx context.Context, workspaceID string, incomingBytes int64) error
}

type storageService struct {
	db         *dbgen.Queries
	usageCache cache.Cache[string, *WorkspaceStorageUsage]
}

// NewStorageService constructs a StorageService with a 60-second TTL cache.
func NewStorageService(db *dbgen.Queries) StorageService {
	c := cache.NewCache[string, *WorkspaceStorageUsage]().
		WithMaxKeys(256).
		WithTTL(60 * time.Second)
	return &storageService{db: db, usageCache: c}
}

func (s *storageService) GetUsage(ctx context.Context, workspaceID string) (*WorkspaceStorageUsage, error) {
	if cached, ok := s.usageCache.Get(workspaceID); ok {
		return cached, nil
	}

	rows, err := s.db.GetStorageByProjectAndType(ctx, dbgen.GetStorageByProjectAndTypeParams{
		WorkspaceID:   workspaceID,
		WorkspaceID_2: workspaceID,
	})
	if err != nil {
		return nil, err
	}

	folderCounts, err := s.db.GetFolderCountsByProject(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	limitBytes, err := s.db.GetWorkspaceStorageLimitBytes(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	usage := buildUsage(rows, folderCounts, limitBytes)
	usage.ComputedAt = time.Now()
	s.usageCache.Add(workspaceID, usage)
	return usage, nil
}

func (s *storageService) GetFolderUsage(
	ctx context.Context,
	workspaceID, projectID string,
) ([]FolderStorageUsage, error) {
	rows, err := s.db.GetStorageByFolder(ctx, dbgen.GetStorageByFolderParams{
		WorkspaceID:   workspaceID,
		WorkspaceID_2: workspaceID,
		ProjectID:     &projectID,
	})
	if err != nil {
		return nil, err
	}

	result := make([]FolderStorageUsage, 0, len(rows))
	for _, r := range rows {
		vb := toInt64(r.VersionsBytes)
		varb := toInt64(r.VariantsBytes)
		result = append(result, FolderStorageUsage{
			FolderID:      toStringPtr(r.FolderID),
			FolderName:    r.FolderName,
			VersionsBytes: vb,
			VariantsBytes: varb,
			TotalBytes:    vb + varb,
		})
	}
	return result, nil
}

func (s *storageService) CheckLimit(ctx context.Context, workspaceID string, incomingBytes int64) error {
	usage, err := s.GetUsage(ctx, workspaceID)
	if err != nil {
		return err
	}
	if usage.LimitBytes == nil {
		return nil
	}
	if usage.TotalBytes+incomingBytes > *usage.LimitBytes {
		return ErrStorageLimitReached
	}
	return nil
}

func (s *storageService) Invalidate(workspaceID string) {
	s.usageCache.Remove(workspaceID)
}

// buildUsage aggregates flat DB rows into a WorkspaceStorageUsage.
// Pure function — no DB access — easy to unit-test.
func buildUsage(
	rows []dbgen.GetStorageByProjectAndTypeRow,
	folderCounts []dbgen.GetFolderCountsByProjectRow,
	limitBytes *int64,
) *WorkspaceStorageUsage {
	fcMap := make(map[string]int64, len(folderCounts))
	for _, fc := range folderCounts {
		if fc.ProjectID != nil {
			fcMap[*fc.ProjectID] = fc.FolderCount
		}
	}

	// projectID (string or "nil") → *ProjectStorageUsage
	byProject := map[string]*ProjectStorageUsage{}
	var projectOrder []string

	for _, r := range rows {
		key := "<nil>"
		if r.ProjectID != nil {
			key = *r.ProjectID
		}

		p, ok := byProject[key]
		if !ok {
			fc := int64(0)
			if r.ProjectID != nil {
				fc = fcMap[*r.ProjectID]
			}
			p = &ProjectStorageUsage{
				ProjectID:   r.ProjectID,
				ProjectName: r.ProjectName,
				FolderCount: fc,
			}
			byProject[key] = p
			projectOrder = append(projectOrder, key)
		}

		vb := toInt64(r.VersionsBytes)
		varb := toInt64(r.VariantsBytes)
		p.VersionsBytes += vb
		p.VariantsBytes += varb

		switch r.AssetType {
		case "image":
			p.ByType.Image += vb + varb
		case "video":
			p.ByType.Video += vb + varb
		case "audio":
			p.ByType.Audio += vb + varb
		case "document":
			p.ByType.Document += vb + varb
		default:
			p.ByType.Other += vb + varb
		}
	}

	usage := &WorkspaceStorageUsage{
		LimitBytes: limitBytes,
		ByProject:  make([]ProjectStorageUsage, 0, len(byProject)),
	}

	for _, key := range projectOrder {
		p := byProject[key]
		p.TotalBytes = p.VersionsBytes + p.VariantsBytes
		usage.VersionsBytes += p.VersionsBytes
		usage.VariantsBytes += p.VariantsBytes

		usage.ByType.Image += p.ByType.Image
		usage.ByType.Video += p.ByType.Video
		usage.ByType.Audio += p.ByType.Audio
		usage.ByType.Document += p.ByType.Document
		usage.ByType.Other += p.ByType.Other

		usage.ByProject = append(usage.ByProject, *p)
	}

	usage.TotalBytes = usage.VersionsBytes + usage.VariantsBytes
	return usage
}

// toStringPtr converts a SQLite CASE expression any result to *string.
func toStringPtr(v any) *string {
	if v == nil {
		return nil
	}
	if s, ok := v.(string); ok {
		return &s
	}
	return nil
}

// toInt64 converts SQLite COALESCE/SUM any results to int64.
func toInt64(v any) int64 {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	}
	return 0
}
