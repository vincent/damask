package transform

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
)

func AudioWaveform(ctx context.Context, src io.Reader, mimeType string) ([]byte, string, error) {
	ext := mimeToExt(mimeType)

	tmpPath, cleanup, err := writeToTempFile(ctx, src, ext)
	if err != nil {
		return nil, "", fmt.Errorf("temp file: %w", err)
	}
	defer cleanup()

	var buf bytes.Buffer
	var stderr bytes.Buffer
	output := tmpPath + "_thumb" + ".png"

	cmd := exec.CommandContext(ctx,
		"ffmpeg",
		"-i",
		tmpPath,
		"-filter_complex",
		"aformat=channel_layouts=mono,showwavespic=s=640x480:colors=#000000",
		"-frames:v",
		"1",
		output,
	)

	cmd.Stdout = &buf
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, "", fmt.Errorf("ffmpeg failed: %w — stderr: %s", err, stderr.String())
	}

	f, err := os.Open(output)
	if err != nil {
		return nil, "", fmt.Errorf("open thumb: %w", err)
	}
	defer f.Close()
	thumbData, err := io.ReadAll(f)
	if err != nil {
		return nil, "", fmt.Errorf("read thumb: %w", err)
	}

	return thumbData, "image/png", nil
}
