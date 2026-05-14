//go:build integration

package service_test

import (
	"context"
	"os"
	"testing"

	"damask/server/internal/desktopconfig"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"
)

func newTestSetup(t *testing.T) (*service.SetupService, *memory.RealUserRepo, *memory.RealWorkspaceRepo) {
	t.Helper()
	svc, u, w, _ := newTestSetupWithDir(t)
	return svc, u, w
}

func newTestSetupWithDir(t *testing.T) (*service.SetupService, *memory.RealUserRepo, *memory.RealWorkspaceRepo, string) {
	t.Helper()
	dir := t.TempDir()
	users := memory.NewRealUserRepo()
	workspaces := memory.NewRealWorkspaceRepo()
	workspaces.SetUserRepo(users)
	svc := service.NewSetupServiceWithCounter(users, workspaces, dir, func(_ context.Context) (int64, error) {
		return users.Count(), nil
	})
	return svc, users, workspaces, dir
}

// --- Status tests ---

func TestSetupStatus_NoConfig_NoOwner(t *testing.T) {
	svc, _, _ := newTestSetup(t)
	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.Configured {
		t.Error("expected Configured=false")
	}
	if status.OwnerExists {
		t.Error("expected OwnerExists=false")
	}
}

func TestSetupStatus_ConfigExists_OwnerExists(t *testing.T) {
	svc, _, _ := newTestSetup(t)

	// Write a config file to the configDir the service uses.
	// We need access to the configDir — use the service's WriteConfig to create it.
	if err := svc.WriteConfig(context.Background(), service.EnvParams{
		Port: 14000,
		StorageParams: service.StorageParams{Type: "local", LocalPath: "/tmp"},
	}); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	if err := svc.CreateOwner(context.Background(), service.OwnerParams{
		WorkspaceName: "Acme",
		Name:          "Alice",
		Email:         "alice@example.com",
		Password:      "hunter2hunter2",
	}); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !status.Configured {
		t.Error("expected Configured=true")
	}
	if !status.OwnerExists {
		t.Error("expected OwnerExists=true")
	}
}

// --- StorageParams.Validate tests ---

func TestStorageParams_Validate_MissingType(t *testing.T) {
	p := service.StorageParams{}
	if err := p.Validate(); err == nil {
		t.Error("expected error for missing type")
	}
}

func TestStorageParams_Validate_LocalPathEmpty(t *testing.T) {
	p := service.StorageParams{Type: "local"}
	if err := p.Validate(); err == nil {
		t.Error("expected error for empty localPath")
	}
}

func TestStorageParams_Validate_S3MissingBucket(t *testing.T) {
	p := service.StorageParams{Type: "s3", S3Region: "us-east-1"}
	if err := p.Validate(); err == nil {
		t.Error("expected error for missing s3Bucket")
	}
}

// --- ValidateStorage local tests ---

func TestValidateStorage_Local_PathNotExist(t *testing.T) {
	svc, _, _ := newTestSetup(t)
	reason, err := svc.ValidateStorage(context.Background(), service.StorageParams{
		Type:      "local",
		LocalPath: "/this/path/does/not/exist/ever",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reason == "" {
		t.Error("expected non-empty reason for non-existent path")
	}
}

func TestValidateStorage_Local_NotWritable(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can write anywhere")
	}
	svc, _, _ := newTestSetup(t)
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o444); err != nil {
		t.Skip("cannot chmod:", err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	reason, err := svc.ValidateStorage(context.Background(), service.StorageParams{
		Type:      "local",
		LocalPath: dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reason == "" {
		t.Error("expected non-empty reason for non-writable path")
	}
}

func TestValidateStorage_Local_OK(t *testing.T) {
	svc, _, _ := newTestSetup(t)
	dir := t.TempDir()
	reason, err := svc.ValidateStorage(context.Background(), service.StorageParams{
		Type:      "local",
		LocalPath: dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reason != "" {
		t.Errorf("expected empty reason, got %q", reason)
	}
}

// --- EnvParams.Validate tests ---

func TestEnvParams_Validate_InvalidPort(t *testing.T) {
	p := service.EnvParams{
		Port:          0,
		StorageParams: service.StorageParams{Type: "local", LocalPath: "/tmp"},
	}
	if err := p.Validate(); err == nil {
		t.Error("expected error for port=0")
	}
}

// --- WriteConfig tests ---

func TestWriteConfig_ProducesCorrectKeys(t *testing.T) {
	svc, _, _, dir := newTestSetupWithDir(t)
	err := svc.WriteConfig(context.Background(), service.EnvParams{
		Port:    14000,
		BaseURL: "http://example.com",
		StorageParams: service.StorageParams{
			Type:      "local",
			LocalPath: "/data/damask",
		},
	})
	if err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	m, err := desktopconfig.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m["PORT"] != "14000" {
		t.Errorf("PORT = %q, want 14000", m["PORT"])
	}
	if m["BASE_URL"] != "http://example.com" {
		t.Errorf("BASE_URL = %q, want http://example.com", m["BASE_URL"])
	}
	if m["STORAGE"] != "local" {
		t.Errorf("STORAGE = %q, want local", m["STORAGE"])
	}
}

func TestWriteConfig_DoesNotOverwritePlaceholderPassword(t *testing.T) {
	svc, _, _, dir := newTestSetupWithDir(t)
	// Write an initial real secret directly into the config dir.
	if err := desktopconfig.Write(dir, map[string]string{"SMTP_PASS": "real-secret"}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// WriteConfig with placeholder — real secret must survive.
	err := svc.WriteConfig(context.Background(), service.EnvParams{
		Port:     14000,
		SMTPHost: "smtp.example.com",
		SMTPPass: "***",
		StorageParams: service.StorageParams{Type: "local", LocalPath: "/tmp"},
	})
	if err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	m, err := desktopconfig.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m["SMTP_PASS"] != "real-secret" {
		t.Errorf("SMTP_PASS = %q, want real-secret (placeholder should not overwrite)", m["SMTP_PASS"])
	}
}

// --- OwnerParams.Validate tests ---

func TestOwnerParams_Validate_ShortPassword(t *testing.T) {
	p := service.OwnerParams{
		WorkspaceName: "Acme",
		Name:          "Alice",
		Email:         "alice@example.com",
		Password:      "short",
	}
	if err := p.Validate(); err == nil {
		t.Error("expected error for short password")
	}
}

// --- CreateOwner tests ---

func TestCreateOwner_Success(t *testing.T) {
	svc, _, _ := newTestSetup(t)
	err := svc.CreateOwner(context.Background(), service.OwnerParams{
		WorkspaceName: "Acme",
		Name:          "Alice",
		Email:         "alice@example.com",
		Password:      "hunter2hunter2",
	})
	if err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	status, _ := svc.Status(context.Background())
	if !status.OwnerExists {
		t.Error("expected OwnerExists=true after creation")
	}
}

func TestCreateOwner_AlreadyExists_Errors(t *testing.T) {
	svc, _, _ := newTestSetup(t)
	p := service.OwnerParams{
		WorkspaceName: "Acme",
		Name:          "Alice",
		Email:         "alice@example.com",
		Password:      "hunter2hunter2",
	}
	if err := svc.CreateOwner(context.Background(), p); err != nil {
		t.Fatalf("first CreateOwner: %v", err)
	}
	p.Email = "bob@example.com"
	if err := svc.CreateOwner(context.Background(), p); err == nil {
		t.Error("expected error when owner already exists")
	}
}

