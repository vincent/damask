package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"damask/server/internal/auth"

	"github.com/gofiber/fiber/v3"
)

// Event is an SSE payload sent to connected clients.
type Event struct {
	Type         string `json:"type"`
	AssetID      string `json:"asset_id"`
	ThumbnailKey string `json:"thumbnail_key"`
}

// EventHub is a workspace-scoped SSE broadcast bus.
type EventHub struct {
	mu      sync.RWMutex
	subs    map[string]map[uint64]chan Event
	counter uint64
}

func NewEventHub() *EventHub {
	return &EventHub{subs: make(map[string]map[uint64]chan Event)}
}

// Subscribe registers a new subscriber for workspaceID.
// The returned cancel func must be called when the connection closes.
func (h *EventHub) Subscribe(workspaceID string) (<-chan Event, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.counter++
	id := h.counter
	ch := make(chan Event, 8)

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
func (h *EventHub) Publish(workspaceID string, ev Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, ch := range h.subs[workspaceID] {
		select {
		case ch <- ev:
		default: // subscriber too slow; drop
		}
	}
}

func (s *Server) handleEvents(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	ch, cancel := s.hub.Subscribe(claims.WorkspaceID)

	return c.SendStreamWriter(func(w *bufio.Writer) {
		defer cancel()

		fmt.Fprintf(w, ": connected\n\n")
		if err := w.Flush(); err != nil {
			return
		}

		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case ev, ok := <-ch:
				if !ok {
					return
				}
				data, _ := json.Marshal(ev)
				fmt.Fprintf(w, "data: %s\n\n", data)
				if err := w.Flush(); err != nil {
					return
				}
			case <-ticker.C:
				fmt.Fprintf(w, ": heartbeat\n\n")
				if err := w.Flush(); err != nil {
					return
				}
			}
		}
	})
}
