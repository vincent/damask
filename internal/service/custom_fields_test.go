package service_test

import (
	"context"
	"errors"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"
)

func newFieldSvc(t *testing.T) service.FieldService {
	t.Helper()
	repo := memory.NewRealFieldRepo()
	return service.NewFieldService(repo)
}

func baseFieldParams() service.CreateFieldDefinitionParams {
	return service.CreateFieldDefinitionParams{
		CreatedBy: "user_1",
		Scope:     "asset",
		Name:      "Rating",
		Key:       "rating",
		FieldType: "number",
	}
}

// --- Create ---

func TestFieldService_Create_OK(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	dto, err := svc.Create(context.Background(), "ws_1", baseFieldParams())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Name != "Rating" || dto.Key != "rating" {
		t.Errorf("unexpected dto: %+v", dto)
	}
	if dto.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestFieldService_Create_EmptyName(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	p := baseFieldParams()
	p.Name = "   "
	_, err := svc.Create(context.Background(), "ws_1", p)
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestFieldService_Create_InvalidScope(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	p := baseFieldParams()
	p.Scope = "invalid"
	_, err := svc.Create(context.Background(), "ws_1", p)
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestFieldService_Create_DuplicateKey(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	if _, err := svc.Create(context.Background(), "ws_1", baseFieldParams()); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := svc.Create(context.Background(), "ws_1", baseFieldParams())
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("expected ErrConflict for duplicate key, got %v", err)
	}
}

func TestFieldService_Create_MaxFields(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	for i := range 50 {
		p := baseFieldParams()
		p.Key = "field_" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		p.Name = "Field " + p.Key
		if _, err := svc.Create(context.Background(), "ws_1", p); err != nil {
			t.Fatalf("create #%d: %v", i, err)
		}
	}
	// 51st should fail
	p := baseFieldParams()
	p.Key = "overflow"
	p.Name = "Overflow"
	_, err := svc.Create(context.Background(), "ws_1", p)
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput (max fields), got %v", err)
	}
}

// --- Get ---

func TestFieldService_Get_NotFound(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	_, err := svc.Get(context.Background(), "ws_1", "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- Update ---

func TestFieldService_Update_ImmutableKey(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	dto, _ := svc.Create(context.Background(), "ws_1", baseFieldParams())
	newKey := "new_key"
	_, err := svc.Update(context.Background(), "ws_1", dto.ID, service.UpdateFieldDefinitionParams{Key: &newKey})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput (immutable key), got %v", err)
	}
}

func TestFieldService_Update_ImmutableFieldType(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	dto, _ := svc.Create(context.Background(), "ws_1", baseFieldParams())
	newType := "text"
	_, err := svc.Update(context.Background(), "ws_1", dto.ID, service.UpdateFieldDefinitionParams{FieldType: &newType})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput (immutable field_type), got %v", err)
	}
}

func TestFieldService_Update_OK(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	dto, _ := svc.Create(context.Background(), "ws_1", baseFieldParams())
	newName := "Score"
	updated, err := svc.Update(
		context.Background(),
		"ws_1",
		dto.ID,
		service.UpdateFieldDefinitionParams{Name: &newName},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != "Score" {
		t.Errorf("Name: got %q, want %q", updated.Name, "Score")
	}
}

func TestFieldService_Update_EmptyName(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	dto, _ := svc.Create(context.Background(), "ws_1", baseFieldParams())
	empty := ""
	_, err := svc.Update(context.Background(), "ws_1", dto.ID, service.UpdateFieldDefinitionParams{Name: &empty})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

// --- Delete ---

func TestFieldService_Delete_OK(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	dto, _ := svc.Create(context.Background(), "ws_1", baseFieldParams())
	if err := svc.Delete(context.Background(), "ws_1", dto.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Soft-deleted: should return not found
	_, err := svc.Get(context.Background(), "ws_1", dto.ID)
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after soft-delete, got %v", err)
	}
}

func TestFieldService_Delete_NotFound(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	err := svc.Delete(context.Background(), "ws_1", "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
