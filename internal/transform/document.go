package transform

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func findExt(dir, ext string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ext {
			return filepath.Join(dir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("soffice: no %s output produced", ext)
}

// DocumentThumbnail converts the first page of an office document to PNG using LibreOffice.
func (t *transformer) DocumentThumbnail(ctx context.Context, src io.Reader, mimeType string) ([]byte, error) {
	ext := mimeToExt(mimeType)
	tmpPath, cleanup, err := writeToTempFile(ctx, src, ext)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	outDir, err := os.MkdirTemp("", "damask-doc-thumb-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(outDir)

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	srcPath := tmpPath

	// HTML must be converted to PDF first; soffice cannot render it directly to PNG.
	if mimeType == "text/html" {
		out, err := exec.CommandContext(ctx, "soffice", "--headless", "--convert-to", "pdf", "--outdir", outDir, tmpPath).CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("soffice html→pdf: %w: %s", err, strings.TrimSpace(string(out)))
		}
		pdfPath, err := findExt(outDir, ".pdf")
		if err != nil {
			return nil, fmt.Errorf("soffice html→pdf: %w", err)
		}
		srcPath = pdfPath
	}

	out, err := exec.CommandContext(ctx, "soffice", "--headless", "--convert-to", "png", "--outdir", outDir, srcPath).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("soffice: %w: %s", err, strings.TrimSpace(string(out)))
	}

	pngPath, err := findExt(outDir, ".png")
	if err != nil {
		return nil, err
	}
	return os.ReadFile(pngPath)
}
