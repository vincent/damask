package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/audit"
	"damask/server/internal/auth"
	"damask/server/internal/config"
	"damask/server/internal/events"
	"damask/server/internal/mail"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	"damask/server/internal/storage"
	"damask/server/internal/telemetry"

	"github.com/google/uuid"
)

var tracer = telemetry.Tracer("damask/internal/workflow")

type Deps struct {
	Workflows   repository.WorkflowRepository
	Runs        repository.WorkflowRunRepository
	Queue       queue.JobQueue
	Storage     storage.Storage
	Mailer      mail.Mailer
	Hub         events.EventHub
	Audit       audit.Writer
	Assets      AssetManager
	Variants    VariantManager
	Versions    VersionManager
	Shares      ShareManager
	Tags        TagManager
	AssetFields AssetFieldManager
	Workspace   WorkspaceManager
	Config      *config.Config
}

type Asset struct {
	ID               string
	WorkspaceID      string
	MimeType         string
	CurrentVersionID *string
	FolderID         *string
	ProjectID        *string
}

type AssetMoveParams struct {
	FolderID  *string
	ProjectID *string
}

type AssetManager interface {
	Get(ctx context.Context, workspaceID, assetID string) (*Asset, error)
	Move(ctx context.Context, workspaceID, assetID string, p AssetMoveParams) (*Asset, error)
}

type VariantPrepareRequest struct {
	WorkspaceID           string
	AssetID               string
	Type                  string
	Params                json.RawMessage
	AssetMimeType         string
	ImageRouterConfigured bool
	DefaultImageModel     string
	DefaultBgRemoveModel  string
	Title                 *string
	IsShared              bool
}

type VariantPrepareResult struct {
	Type     string
	Params   json.RawMessage
	Title    *string
	IsShared bool
}

type VariantJobPayload struct {
	AssetID      string                `json:"asset_id"`
	WorkspaceID  string                `json:"workspace_id"`
	VersionID    string                `json:"version_id"`
	VersionNum   int64                 `json:"version_num"`
	VariantID    string                `json:"variant_id,omitempty"`
	StorageKey   string                `json:"storage_key"`
	MimeType     string                `json:"mime_type"`
	Type         string                `json:"type"`
	Params       json.RawMessage       `json:"params"`
	Title        *string               `json:"title,omitempty"`
	IsShared     bool                  `json:"is_shared,omitempty"`
	Continuation *WorkflowContinuation `json:"continuation,omitempty"`
}

type VariantManager interface {
	PrepareCreate(ctx context.Context, p VariantPrepareRequest) (VariantPrepareResult, error)
	GetVariantByID(ctx context.Context, workspaceID, id string) (repository.Variant, error)
}

type VersionManager interface {
	GetByID(ctx context.Context, id string) (repository.AssetVersion, error)
	NextVersionNum(ctx context.Context, assetID string) (int64, error)
	Create(ctx context.Context, v repository.AssetVersion) (repository.AssetVersion, error)
	SetCurrent(ctx context.Context, assetID, versionID string) error
}

// WorkflowContinuation carries the data needed to resume a workflow run
// after an async job completes.
type WorkflowContinuation struct {
	RunID       string `json:"run_id"`
	NodeID      string `json:"node_id"`
	WorkflowID  string `json:"workflow_id"`
	WorkspaceID string `json:"workspace_id"`
	ContextJSON string `json:"context_json"`
}

type ShareCreateParams struct {
	CreatedBy     string
	Label         string
	TargetType    string
	TargetID      string
	ExpiresInDays *int
	AllowComments bool
	AllowDownload bool
}

type ShareManager interface {
	Create(ctx context.Context, workspaceID string, p ShareCreateParams) (string, error)
}

type TagManager interface {
	AddToAsset(ctx context.Context, workspaceID, assetID, tagName string) (string, error)
}

type FieldValueInput struct {
	FieldID string
	Value   any
}

type AssetFieldManager interface {
	SetValues(ctx context.Context, workspaceID, assetID, userID string, inputs []FieldValueInput) error
}

type WorkspaceManager interface {
	GetImageRouterKeyStatus(ctx context.Context, workspaceID string) (bool, error)
}

type RunWorkflowPayload struct {
	RunID string `json:"run_id"`
}

func NewToken() (string, error) { return newToken() }

func Sha256Hex(raw string) string { return sha256Hex(raw) }

func newID() string { return uuid.NewString() }

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func jsonToMap(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return map[string]any{}
	}
	return out
}

func sha256Hex(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func newToken() (string, error) {
	id := uuid.NewString() + uuid.NewString()
	if id == "" {
		return "", errors.New("failed to generate token")
	}
	return id, nil
}

func actorUserID(ctx context.Context, rc *RunContext) string {
	if userID, ok := rcGetString(rc, "workflow_created_by"); ok && userID != "" {
		return userID
	}
	if actorUserID := auth.ActorFromCtx(ctx).UserID; actorUserID != nil {
		return *actorUserID
	}
	return ""
}

func nowPtr() *time.Time {
	now := time.Now().UTC()
	return &now
}

func ptr(s string) *string { return &s }

func rcGetString(rc *RunContext, key string) (string, bool) {
	val, ok := rc.Get(key)
	if !ok || val == nil {
		return "", false
	}
	switch v := val.(type) {
	case string:
		return v, true
	case fmt.Stringer:
		return v.String(), true
	default:
		return fmt.Sprintf("%v", v), true
	}
}

func rcRequireString(rc *RunContext, key string) (string, error) {
	val, ok := rcGetString(rc, key)
	if !ok || strings.TrimSpace(val) == "" {
		return "", fmt.Errorf("%s is required in workflow context: %w", key, apperr.ErrInvalidInput)
	}
	return val, nil
}

func retryPolicyFromConfig(_ json.RawMessage) RetryPolicy {
	return DefaultRetryPolicy()
}
