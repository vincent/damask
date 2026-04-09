package api

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"damask/server/internal/audit"
	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"

	"github.com/gofiber/fiber/v3"
)

// EventActor is the actor embedded in each event response.
type EventActor struct {
	Type string  `json:"type"`
	ID   *string `json:"id"`
	Name *string `json:"name"`
}

// EventResponse is a single event in the API response. Exported for test access.
type EventResponse struct {
	ID            string          `json:"id"`
	EventType     string          `json:"event_type"`
	Actor         EventActor      `json:"actor"`
	Payload       json.RawMessage `json:"payload"`
	CreatedAt     string          `json:"created_at"`
	HumanReadable string          `json:"human_readable"`
}

// EventListResponse wraps paginated events. Exported for test access.
type EventListResponse struct {
	Events     []EventResponse `json:"events"`
	NextCursor *string         `json:"next_cursor"`
	HasMore    bool            `json:"has_more"`
}

// activityEvent is a unified event for the workspace feed.
type activityEvent struct {
	EventResponse
	EntityType string `json:"entity_type"` // "asset" | "project"
	EntityID   string `json:"entity_id"`
}

func buildEventResponse(id, eventType, createdAt, payload string, userID, userName *string, actorType string) EventResponse {
	actor := EventActor{Type: actorType, ID: userID, Name: userName}
	return EventResponse{
		ID:            id,
		EventType:     eventType,
		Actor:         actor,
		Payload:       json.RawMessage(payload),
		CreatedAt:     createdAt,
		HumanReadable: audit.RenderHumanReadable(eventType, payload),
	}
}

// GET /assets/:id/events
func (s *Server) handleListAssetEvents(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	if _, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
		ID: assetID, WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		return errRes(c, fiber.StatusNotFound, "asset not found")
	}

	limit, cursor, typesFilter := parseEventQueryParams(c)

	fetchLimit := limit + 1
	if len(typesFilter) > 1 {
		fetchLimit = limit*int64(len(typesFilter)) + 1
	}

	rows, err := s.db.ListAssetEvents(c.RequestCtx(), dbgen.ListAssetEventsParams{
		AssetID:     assetID,
		WorkspaceID: claims.WorkspaceID,
		Cursor:      cursor,
		EventType:   singleTypeArg(typesFilter),
		Limit:       fetchLimit,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "failed to list events")
	}

	events := make([]EventResponse, 0, len(rows))
	for _, r := range rows {
		if len(typesFilter) > 0 && !typesFilter[r.EventType] {
			continue
		}
		events = append(events, buildEventResponse(r.ID, r.EventType, r.CreatedAt, r.Payload, r.UserID, r.UserName, r.ActorType))
	}

	return c.JSON(paginateEvents(events, limit))
}

// GET /projects/:id/events
func (s *Server) handleListProjectEvents(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	projectID := c.Params("id")

	if _, err := s.db.GetProjectByID(c.RequestCtx(), dbgen.GetProjectByIDParams{
		ID: projectID, WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		return errRes(c, fiber.StatusNotFound, "project not found")
	}

	limit, cursor, typesFilter := parseEventQueryParams(c)

	fetchLimit := limit + 1
	if len(typesFilter) > 1 {
		fetchLimit = limit*int64(len(typesFilter)) + 1
	}

	rows, err := s.db.ListProjectEvents(c.RequestCtx(), dbgen.ListProjectEventsParams{
		ProjectID:   projectID,
		WorkspaceID: claims.WorkspaceID,
		Cursor:      cursor,
		EventType:   singleTypeArg(typesFilter),
		Limit:       fetchLimit,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "failed to list events")
	}

	events := make([]EventResponse, 0, len(rows))
	for _, r := range rows {
		if len(typesFilter) > 0 && !typesFilter[r.EventType] {
			continue
		}
		events = append(events, buildEventResponse(r.ID, r.EventType, r.CreatedAt, r.Payload, r.UserID, r.UserName, r.ActorType))
	}

	return c.JSON(paginateEvents(events, limit))
}

// GET /activity
func (s *Server) handleListWorkspaceActivity(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	limit := int64(20)
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.ParseInt(l, 10, 64); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	var cursorArg interface{}
	if cur := c.Query("cursor"); cur != "" {
		cursorArg = cur
	}
	var userIDArg interface{}
	if uid := c.Query("user_id"); uid != "" {
		userIDArg = uid
	}
	typesFilter := parseTypesFilter(c.Query("types"))

	fetchLimit := limit + 1

	assetRows, err := s.db.ListWorkspaceAssetEvents(c.RequestCtx(), dbgen.ListWorkspaceAssetEventsParams{
		WorkspaceID: claims.WorkspaceID,
		Cursor:      cursorArg,
		UserID:      userIDArg,
		EventType:   singleTypeArg(typesFilter),
		Limit:       fetchLimit,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "failed to list activity")
	}

	projectRows, err := s.db.ListWorkspaceProjectEvents(c.RequestCtx(), dbgen.ListWorkspaceProjectEventsParams{
		WorkspaceID: claims.WorkspaceID,
		Cursor:      cursorArg,
		UserID:      userIDArg,
		EventType:   singleTypeArg(typesFilter),
		Limit:       fetchLimit,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "failed to list activity")
	}

	merged := make([]activityEvent, 0, len(assetRows)+len(projectRows))
	for _, r := range assetRows {
		if len(typesFilter) > 0 && !typesFilter[r.EventType] {
			continue
		}
		merged = append(merged, activityEvent{
			EventResponse: buildEventResponse(r.ID, r.EventType, r.CreatedAt, r.Payload, r.UserID, r.UserName, r.ActorType),
			EntityType:    "asset",
			EntityID:      r.AssetID,
		})
	}
	for _, r := range projectRows {
		if len(typesFilter) > 0 && !typesFilter[r.EventType] {
			continue
		}
		merged = append(merged, activityEvent{
			EventResponse: buildEventResponse(r.ID, r.EventType, r.CreatedAt, r.Payload, r.UserID, r.UserName, r.ActorType),
			EntityType:    "project",
			EntityID:      r.ProjectID,
		})
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].CreatedAt > merged[j].CreatedAt
	})

	hasMore := int64(len(merged)) > limit
	if hasMore {
		merged = merged[:limit]
	}
	var nextCursor *string
	if hasMore && len(merged) > 0 {
		nc := merged[len(merged)-1].CreatedAt
		nextCursor = &nc
	}
	if merged == nil {
		merged = []activityEvent{}
	}

	return c.JSON(fiber.Map{
		"events":      merged,
		"next_cursor": nextCursor,
		"has_more":    hasMore,
	})
}

// GET /activity/export
func (s *Server) handleExportActivity(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	since := c.Query("since")
	until := c.Query("until")
	format := c.Query("format", "csv")
	if format != "csv" {
		return errRes(c, fiber.StatusBadRequest, "unsupported format; use format=csv")
	}

	if since != "" {
		if _, err := time.Parse("2006-01-02", since); err != nil {
			return errRes(c, fiber.StatusBadRequest, "invalid since date; use YYYY-MM-DD")
		}
	}
	if until != "" {
		if _, err := time.Parse("2006-01-02", until); err != nil {
			return errRes(c, fiber.StatusBadRequest, "invalid until date; use YYYY-MM-DD")
		}
	}

	// Use until as cursor (events before this timestamp).
	var cursorArg interface{}
	if until != "" {
		cursorArg = until + "T23:59:59"
	}

	const maxRows = 10000
	assetRows, err := s.db.ListWorkspaceAssetEvents(c.RequestCtx(), dbgen.ListWorkspaceAssetEventsParams{
		WorkspaceID: claims.WorkspaceID,
		Cursor:      cursorArg,
		UserID:      nil,
		EventType:   nil,
		Limit:       maxRows,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "failed to fetch events")
	}
	projectRows, err := s.db.ListWorkspaceProjectEvents(c.RequestCtx(), dbgen.ListWorkspaceProjectEventsParams{
		WorkspaceID: claims.WorkspaceID,
		Cursor:      cursorArg,
		UserID:      nil,
		EventType:   nil,
		Limit:       maxRows,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "failed to fetch events")
	}

	var sb strings.Builder
	sb.WriteString("event_id,event_type,entity_type,entity_id,actor_type,actor_name,payload_summary,created_at\n")

	writeCSVRow := func(id, eventType, entityType, entityID, actorType string, userName *string, payload, createdAt string) {
		if since != "" && createdAt < since {
			return
		}
		name := ""
		if userName != nil {
			name = *userName
		}
		summary := audit.RenderHumanReadable(eventType, payload)
		sb.WriteString(csvEscape(id) + "," +
			csvEscape(eventType) + "," +
			csvEscape(entityType) + "," +
			csvEscape(entityID) + "," +
			csvEscape(actorType) + "," +
			csvEscape(name) + "," +
			csvEscape(summary) + "," +
			csvEscape(createdAt) + "\n")
	}

	for _, r := range assetRows {
		writeCSVRow(r.ID, r.EventType, "asset", r.AssetID, r.ActorType, r.UserName, r.Payload, r.CreatedAt)
	}
	for _, r := range projectRows {
		writeCSVRow(r.ID, r.EventType, "project", r.ProjectID, r.ActorType, r.UserName, r.Payload, r.CreatedAt)
	}

	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", "attachment; filename=\"activity-export.csv\"")
	return c.SendString(sb.String())
}

// parseEventQueryParams extracts limit, cursor, and types filter from query params.
func parseEventQueryParams(c fiber.Ctx) (limit int64, cursor interface{}, typesFilter map[string]bool) {
	limit = 50
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.ParseInt(l, 10, 64); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	if cur := c.Query("cursor"); cur != "" {
		cursor = cur
	}
	typesFilter = parseTypesFilter(c.Query("types"))
	return
}

// parseTypesFilter parses a comma-separated "types" query param into a set.
func parseTypesFilter(raw string) map[string]bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	m := make(map[string]bool, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			m[p] = true
		}
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

// singleTypeArg returns the single event type for DB filtering when exactly one
// type is requested. Returns nil when 0 or 2+ types are requested (in-process
// filtering handles the multi-type case after fetching).
func singleTypeArg(types map[string]bool) interface{} {
	if len(types) == 1 {
		for t := range types {
			return t
		}
	}
	return nil
}

// paginateEvents trims events to limit, detects has_more, and sets next_cursor.
func paginateEvents(events []EventResponse, limit int64) EventListResponse {
	hasMore := int64(len(events)) > limit
	if hasMore {
		events = events[:limit]
	}
	var nextCursor *string
	if hasMore && len(events) > 0 {
		nc := events[len(events)-1].CreatedAt
		nextCursor = &nc
	}
	if events == nil {
		events = []EventResponse{}
	}
	return EventListResponse{
		Events:     events,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}
}

func csvEscape(s string) string {
	if strings.ContainsAny(s, `",`+"\n") {
		s = strings.ReplaceAll(s, `"`, `""`)
		return `"` + s + `"`
	}
	return s
}
