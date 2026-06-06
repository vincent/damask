// Package events provides SSE event broadcasting.
package events

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"damask/server/internal/auth"
	"damask/server/internal/telemetry"

	"github.com/gofiber/fiber/v3"
)

const sseEventTicker = 10 * time.Second

// Event is an payload sent to connected clients.
type Event struct {
	Type         string `json:"type"`
	AssetID      string `json:"asset_id"`
	VariantID    string `json:"variant_id,omitempty"`
	WorkflowID   string `json:"workflow_id,omitempty"`
	RunID        string `json:"run_id,omitempty"`
	NodeID       string `json:"node_id,omitempty"`
	Status       string `json:"status,omitempty"`
	ThumbnailKey string `json:"thumbnail_key"`
	JobID        string `json:"job_id,omitempty"`
	Error        string `json:"error,omitempty"`
	// Draft event fields.
	Nonce      string `json:"nonce,omitempty"`
	PreviewURL string `json:"preview_url,omitempty"`
	ExpiresAt  string `json:"expires_at,omitempty"`
}

type EventHub interface {
	Subscribe(workspaceID string) (<-chan Event, func())
	Publish(ctx context.Context, workspaceID string, ev Event)
	EventHandler(c fiber.Ctx) error
}

type EventHubImpl struct {
	mu      sync.RWMutex
	subs    map[string]map[uint64]chan Event
	counter uint64
}

// NewEventHub returns a workspace-scoped SSE broadcast bus.
func NewEventHub() EventHub {
	return &EventHubImpl{
		subs: make(map[string]map[uint64]chan Event),
	}
}

// Subscribe registers a new subscriber for workspaceID.
// The returned cancel func must be called when the connection closes.
func (h *EventHubImpl) Subscribe(workspaceID string) (<-chan Event, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.counter++
	id := h.counter
	ch := make(chan Event, 8) //nolint:mnd // fixed buffer size for all subscribers

	if h.subs[workspaceID] == nil {
		h.subs[workspaceID] = make(map[uint64]chan Event)
	}
	h.subs[workspaceID][id] = ch

	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if ws, ok := h.subs[workspaceID]; ok {
			delete(ws, id)
			if len(ws) == 0 {
				delete(h.subs, workspaceID)
			}
		}
		close(ch)
	}
}

// Publish sends an event to all subscribers of workspaceID.
// Non-blocking: slow clients are skipped.
func (h *EventHubImpl) Publish(ctx context.Context, workspaceID string, ev Event) {
	var err error
	_, span := telemetry.StartSpan(ctx, "service.events.publish")
	defer func() { telemetry.EndSpan(span, err) }()

	slog.DebugContext(
		ctx,
		"publishing event",
		"workspace_id",
		workspaceID,
		"event_type",
		ev.Type,
		"asset_id",
		ev.AssetID,
		"variant_id",
		ev.VariantID,
		"job_id",
		ev.JobID,
	)

	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, ch := range h.subs[workspaceID] {
		select {
		case ch <- ev:
		default: // subscriber too slow; drop
		}
	}
}

func (h *EventHubImpl) EventHandler(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	ch, cancel := h.Subscribe(claims.WorkspaceID)

	return c.SendStreamWriter(func(w *bufio.Writer) {
		defer cancel()

		_, _ = fmt.Fprintf(w, ": connected\n\n")
		if err := w.Flush(); err != nil {
			return
		}

		ticker := time.NewTicker(sseEventTicker)
		defer ticker.Stop()

		for {
			select {
			case ev, ok := <-ch:
				if !ok {
					return
				}
				data, _ := json.Marshal(ev)
				_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
				if err := w.Flush(); err != nil {
					return
				}
			case <-ticker.C:
				_, _ = fmt.Fprintf(w, ": heartbeat\n\n")
				if err := w.Flush(); err != nil {
					return
				}
			}
		}
	})
}
