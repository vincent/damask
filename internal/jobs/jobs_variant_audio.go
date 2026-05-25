package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
	"damask/server/internal/transform"
)

// audioTransformer returns a variantTransformer for the given audio job type.
func (s *JobServer) audioTransformer(jobType, sourceMime string, params json.RawMessage) (variantTransformer, error) {
	if !s.trf.FFmpegAvailable() {
		return nil, errors.New("ffmpeg not found in PATH")
	}
	var p transform.AudioParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("parse audio params: %w", err)
		}
	}
	p = normalizeAudioJobParams(jobType, sourceMime, p)
	return func(ctx context.Context, sourceKey string) ([]byte, string, error) {
		rc, err := s.storage.Get(sourceKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()

		srcExt := filepath.Ext(sourceKey)
		srcPath, cleanSrc, err := writeToTempFile(ctx, rc, srcExt)
		if err != nil {
			return nil, "", fmt.Errorf("write src temp: %w", err)
		}
		defer cleanSrc()

		ext := transform.AudioExtension(p.OutputFormat)
		dstPath := srcPath + "_out" + ext
		defer os.Remove(dstPath)

		if _, err := s.runAudioVariant(ctx, jobType, srcPath, dstPath, p); err != nil {
			return nil, "", err
		}
		data, err := os.ReadFile(dstPath)
		if err != nil {
			return nil, "", fmt.Errorf("read output: %w", err)
		}
		return data, transform.AudioMimeType(p.OutputFormat), nil
	}, nil
}

func (s *JobServer) jobAudioTransform(ctx context.Context, job dbgen.Job) error {
	var p VariantJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}
	trf, err := s.audioTransformer(job.Type, p.MimeType, p.Params)
	if err != nil {
		return err
	}
	data, contentType, err := trf(ctx, p.StorageKey)
	if err != nil {
		return err
	}

	// Audio uses the canonical params (post-normalization) for the storage key.
	var canonicalParams transform.AudioParams
	if len(p.Params) > 0 {
		_ = json.Unmarshal(p.Params, &canonicalParams)
	}
	canonicalParams = normalizeAudioJobParams(job.Type, p.MimeType, canonicalParams)
	canonicalJSON, _ := json.Marshal(canonicalParams)
	cj := string(canonicalJSON)

	return s.finalizeRebuildVariant(ctx, assetVersionFromPayload(p), job.Type, cj, CanonicalParamsHash(cj), data, contentType)
}

func (s *JobServer) rebuildAudioVariant(
	ctx context.Context,
	ver dbgen.AssetVersion,
	variantType, paramsJSON, paramsHash string,
) error {
	trf, err := s.audioTransformer(variantType, ver.MimeType, json.RawMessage(paramsJSON))
	if err != nil {
		return err
	}
	data, contentType, err := trf(ctx, ver.StorageKey)
	if err != nil {
		return err
	}
	return s.finalizeRebuildVariant(ctx, ver, variantType, paramsJSON, paramsHash, data, contentType)
}

// runAudioVariant dispatches the ffmpeg audio operation for variantType and returns
// the canonical JSON encoding of the resolved params.
func (s *JobServer) runAudioVariant(
	ctx context.Context,
	variantType, srcPath, dstPath string,
	params transform.AudioParams,
) (string, error) {
	var err error
	switch variantType {
	case queue.JobTypeExtractAudio:
		err = s.trf.ExtractAudio(ctx, srcPath, dstPath, params)
	case queue.JobTypeTranscodeAudio:
		err = s.trf.TranscodeAudio(ctx, srcPath, dstPath, params)
	case queue.JobTypeNormalizeAudio:
		err = s.trf.NormalizeAudio(ctx, srcPath, dstPath, params)
	default:
		return "", fmt.Errorf("unknown audio variant type: %s", variantType)
	}
	if errors.Is(err, transform.ErrNoAudioStream) {
		return "", fmt.Errorf("extract audio: %w", err)
	}
	if err != nil {
		return "", fmt.Errorf("audio transform: %w", err)
	}
	b, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("marshal audio params: %w", err)
	}
	return string(b), nil
}

func normalizeAudioJobParams(jobType, sourceMime string, p transform.AudioParams) transform.AudioParams {
	if p.OutputFormat == "" || p.OutputFormat == "source" {
		switch jobType {
		case queue.JobTypeExtractAudio:
			p.OutputFormat = "aac"
		case queue.JobTypeTranscodeAudio:
			p.OutputFormat = "mp3"
		case queue.JobTypeNormalizeAudio:
			p.OutputFormat = transform.AudioFormatFromMimeType(sourceMime)
		}
	}
	if p.Bitrate == "" {
		p.Bitrate = "192k"
	}
	if p.TargetLUFS == 0 {
		p.TargetLUFS = transform.DefaultLUFS
	}
	return p
}
