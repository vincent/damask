package transform

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"os"
	"os/exec"
)

func MagikFirstThumbnail(ctx context.Context, src io.Reader, mimeType string) ([]byte, string, error) {
	ext := mimeToExt(mimeType)

	tmpPath, cleanup, err := writeToTempFile(ctx, src, ext)
	if err != nil {
		return nil, "", fmt.Errorf("temp file: %w", err)
	}
	defer cleanup()

	var buf bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx,
		"convert",
		tmpPath+"[0]",
		tmpPath+"_thumb"+".jpg",
	)

	cmd.Stdout = &buf
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, "", fmt.Errorf("convert failed: %w — stderr: %s", err, stderr.String())
	}

	thumbData, err := io.ReadAll(bytes.NewReader(buf.Bytes()))
	if err != nil {
		return nil, "", fmt.Errorf("read thumb: %w", err)
	}

	return thumbData, "image/jpeg", nil
}

// ---- OS helpers ----

func writeToTempFile(ctx context.Context, src io.Reader, ext string) (string, func(), error) {
	f, err := os.CreateTemp("", "damask-*"+ext)
	if err != nil {
		return "", nil, fmt.Errorf("create temp: %w", err)
	}
	if _, copyErr := io.Copy(f, src); copyErr != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", nil, fmt.Errorf("copy to temp: %w", copyErr)
	}
	err = f.Close()
	if err != nil {
		return "", nil, fmt.Errorf("close temp: %w", err)
	}
	return f.Name(), func() { _ = os.Remove(f.Name()) }, nil
}

func mimeToExt(ct string) string {
	ms, err := mime.ExtensionsByType(ct)
	if err == nil && len(ms) > 0 {
		return ms[0]
	}
	return "application/octet-stream"
}
