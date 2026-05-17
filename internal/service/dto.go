package service

import (
	"encoding/json"
	"fmt"
	"io"
	netmail "net/mail"
	"strings"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/workflow"
)

// ListAssetsParams holds filters for listing assets via AssetService.List.
type ListAssetsParams struct {
	WorkspaceID string
	// Filters
	ProjectID    *string
	FolderID     *string // non-nil = filter by folder_id; use FolderIsRoot for root
	FolderIsRoot bool    // true = folder_id IS NULL (requires ProjectID)
	CollectionID *string
	TagNames     []string
	SearchQuery  string
	MimePrefix   *string
	// Sort: "created_at" (default), "size", "id", "taken_at"
	SortField string
	SortDesc  bool
	// Cursor (opaque; parsed by handler from cursor query param)
	CursorField string
	CursorValue string
	CursorID    string
	Limit       int64
}

// FieldFilter is a typed field[key][op]=value filter for asset listing.
type FieldFilter struct {
	Key      string
	Operator string // eq | lt | lte | gt | gte | contains | starts_with
	Value    string
}

// ListAssetsByFieldsParams holds parameters for field-filter-based asset listing.
type ListAssetsByFieldsParams struct {
	WorkspaceID  string
	FieldFilters []FieldFilter
	CursorAt     *string // raw cursor value (created_at string)
	CursorID     *string
	Limit        int64
}

// MoveAssetParams holds the destination for AssetService.Move.
// Nil fields mean "keep existing value". An empty-string pointer clears the field.
type MoveAssetParams struct {
	FolderID  *string
	ProjectID *string
}

// AssetDTO is the output of AssetService methods.
type AssetDTO struct {
	ID                   string
	WorkspaceID          string
	ProjectID            *string
	FolderID             *string
	DerivedFromAssetID   *string
	OriginalFilename     string
	StorageKey           string
	MimeType             string
	Size                 int64
	Width                *int64
	Height               *int64
	ThumbnailKey         *string
	ThumbnailContentType string
	Metadata             *string
	CurrentVersionID     *string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type UploadAssetVersionParams struct {
	WorkspaceID string
	AssetID     string
	Filename    string
	ContentType string
	Comment     string
	UserID      string
	Reader      io.Reader
}

type UploadAssetVersionResult struct {
	Asset   *AssetDTO
	Version *VersionDTO
}

type CreateWorkflowParams struct {
	Name                 string
	Description          string
	Graph                string
	NotifyOnFailureEmail string
}

func (p CreateWorkflowParams) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("name is required: %w", apperr.ErrInvalidInput)
	}
	if len(p.Name) > 200 {
		return fmt.Errorf("name must not exceed 200 characters: %w", apperr.ErrInvalidInput)
	}
	var graph workflow.Graph
	if err := json.Unmarshal([]byte(p.Graph), &graph); err != nil {
		return fmt.Errorf("graph is not valid JSON: %w", apperr.ErrInvalidInput)
	}
	if err := graph.Validate(); err != nil {
		return fmt.Errorf("graph: %w: %w", err, apperr.ErrInvalidInput)
	}
	if err := validateWorkflowFailureEmail(p.NotifyOnFailureEmail); err != nil {
		return err
	}
	return nil
}

type UpdateWorkflowParams struct {
	Name                 *string
	Description          *string
	Graph                *string
	NotifyOnFailureEmail *string
}

func (p UpdateWorkflowParams) Validate() error {
	if p.Name != nil && strings.TrimSpace(*p.Name) == "" {
		return fmt.Errorf("name must not be empty: %w", apperr.ErrInvalidInput)
	}
	if p.Graph != nil {
		var graph workflow.Graph
		if err := json.Unmarshal([]byte(*p.Graph), &graph); err != nil {
			return fmt.Errorf("graph is not valid JSON: %w", apperr.ErrInvalidInput)
		}
		if err := graph.Validate(); err != nil {
			return fmt.Errorf("graph: %w: %w", err, apperr.ErrInvalidInput)
		}
	}
	if p.NotifyOnFailureEmail != nil {
		if err := validateWorkflowFailureEmail(*p.NotifyOnFailureEmail); err != nil {
			return err
		}
	}
	return nil
}

type WorkflowDTO struct {
	ID                   string
	WorkspaceID          string
	Name                 string
	Description          string
	Enabled              bool
	TriggerType          string
	Graph                string
	NotifyOnFailureEmail string
	LastRunAt            *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type WorkflowRunDTO struct {
	ID          string
	WorkflowID  string
	Status      string
	TriggerData map[string]any
	Error       *string
	StartedAt   *time.Time
	CompletedAt *time.Time
	Steps       []WorkflowRunStepDTO
	CreatedAt   time.Time
}

type WorkflowRunStepDTO struct {
	NodeID      string
	NodeType    string
	Status      string
	Attempt     int
	InputCtx    map[string]any
	OutputCtx   map[string]any
	Error       *string
	StartedAt   *time.Time
	CompletedAt *time.Time
}

type WorkflowNodePort struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type WorkflowNodeSchema struct {
	Type         string             `json:"type"`
	Label        string             `json:"label"`
	Category     string             `json:"category"`
	Description  string             `json:"description"`
	Inputs       []WorkflowNodePort `json:"inputs"`
	Outputs      []WorkflowNodePort `json:"outputs"`
	ConfigSchema json.RawMessage    `json:"config_schema"`
}

type WorkflowTemplateDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	TriggerType string `json:"trigger_type"`
	Graph       string `json:"graph"`
}

func validateWorkflowFailureEmail(raw string) error {
	email := strings.TrimSpace(raw)
	if email == "" {
		return nil
	}
	if _, err := netmail.ParseAddress(email); err != nil {
		return fmt.Errorf("notify_on_failure_email is invalid: %w", apperr.ErrInvalidInput)
	}
	return nil
}
