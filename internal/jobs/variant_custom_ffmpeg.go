package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"damask/server/internal/telemetry"
	"damask/server/internal/transform"

	"go.opentelemetry.io/otel/attribute"
)

// CustomFFmpegParams holds the user-supplied command for the custom_ffmpeg
// variant type. {input} and {output} are substituted with real temp paths
// at execution time — see transform.RunCustomFFmpeg.
type CustomFFmpegParams struct {
	Command string `json:"command"`
}

// customFFmpegBuild is the variantBuildFn for the custom_ffmpeg variant type.
func (s *JobServer) customFFmpegBuild(_, _, _ string, params json.RawMessage) (variantTransformer, error) {
	return s.customFFmpegTransformer(params)
}

// customFFmpegCanonical returns canonical JSON for custom_ffmpeg params.
func customFFmpegCanonical(_, _ string, params json.RawMessage) (string, error) {
	var p CustomFFmpegParams
	_ = json.Unmarshal(params, &p)
	b, err := json.Marshal(p)
	return string(b), err
}

// customFFmpegTransformer returns a variantTransformer that runs a
// user-supplied ffmpeg command against the source file and detects the
// output MIME type via ffprobe.
func (s *JobServer) customFFmpegTransformer(params json.RawMessage) (variantTransformer, error) {
	if !s.trf.FFmpegAvailable() {
		return nil, errors.New("ffmpeg not found in PATH")
	}
	var p CustomFFmpegParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("parse custom ffmpeg params: %w", err)
	}
	if err := transform.ValidateCustomCommand(p.Command); err != nil {
		return nil, err
	}
	return func(ctx context.Context, sourceKey string) ([]byte, string, error) {
		ctx, span := telemetry.StartBackgroundSpan(ctx, "variant.transform",
			attribute.String("damask.variant_type", "custom_ffmpeg"),
		)
		rc, err := s.storage.Get(sourceKey)
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", err
		}
		defer rc.Close()
		srcPath, cleanSrc, err := writeToTempFile(ctx, rc, filepath.Ext(sourceKey))
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("write src temp: %w", err)
		}
		defer cleanSrc()

		outDir, err := os.MkdirTemp("", "damask-custom-ffmpeg-*")
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("create temp dir: %w", err)
		}
		defer os.RemoveAll(outDir)

		outputPath, runErr := s.trf.RunCustomFFmpeg(ctx, p.Command, srcPath, outDir)
		if runErr != nil {
			telemetry.EndSpan(span, runErr)
			return nil, "", runErr
		}

		data, readErr := os.ReadFile(outputPath)
		if readErr != nil {
			telemetry.EndSpan(span, readErr)
			return nil, "", fmt.Errorf("read output: %w", readErr)
		}

		mimeType := s.trf.DetectOutputMIME(ctx, outputPath)
		telemetry.EndSpan(span, nil)
		return data, mimeType, nil
	}, nil
}
