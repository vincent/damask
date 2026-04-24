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

const maxFieldDefinitionsPerScope = 50

// FieldDefinitionDTO is the output of FieldService methods.
type FieldDefinitionDTO struct {
	ID                 string
	WorkspaceID        string
	CreatedBy          string
	Scope              string
	Name               string
	Key                string
	FieldType          string
	Options            *string
	Required           bool
	Position           int64
	InheritFromProject bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
	DeletedAt          *string
}

// CreateFieldDefinitionParams is the input for FieldService.Create.
type CreateFieldDefinitionParams struct {
	CreatedBy          string
	Scope              string
	Name               string
	Key                string
	FieldType          string
	Options            *string
	Required           bool
	Position           int64
	InheritFromProject bool
}

func (p *CreateFieldDefinitionParams) Validate() error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return fmt.Errorf("name is required: %w", apperr.ErrInvalidInput)
	}
	p.Key = strings.TrimSpace(p.Key)
	if p.Key == "" {
		return fmt.Errorf("key is required: %w", apperr.ErrInvalidInput)
	}
	if p.Scope != "asset" && p.Scope != "project" {
		return fmt.Errorf("scope must be 'asset' or 'project': %w", apperr.ErrInvalidInput)
	}
	validTypes := map[string]bool{
		"text": true, "number": true, "date": true,
		"boolean": true, "select": true, "url": true,
	}
	if !validTypes[p.FieldType] {
		return fmt.Errorf("invalid field_type %q: %w", p.FieldType, apperr.ErrInvalidInput)
	}
	if p.FieldType != "select" {
		p.Options = nil
	}
	return nil
}

// UpdateFieldDefinitionParams is the input for FieldService.Update.
// Key and FieldType are immutable -- passing a different value returns ErrInvalidInput.
type UpdateFieldDefinitionParams struct {
	Name               *string
	Key                *string
	FieldType          *string
	Options            *string
	Required           *bool
	Position           *int64
	InheritFromProject *bool
}

type fieldService struct {
	fields repository.FieldRepository
}

// NewFieldService returns a FieldService.
func NewFieldService(fields repository.FieldRepository) FieldService {
	return &fieldService{fields: fields}
}

func (s *fieldService) List(ctx context.Context, workspaceID, scope string) ([]*FieldDefinitionDTO, error) {
	rows, err := s.fields.List(ctx, workspaceID, scope)
	if err != nil {
		return nil, err
	}
	out := make([]*FieldDefinitionDTO, len(rows))
	for i, r := range rows {
		out[i] = toFieldDTO(r)
	}
	return out, nil
}

func (s *fieldService) Get(ctx context.Context, workspaceID, id string) (*FieldDefinitionDTO, error) {
	f, err := s.fields.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if f.DeletedAt != nil {
		return nil, fmt.Errorf("field definition %q: %w", id, apperr.ErrNotFound)
	}
	return toFieldDTO(f), nil
}

func (s *fieldService) Create(ctx context.Context, workspaceID string, p CreateFieldDefinitionParams) (*FieldDefinitionDTO, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	count, err := s.fields.CountByWorkspaceAndScope(ctx, workspaceID, p.Scope)
	if err != nil {
		return nil, err
	}
	if count >= maxFieldDefinitionsPerScope {
		return nil, fmt.Errorf("maximum of %d field definitions per scope reached: %w", maxFieldDefinitionsPerScope, apperr.ErrInvalidInput)
	}
	f, err := s.fields.Create(ctx, repository.FieldDefinition{
		ID:                 uuid.NewString(),
		WorkspaceID:        workspaceID,
		CreatedBy:          p.CreatedBy,
		Scope:              p.Scope,
		Name:               p.Name,
		Key:                p.Key,
		FieldType:          p.FieldType,
		Options:            p.Options,
		Required:           p.Required,
		Position:           p.Position,
		InheritFromProject: p.InheritFromProject,
	})
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, fmt.Errorf("a field with key %q already exists in this scope: %w", p.Key, apperr.ErrConflict)
		}
		return nil, err
	}
	return toFieldDTO(f), nil
}

func (s *fieldService) Update(ctx context.Context, workspaceID, id string, p UpdateFieldDefinitionParams) (*FieldDefinitionDTO, error) {
	existing, err := s.fields.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if existing.DeletedAt != nil {
		return nil, fmt.Errorf("field definition %q: %w", id, apperr.ErrNotFound)
	}

	// key and field_type are immutable
	if p.Key != nil && *p.Key != existing.Key {
		return nil, fmt.Errorf("key cannot be changed after creation: %w", apperr.ErrInvalidInput)
	}
	if p.FieldType != nil && *p.FieldType != existing.FieldType {
		return nil, fmt.Errorf("field_type cannot be changed after creation: %w", apperr.ErrInvalidInput)
	}

	if p.Name != nil {
		*p.Name = strings.TrimSpace(*p.Name)
		if *p.Name == "" {
			return nil, fmt.Errorf("name cannot be empty: %w", apperr.ErrInvalidInput)
		}
		existing.Name = *p.Name
	}
	if p.Options != nil {
		existing.Options = p.Options
	}
	if p.Required != nil {
		existing.Required = *p.Required
	}
	if p.Position != nil {
		existing.Position = *p.Position
	}
	if p.InheritFromProject != nil {
		existing.InheritFromProject = *p.InheritFromProject
	}

	updated, err := s.fields.Update(ctx, existing)
	if err != nil {
		return nil, err
	}
	return toFieldDTO(updated), nil
}

func (s *fieldService) Delete(ctx context.Context, workspaceID, id string) error {
	existing, err := s.fields.GetByID(ctx, workspaceID, id)
	if err != nil {
		return err
	}
	if existing.DeletedAt != nil {
		return fmt.Errorf("field definition %q: %w", id, apperr.ErrNotFound)
	}
	return s.fields.SoftDelete(ctx, workspaceID, id)
}

func toFieldDTO(f repository.FieldDefinition) *FieldDefinitionDTO {
	return &FieldDefinitionDTO{
		ID:                 f.ID,
		WorkspaceID:        f.WorkspaceID,
		CreatedBy:          f.CreatedBy,
		Scope:              f.Scope,
		Name:               f.Name,
		Key:                f.Key,
		FieldType:          f.FieldType,
		Options:            f.Options,
		Required:           f.Required,
		Position:           f.Position,
		InheritFromProject: f.InheritFromProject,
		CreatedAt:          f.CreatedAt,
		UpdatedAt:          f.UpdatedAt,
		DeletedAt:          f.DeletedAt,
	}
}
