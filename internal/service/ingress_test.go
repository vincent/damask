package service_test

import (
	"context"
	"errors"
	"testing"

	"damask/server/internal/apperr"
	dbpkg "damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/mail"
	"damask/server/internal/service"

	"github.com/google/uuid"
)

const testAppSecret = "test-app-secret-for-tests!!"

// ingressTestEnv holds a db + seeded workspace/user for ingress tests.
type ingressTestEnv struct {
	db          *dbgen.Queries
	svc         service.IngressService
	workspaceID string
	userID      string
}

// newIngressEnv opens a fresh in-memory SQLite DB, seeds a workspace and user,
// and returns a configured IngressService.
func newIngressEnv(t *testing.T) *ingressTestEnv {
	t.Helper()
	queries, sqlDB, err := dbpkg.Open(":memory:?_foreign_keys=ON")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	ctx := context.Background()
	wsID := uuid.NewString()
	userID := uuid.NewString()

	if _, err := queries.CreateWorkspace(ctx, dbgen.CreateWorkspaceParams{
		ID: wsID, Name: "test-workspace",
	}); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := queries.CreateUser(ctx, dbgen.CreateUserParams{
		ID: userID, Email: userID + "@test.com", PasswordHash: "x", Name: "test",
	}); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	mailer := mail.NewMailer(&mail.Config{})
	svc := service.NewIngressService(queries, testAppSecret, nil, mailer)
	return &ingressTestEnv{db: queries, svc: svc, workspaceID: wsID, userID: userID}
}

// seedSource creates an ingress source via the service.
func seedSource(t *testing.T, env *ingressTestEnv, label string) *service.IngressSourceDTO {
	t.Helper()
	dto, err := env.svc.CreateSource(
		context.Background(),
		env.workspaceID,
		env.userID,
		service.CreateIngressSourceParams{
			Type:  "sftp",
			Label: label,
		},
	)
	if err != nil {
		t.Fatalf("seed source: %v", err)
	}
	return dto
}

// -- ListSources --

func TestIngressService_ListSources_Empty(t *testing.T) {
	env := newIngressEnv(t)
	out, err := env.svc.ListSources(context.Background(), env.workspaceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected empty list, got %d", len(out))
	}
}

func TestIngressService_ListSources_WorkspaceIsolation(t *testing.T) {
	env := newIngressEnv(t)
	seedSource(t, env, "my source")

	out, err := env.svc.ListSources(context.Background(), "other-workspace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected 0 for other workspace, got %d", len(out))
	}
}

// -- GetSource --

func TestIngressService_GetSource_NotFound(t *testing.T) {
	env := newIngressEnv(t)
	_, err := env.svc.GetSource(context.Background(), env.workspaceID, "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// -- CreateSource --

func TestIngressService_CreateSource_EmptyLabel(t *testing.T) {
	env := newIngressEnv(t)
	_, err := env.svc.CreateSource(context.Background(), env.workspaceID, env.userID, service.CreateIngressSourceParams{
		Type:  "sftp",
		Label: "",
	})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty label, got %v", err)
	}
}

func TestIngressService_CreateSource_EmptyType(t *testing.T) {
	env := newIngressEnv(t)
	_, err := env.svc.CreateSource(context.Background(), env.workspaceID, env.userID, service.CreateIngressSourceParams{
		Type:  "",
		Label: "my source",
	})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty type, got %v", err)
	}
}

func TestIngressService_CreateSource_DefaultInterval(t *testing.T) {
	env := newIngressEnv(t)
	dto, err := env.svc.CreateSource(
		context.Background(),
		env.workspaceID,
		env.userID,
		service.CreateIngressSourceParams{
			Type:            "sftp",
			Label:           "src",
			PollIntervalMin: 0, // should default to 15
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.PollIntervalMin != 15 {
		t.Errorf("PollIntervalMin: got %d, want 15", dto.PollIntervalMin)
	}
}

func TestIngressService_CreateSource_OK(t *testing.T) {
	env := newIngressEnv(t)
	dto, err := env.svc.CreateSource(
		context.Background(),
		env.workspaceID,
		env.userID,
		service.CreateIngressSourceParams{
			Type:  "sftp",
			Label: "production sftp",
			Config: map[string]any{
				"host":     "sftp.example.com",
				"password": "s3cr3t",
			},
			PollIntervalMin: 30,
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Label != "production sftp" {
		t.Errorf("Label: got %q, want %q", dto.Label, "production sftp")
	}
	if dto.PollIntervalMin != 30 {
		t.Errorf("PollIntervalMin: got %d, want 30", dto.PollIntervalMin)
	}
	if !dto.Enabled {
		t.Errorf("expected Enabled=true by default")
	}
	// Sensitive config fields must be redacted
	if pw, ok := dto.Config["password"]; ok && pw != "***" {
		t.Errorf("password should be redacted, got %v", pw)
	}
}

func TestIngressService_CreateSource_WithRules(t *testing.T) {
	env := newIngressEnv(t)
	dto, err := env.svc.CreateSource(
		context.Background(),
		env.workspaceID,
		env.userID,
		service.CreateIngressSourceParams{
			Type:  "sftp",
			Label: "sftp with rules",
			Rules: []service.CreateIngressRuleParams{
				{Position: 1, Field: "filename", Operator: "contains", Value: ".jpg", Action: "include"},
			},
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rules, err := env.svc.ListRules(context.Background(), env.workspaceID, dto.ID)
	if err != nil {
		t.Fatalf("list rules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Field != "filename" {
		t.Errorf("Field: got %q, want %q", rules[0].Field, "filename")
	}
}

// -- UpdateSource --

func TestIngressService_UpdateSource_NotFound(t *testing.T) {
	env := newIngressEnv(t)
	_, err := env.svc.UpdateSource(context.Background(), env.workspaceID, "nope", service.UpdateIngressSourceParams{})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestIngressService_UpdateSource_LabelKeptWhenEmpty(t *testing.T) {
	env := newIngressEnv(t)
	src := seedSource(t, env, "original label")

	updated, err := env.svc.UpdateSource(
		context.Background(),
		env.workspaceID,
		src.ID,
		service.UpdateIngressSourceParams{
			Label: "", // empty = keep original
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Label != "original label" {
		t.Errorf("Label: got %q, want %q", updated.Label, "original label")
	}
}

// -- DeleteSource --

func TestIngressService_DeleteSource_OK(t *testing.T) {
	env := newIngressEnv(t)
	src := seedSource(t, env, "to delete")

	if err := env.svc.DeleteSource(context.Background(), env.workspaceID, src.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err := env.svc.GetSource(context.Background(), env.workspaceID, src.ID)
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

// -- Rules --

func TestIngressService_ListRules_SourceNotFound(t *testing.T) {
	env := newIngressEnv(t)
	_, err := env.svc.ListRules(context.Background(), env.workspaceID, "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestIngressService_CreateRule_OK(t *testing.T) {
	env := newIngressEnv(t)
	src := seedSource(t, env, "src")

	rule, err := env.svc.CreateRule(context.Background(), env.workspaceID, src.ID, service.CreateIngressRuleParams{
		Position: 1,
		Field:    "filename",
		Operator: "ends_with",
		Value:    ".png",
		Action:   "include",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rule.Action != "include" {
		t.Errorf("Action: got %q, want %q", rule.Action, "include")
	}
}

func TestIngressService_DeleteRule_WrongSource(t *testing.T) {
	env := newIngressEnv(t)
	src1 := seedSource(t, env, "src1")
	src2 := seedSource(t, env, "src2")

	rule, err := env.svc.CreateRule(context.Background(), env.workspaceID, src1.ID, service.CreateIngressRuleParams{
		Position: 1, Field: "filename", Operator: "contains", Value: ".jpg", Action: "include",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Attempt to delete rule via wrong source
	err = env.svc.DeleteRule(context.Background(), env.workspaceID, src2.ID, rule.ID)
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestIngressService_ReorderRules_OK(t *testing.T) {
	env := newIngressEnv(t)
	src := seedSource(t, env, "src")

	r1, _ := env.svc.CreateRule(context.Background(), env.workspaceID, src.ID, service.CreateIngressRuleParams{
		Position: 1, Field: "filename", Operator: "contains", Value: "a", Action: "include",
	})
	r2, _ := env.svc.CreateRule(context.Background(), env.workspaceID, src.ID, service.CreateIngressRuleParams{
		Position: 2, Field: "filename", Operator: "contains", Value: "b", Action: "include",
	})

	reordered, err := env.svc.ReorderRules(context.Background(), env.workspaceID, src.ID, []service.ReorderRuleEntry{
		{ID: r1.ID, Position: 10},
		{ID: r2.ID, Position: 5},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// List returns in position order; r2 (pos=5) should be first
	if reordered[0].ID != r2.ID {
		t.Errorf("expected r2 first after reorder, got %s", reordered[0].ID)
	}
}

// -- Log --

func TestIngressService_ListLog_Empty(t *testing.T) {
	env := newIngressEnv(t)
	out, err := env.svc.ListLog(context.Background(), env.workspaceID, "", 50, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected empty log, got %d", len(out))
	}
}

func TestIngressService_DeleteLogEntry_NotFound(t *testing.T) {
	env := newIngressEnv(t)
	err := env.svc.DeleteLogEntry(context.Background(), env.workspaceID, "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestIngressService_RetryLogEntry_NotFound(t *testing.T) {
	env := newIngressEnv(t)
	_, err := env.svc.RetryLogEntry(context.Background(), env.workspaceID, "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestIngressService_RetryLogEntry_InvalidStatus(t *testing.T) {
	env := newIngressEnv(t)
	src := seedSource(t, env, "src")

	// Insert a log entry (starts as "pending") then mark as "done".
	entry, err := env.db.InsertIngressLogEntry(context.Background(), dbgen.InsertIngressLogEntryParams{
		ID:       uuid.NewString(),
		SourceID: src.ID,
		RemoteID: "remote_1",
		Filename: "file.jpg",
	})
	if err != nil {
		t.Fatalf("insert log entry: %v", err)
	}
	if err := env.db.UpdateIngressLogEntry(context.Background(), dbgen.UpdateIngressLogEntryParams{
		Status: "imported",
		ID:     entry.ID,
	}); err != nil {
		t.Fatalf("update log entry: %v", err)
	}

	_, err = env.svc.RetryLogEntry(context.Background(), env.workspaceID, entry.ID)
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for imported status, got %v", err)
	}
}
