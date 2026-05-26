package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"damask/server/internal/imagerouter"
	"damask/server/internal/telemetry"

	"go.opentelemetry.io/otel/attribute"
)

// imageBgRemoveBuild is the variantBuildFn for background removal.
func (s *JobServer) imageBgRemoveBuild(_, _, workspaceID string, params json.RawMessage) (variantTransformer, error) {
	return s.imageBgRemoveTransformer(workspaceID, params)
}

// imageBgRemoveTransformer returns a variantTransformer for background removal.
func (s *JobServer) imageBgRemoveTransformer(workspaceID string, params json.RawMessage) (variantTransformer, error) {
	var p struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("parse bg remove params: %w", err)
	}
	return func(ctx context.Context, sourceKey string) ([]byte, string, error) {
		ctx, span := telemetry.StartBackgroundSpan(ctx, "variant.transform",
			attribute.String("damask.variant_type", "image_bg_remove"),
		)
		result, err := s.runImageRouterJob(ctx, workspaceID, sourceKey,
			func(client *imagerouter.Client, imageData []byte) ([]byte, error) {
				return client.BgRemove(ctx, imageData, imagerouter.BgRemoveParams{
					Model:  p.Model,
					Prompt: p.Prompt,
				})
			},
		)
		if err != nil && strings.Contains(err.Error(), "Prompt must be a string") {
			err = errors.New(
				"the selected model is not available for background removal. please choose another model and try again",
			)
			telemetry.EndSpan(span, err)
			return nil, "", err
		}
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("remove background: %w", err)
		}
		telemetry.EndSpan(span, nil)
		return result, "image/png", nil
	}, nil
}

// imageWithPromptBuild is the variantBuildFn for AI image-with-prompt.
func (s *JobServer) imageWithPromptBuild(_, _, workspaceID string, params json.RawMessage) (variantTransformer, error) {
	return s.imageWithPromptTransformer(workspaceID, params)
}

// imageWithPromptTransformer returns a variantTransformer for AI image-with-prompt.
func (s *JobServer) imageWithPromptTransformer(workspaceID string, params json.RawMessage) (variantTransformer, error) {
	var p struct {
		Prompt string `json:"prompt"`
		Model  string `json:"model"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("parse image prompt params: %w", err)
	}
	return func(ctx context.Context, sourceKey string) ([]byte, string, error) {
		ctx, span := telemetry.StartBackgroundSpan(ctx, "variant.transform",
			attribute.String("damask.variant_type", "image_with_prompt"),
		)
		result, err := s.runImageRouterJob(ctx, workspaceID, sourceKey,
			func(client *imagerouter.Client, imageData []byte) ([]byte, error) {
				return client.Transform(ctx, imageData, imagerouter.PromptParams{
					Prompt: p.Prompt,
					Model:  p.Model,
				})
			},
		)
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("image transform with prompt: %w", err)
		}
		telemetry.EndSpan(span, nil)
		return result, "image/png", nil
	}, nil
}

func (s *JobServer) runImageRouterJob(
	ctx context.Context,
	workspaceID string,
	sourceKey string,
	callFn func(*imagerouter.Client, []byte) ([]byte, error),
) ([]byte, error) {
	ctx, span := telemetry.StartBackgroundSpan(ctx, "imagerouter.job",
		attribute.String("damask.workspace_id", workspaceID),
	)

	rc, err := s.storage.Get(sourceKey)
	if err != nil {
		telemetry.EndSpan(span, err)
		return nil, fmt.Errorf("get source: %w", err)
	}
	defer rc.Close()

	imageData, err := io.ReadAll(rc)
	if err != nil {
		telemetry.EndSpan(span, err)
		return nil, fmt.Errorf("read source: %w", err)
	}

	key, source, err := s.imgKeyResolver(ctx, workspaceID)
	if err != nil {
		telemetry.EndSpan(span, err)
		return nil, err
	}
	if source == imagerouter.SourceNone {
		telemetry.EndSpan(span, imagerouter.ErrNotConfigured)
		return nil, imagerouter.ErrNotConfigured
	}

	client := imagerouter.NewClient(key, s.cfg.ImageRouter.RetryPaidOnFreeLimit)
	result, err := callFn(client, imageData)
	telemetry.EndSpan(span, err)
	if err != nil {
		return nil, err
	}
	return result, nil
}
