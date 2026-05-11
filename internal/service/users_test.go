package service_test

import (
	"context"
	"errors"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"
	"damask/server/internal/storage"

	"golang.org/x/crypto/bcrypt"
)

func newUserSvc(t *testing.T) (service.UserService, *memory.RealUserRepo, *memory.RealWorkspaceRepo, storage.Storage) {
	t.Helper()
	users := memory.NewRealUserRepo()
	workspaces := memory.NewRealWorkspaceRepo()
	workspaces.SetUserRepo(users)
	stor, err := storage.NewAferoMemoryStorage()
	if err != nil {
		t.Fatal(err)
	}
	svc := service.NewUserService(users, workspaces, stor)
	return svc, users, workspaces, stor
}

func hashPassword(t *testing.T, plain string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(h)
}

// --- Register ---

func TestUserService_Register_OK(t *testing.T) {
	svc, _, workspaces, _ := newUserSvc(t)

	result, err := svc.Register(context.Background(), service.RegisterUserParams{
		UserID:        "u_1",
		Name:          "Alice",
		Email:         "alice@example.com",
		PasswordHash:  hashPassword(t, "secret"),
		WorkspaceName: "Alice's Workspace",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.User.Email != "alice@example.com" {
		t.Errorf("email = %q, want alice@example.com", result.User.Email)
	}
	if result.WorkspaceID == "" {
		t.Error("expected non-empty WorkspaceID")
	}

	// workspace should exist
	ws, err := workspaces.GetByID(context.Background(), result.WorkspaceID)
	if err != nil {
		t.Fatalf("workspace not found after register: %v", err)
	}
	if ws.Name != "Alice's Workspace" {
		t.Errorf("workspace name = %q, want %q", ws.Name, "Alice's Workspace")
	}
}

func TestUserService_Register_DuplicateEmail(t *testing.T) {
	svc, _, _, _ := newUserSvc(t)

	params := service.RegisterUserParams{
		UserID:        "u_1",
		Name:          "Alice",
		Email:         "alice@example.com",
		PasswordHash:  hashPassword(t, "secret"),
		WorkspaceName: "Alice's Workspace",
	}
	if _, err := svc.Register(context.Background(), params); err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	params.UserID = "u_2"
	_, err := svc.Register(context.Background(), params)
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("expected ErrConflict for duplicate email, got %v", err)
	}
}

// --- Login ---

func TestUserService_Login_OK(t *testing.T) {
	svc, _, _, _ := newUserSvc(t)

	plain := "s3cret!"
	_, err := svc.Register(context.Background(), service.RegisterUserParams{
		UserID:        "u_1",
		Name:          "Bob",
		Email:         "bob@example.com",
		PasswordHash:  hashPassword(t, plain),
		WorkspaceName: "Bob's Workspace",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	result, err := svc.Login(context.Background(), service.LoginUserParams{
		Email:         "bob@example.com",
		PlainPassword: plain,
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if result.User.Email != "bob@example.com" {
		t.Errorf("email = %q", result.User.Email)
	}
	if result.WorkspaceID == "" {
		t.Error("expected non-empty WorkspaceID")
	}
}

func TestUserService_Login_WrongPassword(t *testing.T) {
	svc, _, _, _ := newUserSvc(t)

	_, err := svc.Register(context.Background(), service.RegisterUserParams{
		UserID:        "u_1",
		Name:          "Carol",
		Email:         "carol@example.com",
		PasswordHash:  hashPassword(t, "correct"),
		WorkspaceName: "Carol's Workspace",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	_, err = svc.Login(context.Background(), service.LoginUserParams{
		Email:         "carol@example.com",
		PlainPassword: "wrong",
	})
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for wrong password, got %v", err)
	}
}

func TestUserService_Login_UnknownEmail(t *testing.T) {
	svc, _, _, _ := newUserSvc(t)

	_, err := svc.Login(context.Background(), service.LoginUserParams{
		Email:         "nobody@example.com",
		PlainPassword: "x",
	})
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for unknown email, got %v", err)
	}
}

// --- GetByID ---

func TestUserService_GetByID_OK(t *testing.T) {
	svc, users, _, _ := newUserSvc(t)
	users.Seed(repository.User{ID: "u_1", Email: "x@x.com", Name: "X"})

	dto, err := svc.GetByID(context.Background(), "u_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Email != "x@x.com" {
		t.Errorf("email = %q", dto.Email)
	}
}

func TestUserService_GetByID_NotFound(t *testing.T) {
	svc, _, _, _ := newUserSvc(t)
	_, err := svc.GetByID(context.Background(), "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
