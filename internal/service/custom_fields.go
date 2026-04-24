package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"

	"github.com/google/uuid"
)

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

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

func (s *fieldService) GetStats(ctx context.Context, workspaceID, id string) (FieldStatsDTO, error) {
	if _, err := s.fields.GetByID(ctx, workspaceID, id); err != nil {
		return FieldStatsDTO{}, err
	}
	assetCount, err := s.fields.CountAssetValues(ctx, id)
	if err != nil {
		return FieldStatsDTO{}, err
	}
	projectCount, err := s.fields.CountProjectValues(ctx, id)
	if err != nil {
		return FieldStatsDTO{}, err
	}
	return FieldStatsDTO{AssetCount: assetCount, ProjectCount: projectCount}, nil
}

func (s *fieldService) Reorder(ctx context.Context, workspaceID string, items []ReorderFieldItem) error {
	for _, item := range items {
		_ = s.fields.UpdatePosition(ctx, workspaceID, item.ID, item.Position)
	}
	return nil
}

func (s *fieldService) InheritProjectFields(ctx context.Context, workspaceID, assetID, projectID, userID string) error {
	return s.fields.InheritProjectFields(ctx, workspaceID, assetID, projectID, userID)
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

// -- AssetFieldService --------------------------------------------------------

type assetFieldService struct {
	assets     repository.AssetRepository
	fields     repository.FieldRepository
	assetFields repository.AssetFieldRepository
}

// NewAssetFieldService returns an AssetFieldService.
func NewAssetFieldService(assets repository.AssetRepository, fields repository.FieldRepository, assetFields repository.AssetFieldRepository) AssetFieldService {
	return &assetFieldService{assets: assets, fields: fields, assetFields: assetFields}
}

func (s *assetFieldService) GetValues(ctx context.Context, workspaceID, assetID string) ([]*FieldValueDTO, error) {
	if _, err := s.assets.GetByID(ctx, workspaceID, assetID); err != nil {
		return nil, err
	}
	rows, err := s.assetFields.GetValues(ctx, assetID)
	if err != nil {
		return nil, err
	}
	return toFieldValueDTOs(rows), nil
}

func (s *assetFieldService) SetValues(ctx context.Context, workspaceID, assetID, userID string, inputs []SetFieldValueInput) ([]*FieldValueDTO, error) {
	if _, err := s.assets.GetByID(ctx, workspaceID, assetID); err != nil {
		return nil, err
	}
	for _, input := range inputs {
		def, err := s.fields.GetByID(ctx, workspaceID, input.FieldID)
		if err != nil {
			return nil, err
		}
		if def.DeletedAt != nil {
			return nil, fmt.Errorf("field %s has been deleted: %w", input.FieldID, apperr.ErrInvalidInput)
		}
		if input.Value == nil {
			if err := s.assetFields.DeleteValue(ctx, assetID, input.FieldID); err != nil {
				return nil, err
			}
			continue
		}
		p, err := resolveFieldValue(input.FieldID, def.FieldType, def.Options, input.Value)
		if err != nil {
			return nil, fmt.Errorf("%w", apperr.ErrInvalidInput)
		}
		p.CreatedBy = userID
		if err := s.assetFields.UpsertValue(ctx, assetID, p); err != nil {
			return nil, err
		}
	}
	rows, err := s.assetFields.GetValues(ctx, assetID)
	if err != nil {
		return nil, err
	}
	return toFieldValueDTOs(rows), nil
}

func (s *assetFieldService) BulkSetValues(ctx context.Context, workspaceID, userID string, assetIDs []string, inputs []SetFieldValueInput) (int64, error) {
	// Validate values once (same fields for all assets).
	type resolvedInput struct {
		fieldID string
		p       *repository.SetFieldValueParams
	}
	resolved := make([]resolvedInput, len(inputs))
	for i, input := range inputs {
		if input.Value == nil {
			resolved[i] = resolvedInput{fieldID: input.FieldID}
			continue
		}
		def, err := s.fields.GetByID(ctx, workspaceID, input.FieldID)
		if err != nil {
			return 0, err
		}
		if def.DeletedAt != nil {
			return 0, fmt.Errorf("field %s has been deleted: %w", input.FieldID, apperr.ErrInvalidInput)
		}
		p, err := resolveFieldValue(input.FieldID, def.FieldType, def.Options, input.Value)
		if err != nil {
			return 0, fmt.Errorf("%w", apperr.ErrInvalidInput)
		}
		p.CreatedBy = userID
		resolved[i] = resolvedInput{fieldID: input.FieldID, p: &p}
	}

	// Pre-filter: collect only asset IDs that belong to this workspace (read before tx).
	validIDs := make([]string, 0, len(assetIDs))
	for _, assetID := range assetIDs {
		if _, err := s.assets.GetByID(ctx, workspaceID, assetID); err == nil {
			validIDs = append(validIDs, assetID)
		}
	}

	var updatedCount int64
	err := s.assetFields.RunInTx(ctx, func(tx repository.AssetFieldRepository) error {
		for _, assetID := range validIDs {
			assetOK := true
			for _, r := range resolved {
				if r.p == nil {
					if err := tx.DeleteValue(ctx, assetID, r.fieldID); err != nil {
						assetOK = false
						break
					}
					continue
				}
				if err := tx.UpsertValue(ctx, assetID, *r.p); err != nil {
					return err
				}
			}
			if assetOK {
				updatedCount++
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return updatedCount, nil
}

// -- ProjectFieldService -------------------------------------------------------

type projectFieldService struct {
	projects     repository.ProjectRepository
	fields       repository.FieldRepository
	projectFields repository.ProjectFieldRepository
}

// NewProjectFieldService returns a ProjectFieldService.
func NewProjectFieldService(projects repository.ProjectRepository, fields repository.FieldRepository, projectFields repository.ProjectFieldRepository) ProjectFieldService {
	return &projectFieldService{projects: projects, fields: fields, projectFields: projectFields}
}

func (s *projectFieldService) GetValues(ctx context.Context, workspaceID, projectID string) ([]*FieldValueDTO, error) {
	if _, err := s.projects.GetByID(ctx, workspaceID, projectID); err != nil {
		return nil, err
	}
	rows, err := s.projectFields.GetValues(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return toFieldValueDTOs(rows), nil
}

func (s *projectFieldService) SetValues(ctx context.Context, workspaceID, projectID, userID string, inputs []SetFieldValueInput) ([]*FieldValueDTO, error) {
	if _, err := s.projects.GetByID(ctx, workspaceID, projectID); err != nil {
		return nil, err
	}
	for _, input := range inputs {
		def, err := s.fields.GetByID(ctx, workspaceID, input.FieldID)
		if err != nil {
			return nil, err
		}
		if def.Scope != "project" {
			return nil, fmt.Errorf("field %s is not a project field: %w", def.Key, apperr.ErrInvalidInput)
		}
		if input.Value == nil {
			if err := s.projectFields.DeleteValue(ctx, projectID, input.FieldID); err != nil {
				return nil, err
			}
			continue
		}
		p, err := resolveFieldValue(input.FieldID, def.FieldType, def.Options, input.Value)
		if err != nil {
			return nil, fmt.Errorf("%w", apperr.ErrInvalidInput)
		}
		p.CreatedBy = userID
		if err := s.projectFields.UpsertValue(ctx, projectID, p); err != nil {
			return nil, err
		}
	}
	rows, err := s.projectFields.GetValues(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return toFieldValueDTOs(rows), nil
}

// -- Shared helpers -----------------------------------------------------------

func resolveFieldValue(fieldID, fieldType string, options *string, value interface{}) (repository.SetFieldValueParams, error) {
	p := repository.SetFieldValueParams{FieldID: fieldID}
	switch fieldType {
	case "text", "url":
		s, ok := value.(string)
		if !ok {
			return p, fmt.Errorf("field %s expects a string value", fieldID)
		}
		p.ValueText = &s
	case "select":
		s, ok := value.(string)
		if !ok {
			return p, fmt.Errorf("field %s expects a string value", fieldID)
		}
		if options != nil {
			var opts []string
			if err := json.Unmarshal([]byte(*options), &opts); err == nil {
				valid := false
				for _, o := range opts {
					if o == s {
						valid = true
						break
					}
				}
				if !valid {
					return p, fmt.Errorf("value '%s' is not a valid option for field %s", s, fieldID)
				}
			}
		}
		p.ValueText = &s
	case "number":
		switch v := value.(type) {
		case float64:
			p.ValueNumber = &v
		case int64:
			f := float64(v)
			p.ValueNumber = &f
		default:
			return p, fmt.Errorf("field %s expects a numeric value", fieldID)
		}
	case "date":
		s, ok := value.(string)
		if !ok || !dateRe.MatchString(s) {
			return p, fmt.Errorf("field %s expects a date in YYYY-MM-DD format", fieldID)
		}
		if _, err := time.Parse("2006-01-02", s); err != nil {
			return p, fmt.Errorf("field %s: invalid date '%s'", fieldID, s)
		}
		p.ValueDate = &s
	case "boolean":
		b, ok := value.(bool)
		if !ok {
			return p, fmt.Errorf("field %s expects a boolean value", fieldID)
		}
		var v int64
		if b {
			v = 1
		}
		p.ValueBoolean = &v
	}
	return p, nil
}


func toFieldValueDTOs(rows []repository.FieldValue) []*FieldValueDTO {
	out := make([]*FieldValueDTO, len(rows))
	for i, row := range rows {
		out[i] = toFieldValueDTO(row)
	}
	return out
}

func toFieldValueDTO(row repository.FieldValue) *FieldValueDTO {
	dto := &FieldValueDTO{
		FieldID:           row.FieldID,
		FieldKey:          row.FieldKey,
		FieldName:         row.FieldName,
		FieldType:         row.FieldType,
		FieldOptions:      row.FieldOptions,
		DefinitionDeleted: row.DefinitionDeleted,
	}
	switch row.FieldType {
	case "text", "url", "select":
		if row.ValueText != nil {
			dto.Value = *row.ValueText
		}
	case "number":
		if row.ValueNumber != nil {
			dto.Value = *row.ValueNumber
		}
	case "date":
		if row.ValueDate != nil {
			dto.Value = *row.ValueDate
		}
	case "boolean":
		if row.ValueBoolean != nil {
			dto.Value = *row.ValueBoolean != 0
		}
	}
	return dto
}
