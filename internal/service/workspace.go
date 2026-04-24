package service

import (
	"context"
	"time"

	"damask/server/internal/repository"
)

// WorkspaceDTO is the output of WorkspaceService methods.
type WorkspaceDTO struct {
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

// UpdateWorkspaceParams is the input for WorkspaceService.Update.
type UpdateWorkspaceParams struct {
	VersionRetentionCount *int64
	ExifKeep              *bool
	ExifKeepGps           *bool
}

type workspaceService struct {
	workspaces repository.WorkspaceRepository
}

// NewWorkspaceService returns a WorkspaceService.
func NewWorkspaceService(workspaces repository.WorkspaceRepository) WorkspaceService {
	return &workspaceService{workspaces: workspaces}
}

func (s *workspaceService) Get(ctx context.Context, workspaceID string) (*WorkspaceDTO, error) {
	ws, err := s.workspaces.GetByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return toWorkspaceDTO(ws), nil
}

func (s *workspaceService) Update(ctx context.Context, workspaceID string, p UpdateWorkspaceParams) (*WorkspaceDTO, error) {
	existing, err := s.workspaces.GetByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if p.VersionRetentionCount != nil {
		existing.VersionRetentionCount = *p.VersionRetentionCount
	}
	if p.ExifKeep != nil {
		existing.ExifKeep = *p.ExifKeep
	}
	if p.ExifKeepGps != nil {
		existing.ExifKeepGps = *p.ExifKeepGps
	}
	updated, err := s.workspaces.Update(ctx, existing)
	if err != nil {
		return nil, err
	}
	return toWorkspaceDTO(updated), nil
}

func toWorkspaceDTO(ws repository.Workspace) *WorkspaceDTO {
	return &WorkspaceDTO{
		ID:                       ws.ID,
		Name:                     ws.Name,
		IngestToken:              ws.IngestToken,
		VersionRetentionCount:    ws.VersionRetentionCount,
		EventLogRetentionDays:    ws.EventLogRetentionDays,
		DownloadLogRetentionDays: ws.DownloadLogRetentionDays,
		IconAssetID:              ws.IconAssetID,
		IconVersionID:            ws.IconVersionID,
		ExifKeep:                 ws.ExifKeep,
		ExifKeepGps:              ws.ExifKeepGps,
		CreatedAt:                ws.CreatedAt,
		UpdatedAt:                ws.UpdatedAt,
	}
}
