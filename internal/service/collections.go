package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"

	"github.com/google/uuid"
)

// CollectionDTO is the output of CollectionService methods.
type CollectionDTO struct {
	ID          string
	WorkspaceID string
	Name        string
	Description string
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CreateCollectionParams is the input for CollectionService.Create.
type CreateCollectionParams struct {
	Name        string
	Description string
	CreatedBy   string
}

func (p *CreateCollectionParams) Validate() error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return fmt.Errorf("name is required: %w", apperr.ErrInvalidInput)
	}
	return nil
}

// UpdateCollectionParams is the input for CollectionService.Update.
type UpdateCollectionParams struct {
	Name        *string
	Description *string
}

func (p *UpdateCollectionParams) Validate() error {
	if p.Name != nil {
		*p.Name = strings.TrimSpace(*p.Name)
		if *p.Name == "" {
			return fmt.Errorf("name cannot be empty: %w", apperr.ErrInvalidInput)
		}
	}
	return nil
}

type collectionService struct {
	collections repository.CollectionRepository
}

// NewCollectionService returns a CollectionService.
func NewCollectionService(collections repository.CollectionRepository) CollectionService {
	return &collectionService{collections: collections}
}

func (s *collectionService) List(ctx context.Context, workspaceID string) ([]*CollectionDTO, error) {
	rows, err := s.collections.List(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]*CollectionDTO, len(rows))
	for i, r := range rows {
		out[i] = toCollectionDTO(r)
	}
	return out, nil
}

func (s *collectionService) Get(ctx context.Context, workspaceID, id string) (*CollectionDTO, error) {
	col, err := s.collections.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	return toCollectionDTO(col), nil
}

func (s *collectionService) Create(ctx context.Context, workspaceID string, p CreateCollectionParams) (*CollectionDTO, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	col, err := s.collections.Create(ctx, repository.Collection{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		Name:        p.Name,
		Description: p.Description,
		CreatedBy:   p.CreatedBy,
	})
	if err != nil {
		return nil, err
	}
	return toCollectionDTO(col), nil
}

func (s *collectionService) Update(ctx context.Context, workspaceID, id string, p UpdateCollectionParams) (*CollectionDTO, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	existing, err := s.collections.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if p.Name != nil {
		existing.Name = *p.Name
	}
	if p.Description != nil {
		existing.Description = *p.Description
	}
	updated, err := s.collections.Update(ctx, existing)
	if err != nil {
		return nil, err
	}
	return toCollectionDTO(updated), nil
}

func (s *collectionService) Delete(ctx context.Context, workspaceID, id string) error {
	if _, err := s.collections.GetByID(ctx, workspaceID, id); err != nil {
		return err
	}
	return s.collections.Delete(ctx, workspaceID, id)
}

func (s *collectionService) AddAsset(ctx context.Context, workspaceID, collectionID, assetID string) error {
	if _, err := s.collections.GetByID(ctx, workspaceID, collectionID); err != nil {
		return err
	}
	return s.collections.AddAsset(ctx, collectionID, assetID)
}

func (s *collectionService) RemoveAsset(ctx context.Context, workspaceID, collectionID, assetID string) error {
	if _, err := s.collections.GetByID(ctx, workspaceID, collectionID); err != nil {
		return err
	}
	return s.collections.RemoveAsset(ctx, collectionID, assetID)
}

func toCollectionDTO(c repository.Collection) *CollectionDTO {
	return &CollectionDTO{
		ID:          c.ID,
		WorkspaceID: c.WorkspaceID,
		Name:        c.Name,
		Description: c.Description,
		CreatedBy:   c.CreatedBy,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}
