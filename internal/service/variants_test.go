package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/audit"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"
	"damask/server/internal/transform"
)

type systemTagServiceStub struct {
	resolveFn func(ctx context.Context, workspaceID, tagName string, scope service.SystemTagScope) (*service.AssetDTO, error)
}

func (s systemTagServiceStub) List(context.Context, string, bool) ([]*service.TagDTO, error) {
	return nil, nil
}

func (s systemTagServiceStub) GetByName(context.Context, string, string) (*service.TagDTO, error) {
	return nil, nil //nolint:nilnil // tests
}

func (s systemTagServiceStub) Create(context.Context, string, service.CreateTagParams) (*service.TagDTO, error) {
	return nil, nil //nolint:nilnil // tests
}

func (s systemTagServiceStub) Patch(context.Context, string, string, service.PatchTagParams) (*service.TagDTO, error) {
	return nil, nil //nolint:nilnil // tests
}

func (s systemTagServiceStub) EnsureSystemTag(context.Context, string, string) error { return nil }
func (s systemTagServiceStub) Delete(context.Context, string, []string) error        { return nil }
func (s systemTagServiceStub) BulkDelete(context.Context, string, []string) (service.BulkDeleteTagsResult, error) {
	return service.BulkDeleteTagsResult{}, nil
}
func (s systemTagServiceStub) Merge(context.Context, string, []string, string) (service.MergeTagsResult, error) {
	return service.MergeTagsResult{}, nil
}

func (s systemTagServiceStub) ResolveSystemTag(
	ctx context.Context,
	workspaceID, tagName string,
	scope service.SystemTagScope,
) (*service.AssetDTO, error) {
	if s.resolveFn != nil {
		return s.resolveFn(ctx, workspaceID, tagName, scope)
	}
	return nil, nil //nolint:nilnil // tests
}

func (s systemTagServiceStub) TouchLastUsed(context.Context, string, string) error { return nil }
func (s systemTagServiceStub) ListForAsset(context.Context, string) ([]*service.TagDTO, error) {
	return nil, nil
}
func (s systemTagServiceStub) AddToAsset(context.Context, string, string, string) (*service.TagDTO, error) {
	return nil, nil //nolint:nilnil // tests
}
func (s systemTagServiceStub) RemoveFromAsset(context.Context, string, string, string) error {
	return nil
}
func (s systemTagServiceStub) UpsertForAsset(context.Context, string, string, string) error {
	return nil
}
func (s systemTagServiceStub) BatchTagsForAssets(context.Context, []string) (map[string][]string, error) {
	return map[string][]string{}, nil
}

func newVariantSvc(t *testing.T) (service.VariantService, *memory.RealVariantRepo, *memory.AssetRepo) {
	t.Helper()
	varRepo := memory.NewRealVariantRepo()
	assetRepo := memory.NewAssetRepo()
	return service.NewVariantService(varRepo, assetRepo, nil, audit.NopWriter{}), varRepo, assetRepo
}

func newVariantSvcWithTags(
	t *testing.T,
	tags service.TagService,
) (service.VariantService, *memory.AssetRepo) {
	t.Helper()
	varRepo := memory.NewRealVariantRepo()
	assetRepo := memory.NewAssetRepo()
	return service.NewVariantService(varRepo, assetRepo, tags, audit.NopWriter{}), assetRepo
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
	result, err := svc.List(context.Background(), service.ListVariantsParams{
		WorkspaceID: "ws_1",
		AssetID:     "ast_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Variants) != 0 {
		t.Errorf("expected empty list, got %d", len(result.Variants))
	}
	if result.CoveringWorkflow != nil {
		t.Errorf("expected no covering workflow, got %+v", result.CoveringWorkflow)
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
	if dto.Title != "image_resize #1" {
		t.Fatalf("Title: got %q, want auto title", dto.Title)
	}
}

func TestAutoTitle(t *testing.T) {
	if got := service.AutoTitle("bg_remove", 2); got != "bg_remove #2" {
		t.Fatalf("AutoTitle() = %q", got)
	}
}

func TestResolvedTitle_Custom(t *testing.T) {
	title := "Hero shot clean"
	got := service.ResolvedTitle(repository.Variant{Type: "image_resize", Title: &title}, 3)
	if got != title {
		t.Fatalf("ResolvedTitle() = %q, want %q", got, title)
	}
}

func TestResolvedTitle_Nil(t *testing.T) {
	got := service.ResolvedTitle(repository.Variant{Type: "image_resize"}, 3)
	if got != "image_resize #3" {
		t.Fatalf("ResolvedTitle() = %q", got)
	}
}

func TestVariantService_UpdateTitle_Happy(t *testing.T) {
	svc, repo, _ := newVariantSvc(t)
	createdAt := time.Now()
	repo.Seed(repository.Variant{
		ID:             "var_1",
		WorkspaceID:    "ws_1",
		AssetVersionID: "ver_1",
		Type:           "image_resize",
		StorageKey:     "variants/var_1.jpg",
		CreatedAt:      createdAt,
	})

	if err := svc.UpdateTitle(context.Background(), "ws_1", "var_1", "Hero shot clean"); err != nil {
		t.Fatalf("UpdateTitle() error = %v", err)
	}

	dto, err := svc.Get(context.Background(), "ws_1", "var_1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if dto.Title != "Hero shot clean" {
		t.Fatalf("Title = %q", dto.Title)
	}
}

func TestVariantService_UpdateTitle_Clears(t *testing.T) {
	svc, repo, _ := newVariantSvc(t)
	title := "Custom"
	repo.Seed(repository.Variant{
		ID:             "var_1",
		WorkspaceID:    "ws_1",
		AssetVersionID: "ver_1",
		Type:           "image_resize",
		StorageKey:     "variants/var_1.jpg",
		Title:          &title,
		CreatedAt:      time.Now(),
	})

	if err := svc.UpdateTitle(context.Background(), "ws_1", "var_1", "   "); err != nil {
		t.Fatalf("UpdateTitle() error = %v", err)
	}
	dto, err := svc.Get(context.Background(), "ws_1", "var_1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if dto.Title != "image_resize #1" {
		t.Fatalf("Title = %q", dto.Title)
	}
}

func TestVariantService_UpdateTitle_TooLong(t *testing.T) {
	svc, repo, _ := newVariantSvc(t)
	repo.Seed(repository.Variant{
		ID:             "var_1",
		WorkspaceID:    "ws_1",
		AssetVersionID: "ver_1",
		Type:           "image_resize",
		StorageKey:     "variants/var_1.jpg",
		CreatedAt:      time.Now(),
	})

	err := svc.UpdateTitle(context.Background(), "ws_1", "var_1", strings.Repeat("a", 256))
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestVariantService_UpdateSharing_Mixed(t *testing.T) {
	svc, repo, _ := newVariantSvc(t)
	repo.Seed(
		repository.Variant{
			ID:             "var_1",
			WorkspaceID:    "ws_1",
			AssetVersionID: "ver_1",
			Type:           "image_resize",
			StorageKey:     "a",
			CreatedAt:      time.Now(),
		},
		repository.Variant{
			ID:             "var_2",
			WorkspaceID:    "ws_1",
			AssetVersionID: "ver_1",
			Type:           "image_resize",
			StorageKey:     "b",
			IsShared:       true,
			CreatedAt:      time.Now().Add(time.Second),
		},
	)

	err := svc.UpdateSharing(context.Background(), service.UpdateVariantsSharingParams{
		WorkspaceID: "ws_1",
		AssetID:     "asset_1",
		Updates: map[string]bool{
			"var_1": true,
			"var_2": false,
		},
	})
	if err != nil {
		t.Fatalf("UpdateSharing() error = %v", err)
	}

	dto1, _ := svc.Get(context.Background(), "ws_1", "var_1")
	dto2, _ := svc.Get(context.Background(), "ws_1", "var_2")
	if !dto1.IsShared || dto2.IsShared {
		t.Fatalf("unexpected sharing flags: var_1=%v var_2=%v", dto1.IsShared, dto2.IsShared)
	}
}

func TestVariantService_UpdateSharing_Unknown(t *testing.T) {
	svc, repo, _ := newVariantSvc(t)
	repo.Seed(repository.Variant{
		ID:             "var_1",
		WorkspaceID:    "ws_1",
		AssetVersionID: "ver_1",
		Type:           "image_resize",
		StorageKey:     "variants/var_1.jpg",
		CreatedAt:      time.Now(),
	})

	err := svc.UpdateSharing(context.Background(), service.UpdateVariantsSharingParams{
		WorkspaceID: "ws_1",
		AssetID:     "asset_1",
		Updates:     map[string]bool{"missing": true},
	})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
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
	if decodeErr := json.Unmarshal(prepared.Params, &params); decodeErr != nil {
		t.Fatalf("decode params: %v", decodeErr)
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
	if decodeErr := json.Unmarshal(prepared.Params, &params); decodeErr != nil {
		t.Fatalf("decode params: %v", decodeErr)
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
	if decodeErr := json.Unmarshal(prepared.Params, &params); decodeErr != nil {
		t.Fatalf("decode params: %v", decodeErr)
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
		{
			"image transform on audio",
			queue.JobTypeImageResize,
			"audio/mpeg",
			"image transforms require an image asset",
			service.ErrInvalidVariantReq,
		},
		{
			"image with prompt on audio",
			queue.JobTypeImageWithPrompt,
			"audio/mpeg",
			"image transforms require an image asset",
			service.ErrInvalidVariantReq,
		},
		{
			"video transform on image",
			queue.JobTypeVideoTranscode,
			"image/jpeg",
			"video transforms require a video asset",
			service.ErrInvalidVariantReq,
		},
		{"extract audio on image", queue.JobTypeExtractAudio, "image/jpeg", "asset_not_video", apperr.ErrInvalidInput},
		{
			"audio transform on video",
			queue.JobTypeTranscodeAudio,
			"video/mp4",
			"asset_not_audio",
			apperr.ErrInvalidInput,
		},
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

func TestVariantService_PrepareCreate_ImageBgRemoveDefaultsModel(t *testing.T) {
	svc, _, _ := newVariantSvc(t)
	prepared, err := svc.PrepareCreate(context.Background(), service.PrepareCreateVariantParams{
		Type:                  queue.JobTypeImageBgRemove,
		Params:                json.RawMessage(`{}`),
		AssetMimeType:         "image/png",
		ImageRouterConfigured: true,
		DefaultBgRemoveModel:  "bria/remove-background",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var params map[string]string
	if decodeErr := json.Unmarshal(prepared.Params, &params); decodeErr != nil {
		t.Fatalf("decode params: %v", decodeErr)
	}
	if params["model"] != "bria/remove-background" {
		t.Fatalf("expected default model, got %#v", params)
	}
}

func TestVariantService_PrepareCreate_ImageWithPromptNormalizesPrompt(t *testing.T) {
	svc, _, _ := newVariantSvc(t)
	prepared, err := svc.PrepareCreate(context.Background(), service.PrepareCreateVariantParams{
		Type:                  queue.JobTypeImageWithPrompt,
		Params:                json.RawMessage(`{"prompt":"  add soft shadows  "}`),
		AssetMimeType:         "image/png",
		ImageRouterConfigured: true,
		DefaultImageModel:     "black-forest-labs/FLUX.1-fill-dev",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var params map[string]string
	if decodeErr := json.Unmarshal(prepared.Params, &params); decodeErr != nil {
		t.Fatalf("decode params: %v", decodeErr)
	}
	if params["prompt"] != "add soft shadows" {
		t.Fatalf("expected trimmed prompt, got %#v", params["prompt"])
	}
	if params["model"] != "black-forest-labs/FLUX.1-fill-dev" {
		t.Fatalf("expected default model, got %#v", params["model"])
	}
}

func TestVariantService_PrepareCreate_ImageWithPromptRequiresPrompt(t *testing.T) {
	svc, _, _ := newVariantSvc(t)
	_, err := svc.PrepareCreate(context.Background(), service.PrepareCreateVariantParams{
		Type:                  queue.JobTypeImageWithPrompt,
		Params:                json.RawMessage(`{"prompt":"   "}`),
		AssetMimeType:         "image/png",
		ImageRouterConfigured: true,
		DefaultImageModel:     "black-forest-labs/FLUX.1-fill-dev",
	})
	if !errors.Is(err, service.ErrInvalidVariantReq) || !strings.Contains(err.Error(), "prompt_required") {
		t.Fatalf("expected prompt_required invalid request, got %v", err)
	}
}

func TestVariantService_PrepareCreate_ImageRouterRequiresConfig(t *testing.T) {
	svc, _, _ := newVariantSvc(t)
	_, err := svc.PrepareCreate(context.Background(), service.PrepareCreateVariantParams{
		Type:          queue.JobTypeImageBgRemove,
		Params:        json.RawMessage(`{}`),
		AssetMimeType: "image/png",
	})
	if !errors.Is(err, apperr.ErrInvalidInput) || !strings.Contains(err.Error(), "imagerouter_not_configured") {
		t.Fatalf("expected imagerouter_not_configured invalid input, got %v", err)
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
	svc, assetRepo := newVariantSvcWithTags(t, systemTagServiceStub{
		resolveFn: func(_ context.Context, workspaceID, tagName string, scope service.SystemTagScope) (*service.AssetDTO, error) {
			if workspaceID != "ws_1" || tagName != "_watermark" {
				t.Fatalf("unexpected lookup args workspace=%s tag=%s", workspaceID, tagName)
			}
			if scope.FolderID == nil || *scope.FolderID != "fld_1" {
				t.Fatalf("unexpected scope: %+v", scope)
			}
			return &service.AssetDTO{ID: "wm_1"}, nil
		},
	})
	projectID := "prj_1"
	folderID := "fld_1"
	assetRepo.Seed(repository.Asset{ID: "ast_1", WorkspaceID: "ws_1", ProjectID: &projectID, FolderID: &folderID})

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
	if decodeErr := json.Unmarshal(prepared.Params, &params); decodeErr != nil {
		t.Fatalf("decode params: %v", decodeErr)
	}
	if params.WatermarkAssetID != "wm_1" {
		t.Fatalf("expected watermark asset id wm_1, got %s", params.WatermarkAssetID)
	}
	if params.Opacity != 0.4 {
		t.Fatalf("unexpected params: %+v", params)
	}
}

func TestVariantService_PrepareCreate_VideoWatermarkInjectsAssetID(t *testing.T) {
	svc, assetRepo := newVariantSvcWithTags(t, systemTagServiceStub{
		resolveFn: func(_ context.Context, workspaceID, tagName string, scope service.SystemTagScope) (*service.AssetDTO, error) {
			if workspaceID != "ws_1" || tagName != "_watermark" {
				t.Fatalf("unexpected lookup args workspace=%s tag=%s", workspaceID, tagName)
			}
			if scope.ProjectID == nil || *scope.ProjectID != "prj_1" {
				t.Fatalf("unexpected scope: %+v", scope)
			}
			return &service.AssetDTO{ID: "wm_1"}, nil
		},
	})
	projectID := "prj_1"
	assetRepo.Seed(repository.Asset{ID: "vid_1", WorkspaceID: "ws_1", ProjectID: &projectID})

	prepared, err := svc.PrepareCreate(context.Background(), service.PrepareCreateVariantParams{
		WorkspaceID:   "ws_1",
		AssetID:       "vid_1",
		Type:          queue.JobTypeVideoWatermark,
		Params:        json.RawMessage(`{"opacity":0.35,"format":"webm","strip_audio":true}`),
		AssetMimeType: "video/mp4",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var params transform.VideoWatermarkParams
	if decodeErr := json.Unmarshal(prepared.Params, &params); decodeErr != nil {
		t.Fatalf("decode params: %v", decodeErr)
	}
	if params.WatermarkAssetID != "wm_1" {
		t.Fatalf("expected watermark asset id wm_1, got %s", params.WatermarkAssetID)
	}
	if params.Opacity != 0.35 || params.Format != "webm" || !params.StripAudio {
		t.Fatalf("unexpected params: %+v", params)
	}
}

func TestVariantService_PrepareCreate_WatermarkMissingReturnsInvalidInput(t *testing.T) {
	svc, assetRepo := newVariantSvcWithTags(t, systemTagServiceStub{
		resolveFn: func(_ context.Context, _, _ string, _ service.SystemTagScope) (*service.AssetDTO, error) {
			return nil, apperr.ErrNotFound
		},
	})
	assetRepo.Seed(repository.Asset{ID: "ast_1", WorkspaceID: "ws_1"})

	_, err := svc.PrepareCreate(context.Background(), service.PrepareCreateVariantParams{
		WorkspaceID:   "ws_1",
		AssetID:       "ast_1",
		Type:          queue.JobTypeImageWatermark,
		Params:        json.RawMessage(`{}`),
		AssetMimeType: "image/jpeg",
	})
	if !errors.Is(err, apperr.ErrInvalidInput) || !strings.Contains(err.Error(), "no_watermark_asset") {
		t.Fatalf("expected watermark missing invalid input, got %v", err)
	}
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
