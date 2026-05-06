package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/audit"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"
	"damask/server/internal/transform"
)

type watermarkServiceStub struct {
	resolveFn func(ctx context.Context, workspaceID, assetID string) (*service.WatermarkAssetDTO, error)
}

func (s watermarkServiceStub) ResolveWatermarkAsset(ctx context.Context, workspaceID, assetID string) (*service.WatermarkAssetDTO, error) {
	if s.resolveFn != nil {
		return s.resolveFn(ctx, workspaceID, assetID)
	}
	return nil, nil
}

func newVariantSvc(t *testing.T) (service.VariantService, *memory.RealVariantRepo, *memory.AssetRepo) {
	t.Helper()
	varRepo := memory.NewRealVariantRepo()
	assetRepo := memory.NewAssetRepo()
	return service.NewVariantService(varRepo, assetRepo, nil, audit.NopWriter{}), varRepo, assetRepo
}

func newVariantSvcWithWatermarks(t *testing.T, wm service.WatermarkService) (service.VariantService, *memory.RealVariantRepo, *memory.AssetRepo) {
	t.Helper()
	varRepo := memory.NewRealVariantRepo()
	assetRepo := memory.NewAssetRepo()
	return service.NewVariantService(varRepo, assetRepo, wm, audit.NopWriter{}), varRepo, assetRepo
}

func newVariantSvcSpy(t *testing.T) (service.VariantService, *memory.RealVariantRepo, *memory.AssetRepo, *spyWriter) {
	t.Helper()
	spy := newSpy()
	varRepo := memory.NewRealVariantRepo()
	assetRepo := memory.NewAssetRepo()
	return service.NewVariantService(varRepo, assetRepo, nil, spy), varRepo, assetRepo, spy
}

// --- List ---

func TestVariantService_List_Empty(t *testing.T) {
	svc, _, assetRepo := newVariantSvc(t)
	assetRepo.Seed(repository.Asset{ID: "ast_1", WorkspaceID: "ws_1"})
	out, err := svc.List(context.Background(), "ws_1", "ast_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected empty list, got %d", len(out))
	}
}

// --- Get ---

func TestVariantService_Get_NotFound(t *testing.T) {
	svc, _, _ := newVariantSvc(t)
	_, err := svc.Get(context.Background(), "ws_1", "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestVariantService_Get_OK(t *testing.T) {
	svc, varRepo, _ := newVariantSvc(t)
	varRepo.Seed(repository.Variant{
		ID:             "var_1",
		WorkspaceID:    "ws_1",
		AssetVersionID: "v1",
		Type:           "image_resize",
		StorageKey:     "variants/var_1.jpg",
	})
	dto, err := svc.Get(context.Background(), "ws_1", "var_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Type != "image_resize" {
		t.Errorf("Type: got %q, want %q", dto.Type, "image_resize")
	}
}

// --- PrepareCreate ---

func TestVariantService_PrepareCreate_ExtractAudioDefaults(t *testing.T) {
	svc, _, _ := newVariantSvc(t)
	prepared, err := svc.PrepareCreate(context.Background(), service.PrepareCreateVariantParams{
		Type:          queue.JobTypeExtractAudio,
		Params:        json.RawMessage(`{}`),
		AssetMimeType: "video/mp4",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var params transform.AudioParams
	if err := json.Unmarshal(prepared.Params, &params); err != nil {
		t.Fatalf("decode params: %v", err)
	}
	if params.OutputFormat != "aac" || params.Bitrate != "192k" {
		t.Fatalf("unexpected params: %+v", params)
	}
}

func TestVariantService_PrepareCreate_TranscodeAudioDefaultsBitrate(t *testing.T) {
	svc, _, _ := newVariantSvc(t)
	prepared, err := svc.PrepareCreate(context.Background(), service.PrepareCreateVariantParams{
		Type:          queue.JobTypeTranscodeAudio,
		Params:        json.RawMessage(`{"format":"opus"}`),
		AssetMimeType: "audio/mpeg",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var params transform.AudioParams
	if err := json.Unmarshal(prepared.Params, &params); err != nil {
		t.Fatalf("decode params: %v", err)
	}
	if params.OutputFormat != "opus" || params.Bitrate != "192k" {
		t.Fatalf("unexpected params: %+v", params)
	}
}

func TestVariantService_PrepareCreate_NormalizeAudioSourceM4A(t *testing.T) {
	svc, _, _ := newVariantSvc(t)
	prepared, err := svc.PrepareCreate(context.Background(), service.PrepareCreateVariantParams{
		Type:          queue.JobTypeNormalizeAudio,
		Params:        json.RawMessage(`{"format":"source"}`),
		AssetMimeType: "audio/mp4",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var params transform.AudioParams
	if err := json.Unmarshal(prepared.Params, &params); err != nil {
		t.Fatalf("decode params: %v", err)
	}
	if params.OutputFormat != "aac" || params.TargetLUFS != -16 {
		t.Fatalf("unexpected params: %+v", params)
	}
}

func TestVariantService_PrepareCreate_RejectsUnsupportedAudioBitrate(t *testing.T) {
	svc, _, _ := newVariantSvc(t)
	_, err := svc.PrepareCreate(context.Background(), service.PrepareCreateVariantParams{
		Type:          queue.JobTypeTranscodeAudio,
		Params:        json.RawMessage(`{"format":"mp3","bitrate":"500k"}`),
		AssetMimeType: "audio/mpeg",
	})
	if !errors.Is(err, apperr.ErrInvalidInput) || !strings.Contains(err.Error(), "unsupported audio bitrate") {
		t.Fatalf("expected unsupported bitrate invalid input, got %v", err)
	}
}

func TestVariantService_PrepareCreate_RejectsWrongMimeFamilies(t *testing.T) {
	svc, _, _ := newVariantSvc(t)
	tests := []struct {
		name        string
		variantType string
		mimeType    string
		wantMessage string
		wantErr     error
	}{
		{"image transform on audio", queue.JobTypeImageResize, "audio/mpeg", "image transforms require an image asset", service.ErrInvalidVariantReq},
		{"video transform on image", queue.JobTypeVideoTranscode, "image/jpeg", "video transforms require a video asset", service.ErrInvalidVariantReq},
		{"extract audio on image", queue.JobTypeExtractAudio, "image/jpeg", "asset_not_video", apperr.ErrInvalidInput},
		{"audio transform on video", queue.JobTypeTranscodeAudio, "video/mp4", "asset_not_audio", apperr.ErrInvalidInput},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.PrepareCreate(context.Background(), service.PrepareCreateVariantParams{
				Type:          tt.variantType,
				Params:        json.RawMessage(`{"format":"mp3"}`),
				AssetMimeType: tt.mimeType,
			})
			if !errors.Is(err, tt.wantErr) || !strings.Contains(err.Error(), tt.wantMessage) {
				t.Fatalf("expected %q invalid input, got %v", tt.wantMessage, err)
			}
		})
	}
}

func TestVariantService_PrepareCreate_RejectsInvalidType(t *testing.T) {
	svc, _, _ := newVariantSvc(t)
	_, err := svc.PrepareCreate(context.Background(), service.PrepareCreateVariantParams{
		Type:          "not_a_variant",
		AssetMimeType: "image/jpeg",
	})
	if !errors.Is(err, service.ErrInvalidVariantType) {
		t.Fatalf("expected ErrInvalidVariantType, got %v", err)
	}
}

func TestVariantService_PrepareCreate_WatermarkInjectsAssetID(t *testing.T) {
	svc, _, _ := newVariantSvcWithWatermarks(t, watermarkServiceStub{
		resolveFn: func(_ context.Context, workspaceID, assetID string) (*service.WatermarkAssetDTO, error) {
			if workspaceID != "ws_1" || assetID != "ast_1" {
				t.Fatalf("unexpected lookup args workspace=%s asset=%s", workspaceID, assetID)
			}
			return &service.WatermarkAssetDTO{ID: "wm_1"}, nil
		},
	})

	prepared, err := svc.PrepareCreate(context.Background(), service.PrepareCreateVariantParams{
		WorkspaceID:   "ws_1",
		AssetID:       "ast_1",
		Type:          queue.JobTypeImageWatermark,
		Params:        json.RawMessage(`{"opacity":0.4}`),
		AssetMimeType: "image/jpeg",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var params transform.WatermarkParams
	if err := json.Unmarshal(prepared.Params, &params); err != nil {
		t.Fatalf("decode params: %v", err)
	}
	if params.WatermarkAssetID != "wm_1" {
		t.Fatalf("expected watermark asset id wm_1, got %s", params.WatermarkAssetID)
	}
	if params.Opacity != 0.4 {
		t.Fatalf("unexpected params: %+v", params)
	}
}

func TestVariantService_PrepareCreate_WatermarkMissingReturnsInvalidInput(t *testing.T) {
	svc, _, _ := newVariantSvcWithWatermarks(t, watermarkServiceStub{
		resolveFn: func(_ context.Context, _, _ string) (*service.WatermarkAssetDTO, error) {
			return nil, invalidWatermarkNotFound()
		},
	})

	_, err := svc.PrepareCreate(context.Background(), service.PrepareCreateVariantParams{
		WorkspaceID:   "ws_1",
		AssetID:       "ast_1",
		Type:          queue.JobTypeImageWatermark,
		Params:        json.RawMessage(`{}`),
		AssetMimeType: "image/jpeg",
	})
	if !errors.Is(err, apperr.ErrInvalidInput) || !strings.Contains(err.Error(), "no watermark asset found") {
		t.Fatalf("expected watermark missing invalid input, got %v", err)
	}
}

func invalidWatermarkNotFound() error {
	return fmt.Errorf("%w: %w", service.ErrNoWatermarkAsset, apperr.ErrInvalidInput)
}

// --- Delete ---

func TestVariantService_Delete_AssetNotFound(t *testing.T) {
	svc, _, _ := newVariantSvc(t)
	err := svc.Delete(context.Background(), "ws_1", "ast_nope", "var_1")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestVariantService_Delete_VariantNotFound(t *testing.T) {
	svc, _, assetRepo := newVariantSvc(t)
	currentVID := "v1"
	assetRepo.Seed(repository.Asset{ID: "ast_1", WorkspaceID: "ws_1", CurrentVersionID: &currentVID})
	err := svc.Delete(context.Background(), "ws_1", "ast_1", "var_nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestVariantService_Delete_OldVersion(t *testing.T) {
	svc, varRepo, assetRepo := newVariantSvc(t)
	currentVID := "v_current"
	assetRepo.Seed(repository.Asset{
		ID: "ast_1", WorkspaceID: "ws_1", CurrentVersionID: &currentVID,
	})
	varRepo.Seed(repository.Variant{
		ID:             "var_1",
		WorkspaceID:    "ws_1",
		AssetVersionID: "v_old", // not the current version
		Type:           "image_resize",
	})
	err := svc.Delete(context.Background(), "ws_1", "ast_1", "var_1")
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput (old version), got %v", err)
	}
}

func TestVariantService_Delete_OK(t *testing.T) {
	svc, varRepo, assetRepo := newVariantSvc(t)
	currentVID := "v1"
	assetRepo.Seed(repository.Asset{
		ID: "ast_1", WorkspaceID: "ws_1", CurrentVersionID: &currentVID,
	})
	varRepo.Seed(repository.Variant{
		ID:             "var_1",
		WorkspaceID:    "ws_1",
		AssetVersionID: currentVID,
		Type:           "image_resize",
	})
	if err := svc.Delete(context.Background(), "ws_1", "ast_1", "var_1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err := svc.Get(context.Background(), "ws_1", "var_1")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

// --- Audit events ---

func TestVariantService_Create_ManualUpload_EmitsAuditEvent(t *testing.T) {
	svc, _, assetRepo, spy := newVariantSvcSpy(t)
	assetRepo.Seed(repository.Asset{ID: "ast_1", WorkspaceID: "ws_1"})
	_, err := svc.Create(context.Background(), service.CreateVariantParams{
		WorkspaceID:    "ws_1",
		AssetID:        "ast_1", // non-empty → manual upload → triggers audit
		AssetVersionID: "v1",
		Type:           "image_resize",
		StorageKey:     "variants/x.jpg",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := spy.lastAsset()
	if e.EventType != audit.EventAssetVariantCreated {
		t.Errorf("EventType: got %q, want %q", e.EventType, audit.EventAssetVariantCreated)
	}
	if e.AssetID != "ast_1" {
		t.Errorf("AssetID: got %q, want %q", e.AssetID, "ast_1")
	}
}

func TestVariantService_Create_JobQueued_NoAudit(t *testing.T) {
	svc, _, _, spy := newVariantSvcSpy(t)
	_, err := svc.Create(context.Background(), service.CreateVariantParams{
		WorkspaceID:    "ws_1",
		AssetID:        "", // empty → job-enqueued → no audit in Create
		AssetVersionID: "v1",
		Type:           "image_resize",
		StorageKey:     "variants/x.jpg",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spy.assetCount() != 0 {
		t.Errorf("expected no audit event for job-queued variant, got %d", spy.assetCount())
	}
}

func TestVariantService_WriteVariantQueued_EmitsAuditEvent(t *testing.T) {
	svc, _, _, spy := newVariantSvcSpy(t)
	svc.WriteVariantQueued(context.Background(), "ws_1", "ast_1", "image_resize")
	e := spy.lastAsset()
	if e.EventType != audit.EventAssetVariantCreated {
		t.Errorf("EventType: got %q, want %q", e.EventType, audit.EventAssetVariantCreated)
	}
	if e.AssetID != "ast_1" {
		t.Errorf("AssetID: got %q, want %q", e.AssetID, "ast_1")
	}
}

func TestVariantService_Delete_EmitsAuditEvent(t *testing.T) {
	svc, varRepo, assetRepo, spy := newVariantSvcSpy(t)
	currentVID := "v1"
	assetRepo.Seed(repository.Asset{ID: "ast_1", WorkspaceID: "ws_1", CurrentVersionID: &currentVID})
	varRepo.Seed(repository.Variant{
		ID:             "var_1",
		WorkspaceID:    "ws_1",
		AssetVersionID: currentVID,
		Type:           "image_resize",
	})
	if err := svc.Delete(context.Background(), "ws_1", "ast_1", "var_1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := spy.lastAsset()
	if e.EventType != audit.EventAssetVariantDeleted {
		t.Errorf("EventType: got %q, want %q", e.EventType, audit.EventAssetVariantDeleted)
	}
	if e.AssetID != "ast_1" {
		t.Errorf("AssetID: got %q, want %q", e.AssetID, "ast_1")
	}
}
