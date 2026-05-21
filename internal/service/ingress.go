package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"damask/server/internal/apperr"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/ingress"
	"damask/server/internal/mail"
	"damask/server/internal/queue"
	apptelemetry "damask/server/internal/telemetry"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
)

const publicTokenLength = 20

// IngressSourceDTO is the output of IngressService source methods.
type IngressSourceDTO struct {
	ID              string         `json:"id"`
	WorkspaceID     string         `json:"workspace_id"`
	CreatedBy       string         `json:"created_by"`
	Type            string         `json:"type"`
	Label           string         `json:"label"`
	PublicToken     string         `json:"public_token"`
	Config          map[string]any `json:"config"` // sensitive fields redacted
	DestFolderID    *string        `json:"dest_folder_id"`
	DestProjectID   *string        `json:"dest_project_id"`
	Enabled         bool           `json:"enabled"`
	PollIntervalMin int64          `json:"poll_interval_min"`
	LastPolledAt    *time.Time     `json:"last_polled_at"`
	LastError       *string        `json:"last_error"`
	ErrorCount      int64          `json:"error_count"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// IngressRuleDTO is the output of IngressService rule methods.
type IngressRuleDTO struct {
	ID       string `json:"id"`
	SourceID string `json:"source_id"`
	Position int64  `json:"position"`
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
	Action   string `json:"action"`
}

// IngressLogEntryDTO is the output of IngressService log methods.
type IngressLogEntryDTO struct {
	ID         string    `json:"id"`
	SourceID   string    `json:"source_id"`
	RemoteID   string    `json:"remote_id"`
	Filename   string    `json:"filename"`
	AssetID    *string   `json:"asset_id"`
	Status     string    `json:"status"`
	Error      *string   `json:"error"`
	ImportedAt time.Time `json:"imported_at"`
}

// CreateIngressSourceParams is the input for IngressService.CreateSource.
type CreateIngressSourceParams struct {
	Type            string
	Label           string
	Config          map[string]any
	DestFolderID    *string
	DestProjectID   *string
	Enabled         *bool
	PollIntervalMin int64
	Rules           []CreateIngressRuleParams
}

// UpdateIngressSourceParams is the input for IngressService.UpdateSource.
// Nil pointer fields mean "keep existing value".
type UpdateIngressSourceParams struct {
	Label           string
	Config          map[string]any // nil = keep existing
	DestFolderID    *string        // use NullableStringUpdate to distinguish nil-absent from nil-clear
	DestFolderSet   bool           // true if DestFolderID should be updated (even to nil)
	DestProjectID   *string
	DestProjectSet  bool
	Enabled         *bool
	PollIntervalMin int64
}

// CreateIngressRuleParams is the input for rule creation.
type CreateIngressRuleParams struct {
	Position int64
	Field    string
	Operator string
	Value    string
	Action   string
}

// UpdateIngressRuleParams is the input for rule update.
type UpdateIngressRuleParams struct {
	Position int64
	Field    string
	Operator string
	Value    string
	Action   string
}

// ReorderRuleEntry pairs a rule ID with its new position.
type ReorderRuleEntry struct {
	ID       string `json:"id"`
	Position int64  `json:"position"`
}

type ingressService struct {
	db        *dbgen.Queries
	appSecret string
	q         queue.JobQueue
	mailer    mail.Mailer
}

// NewIngressService returns an IngressService.
func NewIngressService(db *dbgen.Queries, appSecret string, q queue.JobQueue, mailer mail.Mailer) IngressService {
	return &ingressService{db: db, appSecret: appSecret, q: q, mailer: mailer}
}

func (s *ingressService) ListSources(ctx context.Context, workspaceID string) ([]*IngressSourceDTO, error) {
	rows, err := s.db.ListIngressSources(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]*IngressSourceDTO, 0, len(rows))
	for _, row := range rows {
		dto, err := s.toSourceDTO(row)
		if err != nil {
			continue // skip unreadable sources rather than failing the whole list
		}
		out = append(out, dto)
	}
	return out, nil
}

func (s *ingressService) GetSource(ctx context.Context, workspaceID, id string) (*IngressSourceDTO, error) {
	src, err := s.db.GetIngressSource(ctx, dbgen.GetIngressSourceParams{
		ID: id, WorkspaceID: workspaceID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("source %q: %w", id, apperr.ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	return s.toSourceDTO(src)
}

func (s *ingressService) CreateSource(
	ctx context.Context,
	workspaceID, userID string,
	p CreateIngressSourceParams,
) (*IngressSourceDTO, error) {
	p.Label = strings.TrimSpace(p.Label)
	if p.Label == "" {
		return nil, fmt.Errorf("label is required: %w", apperr.ErrInvalidInput)
	}
	if p.Type == "" {
		return nil, fmt.Errorf("type is required: %w", apperr.ErrInvalidInput)
	}

	interval := p.PollIntervalMin
	if interval <= 0 {
		interval = 15
	}

	if p.Config == nil {
		p.Config = map[string]any{}
	}
	p.Config["workspace_id"] = workspaceID

	mutatedConfig, err := ingress.RunOnCreateHook(p.Type, p.Config)
	if err != nil {
		return nil, fmt.Errorf("could not prepare config: %w", err)
	}
	configBytes, err := json.Marshal(mutatedConfig)
	if err != nil {
		return nil, fmt.Errorf("invalid config: %w", apperr.ErrInvalidInput)
	}
	encryptedConfig, err := ingress.EncryptConfig(s.appSecret, configBytes)
	if err != nil {
		return nil, fmt.Errorf("could not encrypt config: %w", err)
	}

	enabled := int64(1)
	if p.Enabled != nil && !*p.Enabled {
		enabled = 0
	}

	publicToken, err := ingress.GenerateToken(publicTokenLength)
	if err != nil {
		return nil, fmt.Errorf("could not generate public token: %w", err)
	}

	src, err := s.db.CreateIngressSource(ctx, dbgen.CreateIngressSourceParams{
		ID:              uuid.NewString(),
		WorkspaceID:     workspaceID,
		CreatedBy:       userID,
		Type:            p.Type,
		Label:           p.Label,
		Config:          encryptedConfig,
		PublicToken:     publicToken,
		DestFolderID:    p.DestFolderID,
		DestProjectID:   p.DestProjectID,
		Enabled:         enabled,
		PollIntervalMin: interval,
	})
	if err != nil {
		return nil, err
	}

	for _, rule := range p.Rules {
		if _, err := s.db.CreateIngressRule(ctx, dbgen.CreateIngressRuleParams{
			ID:       uuid.NewString(),
			SourceID: src.ID,
			Position: rule.Position,
			Field:    rule.Field,
			Operator: rule.Operator,
			Value:    rule.Value,
			Action:   rule.Action,
		}); err != nil {
			return nil, err
		}
	}

	// Fire-and-forget welcome email. Failures are logged but do not abort creation.
	if creator, err := s.db.GetUserByID(ctx, userID); err == nil {
		if err := s.mailer.SendIngressSourceAdded(ctx, creator.Email, src.Label, workspaceID); err != nil {
			slog.ErrorContext(ctx, "failed to send ingress source added mail", "error", err)
		}
	}

	return s.toSourceDTO(src)
}

func (s *ingressService) UpdateSource(
	ctx context.Context,
	workspaceID, id string,
	p UpdateIngressSourceParams,
) (*IngressSourceDTO, error) {
	existing, err := s.db.GetIngressSource(ctx, dbgen.GetIngressSourceParams{
		ID: id, WorkspaceID: workspaceID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("source %q: %w", id, apperr.ErrNotFound)
	}
	if err != nil {
		return nil, err
	}

	interval := p.PollIntervalMin
	if interval <= 0 {
		interval = existing.PollIntervalMin
	}

	encryptedConfig := existing.Config
	if p.Config != nil {
		p.Config["workspace_id"] = workspaceID
		configBytes, err := json.Marshal(p.Config)
		if err != nil {
			return nil, fmt.Errorf("invalid config: %w", apperr.ErrInvalidInput)
		}
		encryptedConfig, err = ingress.EncryptConfig(s.appSecret, configBytes)
		if err != nil {
			return nil, fmt.Errorf("could not encrypt config: %w", err)
		}
	}

	enabled := existing.Enabled
	if p.Enabled != nil {
		if *p.Enabled {
			enabled = 1
		} else {
			enabled = 0
		}
	}

	label := p.Label
	if label == "" {
		label = existing.Label
	}

	destFolder := existing.DestFolderID
	if p.DestFolderSet {
		destFolder = p.DestFolderID
	}
	destProject := existing.DestProjectID
	if p.DestProjectSet {
		destProject = p.DestProjectID
	}

	src, err := s.db.UpdateIngressSource(ctx, dbgen.UpdateIngressSourceParams{
		Label:           label,
		Config:          encryptedConfig,
		DestFolderID:    destFolder,
		DestProjectID:   destProject,
		Enabled:         enabled,
		PollIntervalMin: interval,
		ID:              id,
		WorkspaceID:     workspaceID,
	})
	if err != nil {
		return nil, err
	}
	return s.toSourceDTO(src)
}

func (s *ingressService) DeleteSource(ctx context.Context, workspaceID, id string) error {
	return s.db.DeleteIngressSource(ctx, dbgen.DeleteIngressSourceParams{
		ID: id, WorkspaceID: workspaceID,
	})
}

func (s *ingressService) TestSource(ctx context.Context, workspaceID, id string) (err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.ingress.test_source",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.ingress.source_id", id),
	)
	defer func() {
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"ingress source test failed",
				"workspace_id",
				workspaceID,
				"source_id",
				id,
				"error",
				err,
			)
		}
	}()

	src, err := s.db.GetIngressSource(ctx, dbgen.GetIngressSourceParams{
		ID: id, WorkspaceID: workspaceID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("source %q: %w", id, apperr.ErrNotFound)
	}
	if err != nil {
		return err
	}

	configJSON, err := ingress.DecryptConfig(s.appSecret, src.Config)
	if err != nil {
		return fmt.Errorf("could not decrypt config: %w", err)
	}

	source, err := ingress.Build(src.Type, configJSON)
	if err != nil {
		return fmt.Errorf("%s: %w", err.Error(), apperr.ErrInvalidInput)
	}

	testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := source.Validate(testCtx); err != nil {
		return fmt.Errorf("%s: %w", err.Error(), apperr.ErrInvalidInput)
	}
	return nil
}

func (s *ingressService) TriggerPoll(ctx context.Context, workspaceID, id string) (jobID string, err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.ingress.trigger_poll",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.ingress.source_id", id),
	)
	defer func() {
		if jobID != "" {
			span.SetAttributes(attribute.String("damask.job_id", jobID))
		}
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"ingress poll trigger failed",
				"workspace_id",
				workspaceID,
				"source_id",
				id,
				"error",
				err,
			)
		}
	}()

	src, err := s.db.GetIngressSource(ctx, dbgen.GetIngressSourceParams{
		ID: id, WorkspaceID: workspaceID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("source %q: %w", id, apperr.ErrNotFound)
	}
	if err != nil {
		return "", err
	}

	payload, _ := json.Marshal(ingress.PollJobPayload{
		SourceID:    src.ID,
		WorkspaceID: src.WorkspaceID,
	})
	job, err := s.q.Enqueue(ctx, workspaceID, queue.JobTypeIngestPoll, string(payload))
	if err != nil {
		return "", fmt.Errorf("could not enqueue poll job: %w", apperr.ErrConflict)
	}
	return job.ID, nil
}

// -- Rules --

func (s *ingressService) ListRules(ctx context.Context, workspaceID, sourceID string) ([]*IngressRuleDTO, error) {
	if _, err := s.db.GetIngressSource(ctx, dbgen.GetIngressSourceParams{
		ID: sourceID, WorkspaceID: workspaceID,
	}); errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("source %q: %w", sourceID, apperr.ErrNotFound)
	} else if err != nil {
		return nil, err
	}

	rows, err := s.db.ListIngressRules(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	out := make([]*IngressRuleDTO, len(rows))
	for i, r := range rows {
		out[i] = toRuleDTO(r)
	}
	return out, nil
}

func (s *ingressService) CreateRule(
	ctx context.Context,
	workspaceID, sourceID string,
	p CreateIngressRuleParams,
) (*IngressRuleDTO, error) {
	if _, err := s.db.GetIngressSource(ctx, dbgen.GetIngressSourceParams{
		ID: sourceID, WorkspaceID: workspaceID,
	}); errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("source %q: %w", sourceID, apperr.ErrNotFound)
	} else if err != nil {
		return nil, err
	}

	r, err := s.db.CreateIngressRule(ctx, dbgen.CreateIngressRuleParams{
		ID:       uuid.NewString(),
		SourceID: sourceID,
		Position: p.Position,
		Field:    p.Field,
		Operator: p.Operator,
		Value:    p.Value,
		Action:   p.Action,
	})
	if err != nil {
		return nil, err
	}
	return toRuleDTO(r), nil
}

func (s *ingressService) UpdateRule(
	ctx context.Context,
	workspaceID, sourceID, ruleID string,
	p UpdateIngressRuleParams,
) (*IngressRuleDTO, error) {
	if _, err := s.db.GetIngressSource(ctx, dbgen.GetIngressSourceParams{
		ID: sourceID, WorkspaceID: workspaceID,
	}); errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("source %q: %w", sourceID, apperr.ErrNotFound)
	} else if err != nil {
		return nil, err
	}

	existing, err := s.db.GetIngressRule(ctx, ruleID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("rule %q: %w", ruleID, apperr.ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	if existing.SourceID != sourceID {
		return nil, fmt.Errorf("rule %q: %w", ruleID, apperr.ErrNotFound)
	}

	field := p.Field
	if field == "" {
		field = existing.Field
	}
	operator := p.Operator
	if operator == "" {
		operator = existing.Operator
	}
	value := p.Value
	if value == "" {
		value = existing.Value
	}
	action := p.Action
	if action == "" {
		action = existing.Action
	}

	r, err := s.db.UpdateIngressRule(ctx, dbgen.UpdateIngressRuleParams{
		Position: p.Position,
		Field:    field,
		Operator: operator,
		Value:    value,
		Action:   action,
		ID:       ruleID,
	})
	if err != nil {
		return nil, err
	}
	return toRuleDTO(r), nil
}

func (s *ingressService) DeleteRule(ctx context.Context, workspaceID, sourceID, ruleID string) error {
	if _, err := s.db.GetIngressSource(ctx, dbgen.GetIngressSourceParams{
		ID: sourceID, WorkspaceID: workspaceID,
	}); errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("source %q: %w", sourceID, apperr.ErrNotFound)
	} else if err != nil {
		return err
	}

	existing, err := s.db.GetIngressRule(ctx, ruleID)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("rule %q: %w", ruleID, apperr.ErrNotFound)
	}
	if err != nil {
		return err
	}
	if existing.SourceID != sourceID {
		return fmt.Errorf("rule %q: %w", ruleID, apperr.ErrNotFound)
	}
	return s.db.DeleteIngressRule(ctx, ruleID)
}

func (s *ingressService) ReorderRules(
	ctx context.Context,
	workspaceID, sourceID string,
	entries []ReorderRuleEntry,
) ([]*IngressRuleDTO, error) {
	if _, err := s.db.GetIngressSource(ctx, dbgen.GetIngressSourceParams{
		ID: sourceID, WorkspaceID: workspaceID,
	}); errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("source %q: %w", sourceID, apperr.ErrNotFound)
	} else if err != nil {
		return nil, err
	}

	for _, e := range entries {
		existing, err := s.db.GetIngressRule(ctx, e.ID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("rule %q: %w", e.ID, apperr.ErrNotFound)
		}
		if err != nil {
			return nil, err
		}
		if existing.SourceID != sourceID {
			return nil, fmt.Errorf("rule %q: %w", e.ID, apperr.ErrNotFound)
		}
		if _, err := s.db.UpdateIngressRule(ctx, dbgen.UpdateIngressRuleParams{
			Position: e.Position,
			Field:    existing.Field,
			Operator: existing.Operator,
			Value:    existing.Value,
			Action:   existing.Action,
			ID:       e.ID,
		}); err != nil {
			return nil, err
		}
	}

	rows, err := s.db.ListIngressRules(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	out := make([]*IngressRuleDTO, len(rows))
	for i, r := range rows {
		out[i] = toRuleDTO(r)
	}
	return out, nil
}

// -- Log --

func (s *ingressService) ListLog(
	ctx context.Context,
	workspaceID string,
	statusFilter string,
	limit, offset int64,
) ([]*IngressLogEntryDTO, error) {
	var statusArg any
	if statusFilter != "" {
		statusArg = statusFilter
	}
	entries, err := s.db.ListWorkspaceIngressLog(ctx, dbgen.ListWorkspaceIngressLogParams{
		WorkspaceID: workspaceID,
		Status:      statusArg,
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*IngressLogEntryDTO, len(entries))
	for i, e := range entries {
		out[i] = toLogEntryDTO(e)
	}
	return out, nil
}

func (s *ingressService) ListSourceLog(
	ctx context.Context,
	workspaceID, sourceID string,
	limit, offset int64,
) ([]*IngressLogEntryDTO, error) {
	if _, err := s.db.GetIngressSource(ctx, dbgen.GetIngressSourceParams{
		ID: sourceID, WorkspaceID: workspaceID,
	}); errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("source %q: %w", sourceID, apperr.ErrNotFound)
	} else if err != nil {
		return nil, err
	}

	entries, err := s.db.ListIngressSourceLog(ctx, dbgen.ListIngressSourceLogParams{
		SourceID: sourceID,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*IngressLogEntryDTO, len(entries))
	for i, e := range entries {
		out[i] = toLogEntryDTO(e)
	}
	return out, nil
}

func (s *ingressService) DeleteLogEntry(ctx context.Context, workspaceID, entryID string) error {
	entry, err := s.db.GetIngressLogEntry(ctx, entryID)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("log entry %q: %w", entryID, apperr.ErrNotFound)
	}
	if err != nil {
		return err
	}

	if _, err := s.db.GetIngressSource(ctx, dbgen.GetIngressSourceParams{
		ID: entry.SourceID, WorkspaceID: workspaceID,
	}); errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("log entry %q: %w", entryID, apperr.ErrForbidden)
	} else if err != nil {
		return err
	}

	return s.db.DeleteIngressLogEntry(ctx, entryID)
}

func (s *ingressService) RetryLogEntry(ctx context.Context, workspaceID, entryID string) (jobID string, err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.ingress.retry_log_entry",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.ingress.log_entry_id", entryID),
	)
	defer func() {
		if jobID != "" {
			span.SetAttributes(attribute.String("damask.job_id", jobID))
		}
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"ingress retry failed",
				"workspace_id",
				workspaceID,
				"entry_id",
				entryID,
				"error",
				err,
			)
		}
	}()

	entry, err := s.db.GetIngressLogEntry(ctx, entryID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("log entry %q: %w", entryID, apperr.ErrNotFound)
	}
	if err != nil {
		return "", err
	}

	src, err := s.db.GetIngressSource(ctx, dbgen.GetIngressSourceParams{
		ID: entry.SourceID, WorkspaceID: workspaceID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("log entry %q: %w", entryID, apperr.ErrForbidden)
	}
	if err != nil {
		return "", err
	}

	if entry.Status == "imported" || entry.Status == WorkflowRunStatusPending {
		return "", fmt.Errorf("only error or skipped entries can be retried: %w", apperr.ErrInvalidInput)
	}

	if err := s.db.UpdateIngressLogEntry(ctx, dbgen.UpdateIngressLogEntryParams{
		Status: WorkflowRunStatusPending,
		ID:     entryID,
	}); err != nil {
		return "", err
	}

	payload, _ := json.Marshal(ingress.FetchJobPayload{
		SourceID:    src.ID,
		WorkspaceID: src.WorkspaceID,
		LogEntryID:  entry.ID,
		RemoteID:    entry.RemoteID,
		Filename:    entry.Filename,
	})
	job, err := s.q.Enqueue(ctx, workspaceID, queue.JobTypeIngestFetch, string(payload))
	if err != nil {
		return "", fmt.Errorf("could not enqueue retry job: %w", apperr.ErrConflict)
	}
	return job.ID, nil
}

// -- Converters --

var sensitiveIngressKeys = []string{"password", "secret", "key", "token"}

func redactConfig(raw map[string]any) map[string]any {
	out := make(map[string]any, len(raw))
	for k, v := range raw {
		kl := strings.ToLower(k)
		redact := false
		for _, s := range sensitiveIngressKeys {
			if strings.Contains(kl, s) {
				redact = true
				break
			}
		}
		if redact {
			out[k] = "***"
		} else {
			out[k] = v
		}
	}
	return out
}

func (s *ingressService) toSourceDTO(src dbgen.IngressSource) (*IngressSourceDTO, error) {
	configJSON, err := ingress.DecryptConfig(s.appSecret, src.Config)
	if err != nil {
		return nil, err
	}
	var configMap map[string]any
	if err := json.Unmarshal(configJSON, &configMap); err != nil {
		configMap = map[string]any{}
	}
	return &IngressSourceDTO{
		ID:              src.ID,
		WorkspaceID:     src.WorkspaceID,
		CreatedBy:       src.CreatedBy,
		Type:            src.Type,
		Label:           src.Label,
		PublicToken:     src.PublicToken,
		Config:          redactConfig(configMap),
		DestFolderID:    src.DestFolderID,
		DestProjectID:   src.DestProjectID,
		Enabled:         src.Enabled != 0,
		PollIntervalMin: src.PollIntervalMin,
		LastPolledAt:    src.LastPolledAt,
		LastError:       src.LastError,
		ErrorCount:      src.ErrorCount,
		CreatedAt:       src.CreatedAt,
		UpdatedAt:       src.UpdatedAt,
	}, nil
}

func toRuleDTO(r dbgen.IngressRule) *IngressRuleDTO {
	return &IngressRuleDTO{
		ID:       r.ID,
		SourceID: r.SourceID,
		Position: r.Position,
		Field:    r.Field,
		Operator: r.Operator,
		Value:    r.Value,
		Action:   r.Action,
	}
}

func toLogEntryDTO(e dbgen.IngressLog) *IngressLogEntryDTO {
	return &IngressLogEntryDTO{
		ID:         e.ID,
		SourceID:   e.SourceID,
		RemoteID:   e.RemoteID,
		Filename:   e.Filename,
		AssetID:    e.AssetID,
		Status:     e.Status,
		Error:      e.Error,
		ImportedAt: e.ImportedAt,
	}
}
