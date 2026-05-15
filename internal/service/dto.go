package service

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"damask/server/internal/apperr"
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
	Name        string
	Description string
	Graph       string
}

func (p CreateWorkflowParams) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("name is required: %w", apperr.ErrInvalidInput)
	}
	if len(p.Name) > 200 {
		return fmt.Errorf("name must not exceed 200 characters: %w", apperr.ErrInvalidInput)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(p.Graph), &raw); err != nil {
		return fmt.Errorf("graph is not valid JSON: %w", apperr.ErrInvalidInput)
	}
	return nil
}

type UpdateWorkflowParams struct {
	Name        *string
	Description *string
	Graph       *string
}

func (p UpdateWorkflowParams) Validate() error {
	if p.Name != nil && strings.TrimSpace(*p.Name) == "" {
		return fmt.Errorf("name must not be empty: %w", apperr.ErrInvalidInput)
	}
	if p.Graph != nil {
		var raw map[string]any
		if err := json.Unmarshal([]byte(*p.Graph), &raw); err != nil {
			return fmt.Errorf("graph is not valid JSON: %w", apperr.ErrInvalidInput)
		}
	}
	return nil
}

type WorkflowDTO struct {
	ID          string
	WorkspaceID string
	Name        string
	Description string
	Enabled     bool
	TriggerType string
	Graph       string
	LastRunAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
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
	ID    string
	Label string
}

type WorkflowNodeSchema struct {
	Type         string
	Label        string
	Category     string
	Description  string
	Inputs       []WorkflowNodePort
	Outputs      []WorkflowNodePort
	ConfigSchema json.RawMessage
}
