// Package audit provides append-only audit logging for asset and project events.
// Event writes are best-effort: failures are logged internally but never propagate
// to the calling handler.
package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// EventWriter writes asset and project events to the DB.
// It must be injected into any handler that modifies asset or project state.
type EventWriter struct {
	db *sql.DB
}

// New creates an EventWriter backed by the given *sql.DB.
func New(db *sql.DB) *EventWriter {
	return &EventWriter{db: db}
}

// AssetEvent describes a single asset event to be recorded.
type AssetEvent struct {
	WorkspaceID string
	AssetID     string
	UserID      *string // nil for system events
	ActorType   string  // "user" | "system"
	EventType   string
	Payload     any // will be JSON-marshalled
}

// ProjectEvent describes a single project event to be recorded.
type ProjectEvent struct {
	WorkspaceID string
	ProjectID   string
	UserID      *string
	ActorType   string
	EventType   string
	Payload     any
}

// WriteAsset inserts an asset event row. Errors are logged but never returned.
func (w *EventWriter) WriteAsset(ctx context.Context, e AssetEvent) {
	payload, err := json.Marshal(e.Payload)
	if err != nil {
		slog.Error("audit: marshal asset event", "event_type", e.EventType, "error", err)
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err = w.db.ExecContext(ctx, `
		INSERT INTO asset_events (id, workspace_id, asset_id, user_id, actor_type, event_type, payload, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'))
	`, uuid.NewString(), e.WorkspaceID, e.AssetID, e.UserID, e.ActorType, e.EventType, string(payload))
	if err != nil {
		slog.Error("audit: insert asset event", "event_type", e.EventType, "asset_id", e.AssetID, "error", err)
	}
}

// WriteAssetAsync writes an asset event in a background goroutine.
// Use for high-volume events like asset_downloaded where latency matters.
func (w *EventWriter) WriteAssetAsync(e AssetEvent) {
	go func() {
		w.WriteAsset(context.Background(), e)
	}()
}

// WriteProject inserts a project event row. Errors are logged but never returned.
func (w *EventWriter) WriteProject(ctx context.Context, e ProjectEvent) {
	payload, err := json.Marshal(e.Payload)
	if err != nil {
		slog.Error("audit: marshal project event", "event_type", e.EventType, "error", err)
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err = w.db.ExecContext(ctx, `
		INSERT INTO project_events (id, workspace_id, project_id, user_id, actor_type, event_type, payload, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'))
	`, uuid.NewString(), e.WorkspaceID, e.ProjectID, e.UserID, e.ActorType, e.EventType, string(payload))
	if err != nil {
		slog.Error("audit: insert project event", "event_type", e.EventType, "project_id", e.ProjectID, "error", err)
	}
}
