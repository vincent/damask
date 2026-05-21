package jobs

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"damask/server/internal/config"
	"damask/server/internal/imagerouter"
	"damask/server/internal/storage"
)

func encodeResolverPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func newImageRouterJobServer(t *testing.T, resolver imagerouter.KeyResolver) *JobServer {
	t.Helper()
	stor, err := storage.NewAferoMemoryStorage()
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	return &JobServer{
		storage:        stor,
		cfg:            &config.Config{ImageRouter: config.ImageRouterConfig{}},
		imgKeyResolver: resolver,
	}
}

func seedSourceImage(t *testing.T, s *JobServer) string {
	t.Helper()
	key := "source.png"
	if err := s.storage.Put(key, bytes.NewReader(encodeResolverPNG(t))); err != nil {
		t.Fatalf("Put: %v", err)
	}
	return key
}

func TestRunImageRouterJobWorkspaceOverrideBeatsEnv(t *testing.T) {
	s := newImageRouterJobServer(t, func(_ context.Context, workspaceID string) (string, imagerouter.KeySource, error) {
		if workspaceID != "ws_1" {
			t.Fatalf("workspaceID = %q", workspaceID)
		}
		return "workspace-key", imagerouter.SourceWorkspace, nil
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer workspace-key" {
			t.Fatalf("authorization = %q, want Bearer workspace-key", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	restore := imagerouter.SetBaseURLForTest(srv.URL + "/v1")
	defer restore()

	_, err := s.runImageRouterJob(
		context.Background(),
		"ws_1",
		seedSourceImage(t, s),
		func(client *imagerouter.Client, _ []byte) ([]byte, error) {
			return nil, client.Validate(context.Background())
		},
	)
	if err != nil {
		t.Fatalf("runImageRouterJob: %v", err)
	}
}

func TestRunImageRouterJobFallsBackToEnv(t *testing.T) {
	s := newImageRouterJobServer(t, func(_ context.Context, workspaceID string) (string, imagerouter.KeySource, error) {
		if workspaceID != "ws_1" {
			t.Fatalf("workspaceID = %q", workspaceID)
		}
		return "env-key", imagerouter.SourceEnv, nil
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer env-key" {
			t.Fatalf("authorization = %q, want Bearer env-key", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	restore := imagerouter.SetBaseURLForTest(srv.URL + "/v1")
	defer restore()

	_, err := s.runImageRouterJob(
		context.Background(),
		"ws_1",
		seedSourceImage(t, s),
		func(client *imagerouter.Client, _ []byte) ([]byte, error) {
			return nil, client.Validate(context.Background())
		},
	)
	if err != nil {
		t.Fatalf("runImageRouterJob: %v", err)
	}
}

func TestRunImageRouterJobMissingKeyFailsCleanly(t *testing.T) {
	s := newImageRouterJobServer(t, func(context.Context, string) (string, imagerouter.KeySource, error) {
		return "", imagerouter.SourceNone, nil
	})

	_, err := s.runImageRouterJob(
		context.Background(),
		"ws_1",
		seedSourceImage(t, s),
		func(client *imagerouter.Client, _ []byte) ([]byte, error) {
			return nil, client.Validate(context.Background())
		},
	)
	if !errors.Is(err, imagerouter.ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestRunImageRouterJobCorruptWorkspaceCiphertextFailsCleanly(t *testing.T) {
	want := errors.New("decrypt failed")
	s := newImageRouterJobServer(t, func(context.Context, string) (string, imagerouter.KeySource, error) {
		return "", imagerouter.SourceNone, want
	})

	_, err := s.runImageRouterJob(
		context.Background(),
		"ws_1",
		seedSourceImage(t, s),
		func(client *imagerouter.Client, _ []byte) ([]byte, error) {
			return nil, client.Validate(context.Background())
		},
	)
	if !errors.Is(err, want) {
		t.Fatalf("expected %v, got %v", want, err)
	}
}
