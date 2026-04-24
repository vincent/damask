package service_test

import (
	"context"
	"errors"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"
	"golang.org/x/crypto/bcrypt"
)

func init() {
	// Use min cost so tests don't spend time on bcrypt.
	service.ShareBcryptCost = bcrypt.MinCost
}

func newShareSvc(t *testing.T) (service.ShareService, *memory.RealShareRepo) {
	t.Helper()
	repo := memory.NewRealShareRepo()
	return service.NewShareService(repo), repo
}

func baseShareParams() service.CreateShareParams {
	return service.CreateShareParams{
		CreatedBy:  "user_1",
		Label:      "My Share",
		TargetType: "asset",
		TargetID:   "ast_1",
	}
}

// --- Create ---

func TestShareService_Create_OK(t *testing.T) {
	svc, _ := newShareSvc(t)
	dto, err := svc.Create(context.Background(), "ws_1", baseShareParams())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.TargetType != "asset" || dto.WorkspaceID != "ws_1" {
		t.Errorf("unexpected dto: %+v", dto)
	}
	if dto.ID == "" {
		t.Error("expected non-empty ID")
	}
	if dto.PasswordHash != nil {
		t.Error("expected no password hash when no password given")
	}
}

func TestShareService_Create_InvalidTargetType(t *testing.T) {
	svc, _ := newShareSvc(t)
	p := baseShareParams()
	p.TargetType = "invalid"
	_, err := svc.Create(context.Background(), "ws_1", p)
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestShareService_Create_WithPassword(t *testing.T) {
	svc, _ := newShareSvc(t)
	p := baseShareParams()
	pass := "secret123"
	p.Password = &pass
	dto, err := svc.Create(context.Background(), "ws_1", p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.PasswordHash == nil {
		t.Fatal("expected password hash to be set")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*dto.PasswordHash), []byte(pass)); err != nil {
		t.Errorf("password hash does not match: %v", err)
	}
}

func TestShareService_Create_WithExpiry(t *testing.T) {
	svc, _ := newShareSvc(t)
	p := baseShareParams()
	days := 7
	p.ExpiresInDays = &days
	dto, err := svc.Create(context.Background(), "ws_1", p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.ExpiresAt == nil {
		t.Error("expected ExpiresAt to be set")
	}
}

// --- Get ---

func TestShareService_Get_NotFound(t *testing.T) {
	svc, _ := newShareSvc(t)
	_, err := svc.Get(context.Background(), "ws_1", "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- Update ---

func TestShareService_Update_Label(t *testing.T) {
	svc, _ := newShareSvc(t)
	dto, _ := svc.Create(context.Background(), "ws_1", baseShareParams())
	newLabel := "Updated Label"
	updated, err := svc.Update(context.Background(), "ws_1", dto.ID, service.UpdateShareParams{Label: &newLabel})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Label != "Updated Label" {
		t.Errorf("Label: got %q, want %q", updated.Label, "Updated Label")
	}
}

func TestShareService_Update_ClearPassword(t *testing.T) {
	svc, _ := newShareSvc(t)
	p := baseShareParams()
	pass := "secret"
	p.Password = &pass
	dto, _ := svc.Create(context.Background(), "ws_1", p)
	updated, err := svc.Update(context.Background(), "ws_1", dto.ID, service.UpdateShareParams{ClearPassword: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.PasswordHash != nil {
		t.Error("expected password hash to be cleared")
	}
}

func TestShareService_Update_NotFound(t *testing.T) {
	svc, _ := newShareSvc(t)
	newLabel := "x"
	_, err := svc.Update(context.Background(), "ws_1", "nope", service.UpdateShareParams{Label: &newLabel})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- Revoke ---

func TestShareService_Revoke_OK(t *testing.T) {
	svc, _ := newShareSvc(t)
	dto, _ := svc.Create(context.Background(), "ws_1", baseShareParams())
	if err := svc.Revoke(context.Background(), "ws_1", dto.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	revoked, err := svc.Get(context.Background(), "ws_1", dto.ID)
	if err != nil {
		t.Fatalf("unexpected error after revoke: %v", err)
	}
	if revoked.RevokedAt == nil {
		t.Error("expected RevokedAt to be set after revoke")
	}
}

func TestShareService_Revoke_NotFound(t *testing.T) {
	svc, _ := newShareSvc(t)
	err := svc.Revoke(context.Background(), "ws_1", "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
