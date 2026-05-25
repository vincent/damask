package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/service"
)

func newAuditSvc(t *testing.T) service.AuditLogService {
	t.Helper()
	env := newIngressEnv(t) // reuse: opens in-memory SQLite with migrations + seeded workspace/user
	return service.NewAuditLogService(env.queries)
}

// -- ListAssetEvents --

func TestAuditLogService_ListAssetEvents_Empty(t *testing.T) {
	t.Parallel()
	svc := newAuditSvc(t)
	result, err := svc.ListAssetEvents(context.Background(), service.ListAssetEventsParams{
		AssetID:     "ast_1",
		WorkspaceID: "ws_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Events) != 0 {
		t.Errorf("expected empty events, got %d", len(result.Events))
	}
	if result.HasMore {
		t.Errorf("expected HasMore=false")
	}
}

// -- ListProjectEvents --

func TestAuditLogService_ListProjectEvents_Empty(t *testing.T) {
	t.Parallel()
	svc := newAuditSvc(t)
	result, err := svc.ListProjectEvents(context.Background(), service.ListProjectEventsParams{
		ProjectID:   "proj_1",
		WorkspaceID: "ws_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Events) != 0 {
		t.Errorf("expected empty events, got %d", len(result.Events))
	}
}

// -- ListWorkspaceActivity --

func TestAuditLogService_ListWorkspaceActivity_Empty(t *testing.T) {
	t.Parallel()
	svc := newAuditSvc(t)
	result, err := svc.ListWorkspaceActivity(context.Background(), service.ListWorkspaceActivityParams{
		WorkspaceID: "ws_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Events) != 0 {
		t.Errorf("expected empty activity, got %d", len(result.Events))
	}
}

// -- ExportActivity --

func TestAuditLogService_ExportActivity_EmptyWorkspace(t *testing.T) {
	t.Parallel()
	svc := newAuditSvc(t)
	csv, err := svc.ExportActivity(context.Background(), service.ExportActivityParams{
		WorkspaceID: "ws_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(csv, "event_id,") {
		t.Errorf("expected CSV header, got: %q", csv[:min(len(csv), 60)])
	}
}

// -- clampLimit / pagination helpers (tested via ListAssetEvents) --

func TestAuditLogService_ListAssetEvents_LimitDefault(t *testing.T) {
	t.Parallel()
	svc := newAuditSvc(t)
	// limit=0 should default to 50 (no panic, no error)
	result, err := svc.ListAssetEvents(context.Background(), service.ListAssetEventsParams{
		AssetID: "x", WorkspaceID: "y", Limit: 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// -- ValidateExportDateRange --

func TestValidateExportDateRange_OK(t *testing.T) {
	t.Parallel()
	if err := service.ValidateExportDateRange("2024-01-01", "2024-12-31"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateExportDateRange_EmptyOK(t *testing.T) {
	t.Parallel()
	if err := service.ValidateExportDateRange("", ""); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateExportDateRange_BadSince(t *testing.T) {
	t.Parallel()
	err := service.ValidateExportDateRange("not-a-date", "")
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestValidateExportDateRange_BadUntil(t *testing.T) {
	t.Parallel()
	err := service.ValidateExportDateRange("", "2024/12/31")
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

// -- ParseTypesFilter --

func TestParseTypesFilter_Empty(t *testing.T) {
	t.Parallel()
	if got := service.ParseTypesFilter(""); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestParseTypesFilter_Single(t *testing.T) {
	t.Parallel()
	got := service.ParseTypesFilter("asset_renamed")
	if len(got) != 1 || got[0] != "asset_renamed" {
		t.Errorf("unexpected result: %v", got)
	}
}

func TestParseTypesFilter_Multiple(t *testing.T) {
	t.Parallel()
	got := service.ParseTypesFilter("asset_renamed, asset_shared")
	if len(got) != 2 {
		t.Errorf("expected 2 types, got %d: %v", len(got), got)
	}
}
