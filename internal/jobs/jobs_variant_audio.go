package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
	"damask/server/internal/storage"
	"damask/server/internal/transform"

	"github.com/google/uuid"
)

func (s *JobServer) jobAudioTransform(ctx context.Context, job dbgen.Job) error {
	if !s.trf.FFmpegAvailable() {
		return errors.New("ffmpeg not found in PATH")
	}

	var p VariantJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	var params transform.AudioParams
	if len(p.Params) > 0 {
		if err := json.Unmarshal(p.Params, &params); err != nil {
			return fmt.Errorf("parse audio params: %w", err)
		}
	}
	params = normalizeAudioJobParams(job.Type, p.MimeType, params)

	rc, err := s.storage.Get(p.StorageKey)
	if err != nil {
		return err
	}
	defer rc.Close()

	srcExt := filepath.Ext(p.StorageKey)
	srcPath, cleanSrc, err := writeToTempFile(ctx, rc, srcExt)
	if err != nil {
		return fmt.Errorf("write src temp: %w", err)
	}
	defer cleanSrc()

	ext := transform.AudioExtension(params.OutputFormat)
	dstPath := srcPath + "_out" + ext
	defer os.Remove(dstPath)

	canonicalParams, err := s.runAudioVariant(ctx, job.Type, srcPath, dstPath, params)
	if err != nil {
		return err
	}

	paramsHash := canonicalParamsHash(canonicalParams)
	storageKey := storage.VersionedVariantKey(p.WorkspaceID, p.AssetID, p.VersionNum, job.Type, paramsHash, ext)

	dstData, err := os.ReadFile(dstPath)
	if err != nil {
		return fmt.Errorf("read output: %w", err)
	}
	if err := s.storage.Put(storageKey, bytes.NewReader(dstData)); err != nil {
		return fmt.Errorf("store variant: %w", err)
	}

	sz := int64(len(dstData))
	variantID := p.VariantID
	if variantID == "" {
		variantID = uuid.NewString()
	}
	_, err = s.db.CreateVariant(ctx, dbgen.CreateVariantParams{
		ID:              variantID,
		WorkspaceID:     p.WorkspaceID,
		AssetVersionID:  p.VersionID,
		Type:            job.Type,
		StorageKey:      storageKey,
		TransformParams: &canonicalParams,
		Size:            &sz,
	})
	if err == nil {
		s.publishVariantReady(ctx, p.WorkspaceID, p.AssetID, variantID)
		s.enqueueVariantThumb(ctx, p, variantID, storageKey, transform.AudioMimeType(params.OutputFormat))
	}
	return err
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

func (s *JobServer) rebuildAudioVariant(
	ctx context.Context,
	ver dbgen.AssetVersion,
	variantType, paramsJSON, paramsHash string,
) error {
	if !s.trf.FFmpegAvailable() {
		return errors.New("ffmpeg not found in PATH")
	}

	var params transform.AudioParams
	if paramsJSON != "" && paramsJSON != "{}" {
		if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
			return fmt.Errorf("parse audio params: %w", err)
		}
	}
	params = normalizeAudioJobParams(variantType, ver.MimeType, params)

	rc, err := s.storage.Get(ver.StorageKey)
	if err != nil {
		return err
	}
	defer rc.Close()

	srcExt := filepath.Ext(ver.StorageKey)
	srcPath, cleanSrc, err := writeToTempFile(ctx, rc, srcExt)
	if err != nil {
		return fmt.Errorf("write src temp: %w", err)
	}
	defer cleanSrc()

	ext := transform.AudioExtension(params.OutputFormat)
	dstPath := srcPath + "_out" + ext
	defer os.Remove(dstPath)

	canonicalParams, err := s.runAudioVariant(ctx, variantType, srcPath, dstPath, params)
	if err != nil {
		return err
	}

	dstData, err := os.ReadFile(dstPath)
	if err != nil {
		return fmt.Errorf("read output: %w", err)
	}

	storageKey := storage.VersionedVariantKey(
		ver.WorkspaceID,
		ver.AssetID,
		ver.VersionNum,
		variantType,
		paramsHash,
		ext,
	)
	if err := s.storage.Put(storageKey, bytes.NewReader(dstData)); err != nil {
		return fmt.Errorf("store variant: %w", err)
	}

	sz := int64(len(dstData))
	vid := uuid.NewString()
	_, err = s.db.CreateVariant(ctx, dbgen.CreateVariantParams{
		ID:              vid,
		WorkspaceID:     ver.WorkspaceID,
		AssetVersionID:  ver.ID,
		Type:            variantType,
		StorageKey:      storageKey,
		TransformParams: &canonicalParams,
		Size:            &sz,
	})
	if err == nil {
		s.publishVariantReady(ctx, ver.WorkspaceID, ver.AssetID, vid)
		s.enqueueVariantThumbRaw(
			ctx,
			ver.WorkspaceID,
			ver.AssetID,
			vid,
			storageKey,
			transform.AudioMimeType(params.OutputFormat),
		)
	}
	return err
}
