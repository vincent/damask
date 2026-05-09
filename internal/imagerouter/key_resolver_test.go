package imagerouter

import (
	"context"
	"errors"
	"testing"

	"damask/server/internal/ingress"
)

type stubWorkspaceKeyRepo struct {
	key string
	err error
}

func (r stubWorkspaceKeyRepo) GetImageRouterKey(_ context.Context, _ string) (string, error) {
	return r.key, r.err
}

func TestResolveKeyWorkspaceOverride(t *testing.T) {
	encKey, err := ingress.EncryptConfig("secret", []byte("workspace-key"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	key, source, err := ResolveKey(context.Background(), "ws_1", stubWorkspaceKeyRepo{key: encKey}, "secret", "env-key")
	if err != nil {
		t.Fatalf("ResolveKey: %v", err)
	}
	if key != "workspace-key" {
		t.Fatalf("key = %q, want workspace-key", key)
	}
	if source != SourceWorkspace {
		t.Fatalf("source = %q, want %q", source, SourceWorkspace)
	}
}

func TestResolveKeyFallsBackToEnv(t *testing.T) {
	key, source, err := ResolveKey(context.Background(), "ws_1", stubWorkspaceKeyRepo{}, "secret", "env-key")
	if err != nil {
		t.Fatalf("ResolveKey: %v", err)
	}
	if key != "env-key" {
		t.Fatalf("key = %q, want env-key", key)
	}
	if source != SourceEnv {
		t.Fatalf("source = %q, want %q", source, SourceEnv)
	}
}

func TestResolveKeyNoneConfigured(t *testing.T) {
	key, source, err := ResolveKey(context.Background(), "ws_1", stubWorkspaceKeyRepo{}, "secret", "")
	if err != nil {
		t.Fatalf("ResolveKey: %v", err)
	}
	if key != "" {
		t.Fatalf("key = %q, want empty", key)
	}
	if source != SourceNone {
		t.Fatalf("source = %q, want %q", source, SourceNone)
	}
}

func TestResolveKeyCorruptCiphertext(t *testing.T) {
	_, _, err := ResolveKey(context.Background(), "ws_1", stubWorkspaceKeyRepo{key: "not-base64"}, "secret", "env-key")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetKeyStatus(t *testing.T) {
	status, err := GetKeyStatus(context.Background(), "ws_1", stubWorkspaceKeyRepo{}, "secret", "env-key")
	if err != nil {
		t.Fatalf("GetKeyStatus: %v", err)
	}
	if !status.KeySet || status.Source != SourceEnv {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestResolveKeyPropagatesRepoError(t *testing.T) {
	want := errors.New("boom")
	_, _, err := ResolveKey(context.Background(), "ws_1", stubWorkspaceKeyRepo{err: want}, "secret", "")
	if !errors.Is(err, want) {
		t.Fatalf("expected %v, got %v", want, err)
	}
}
