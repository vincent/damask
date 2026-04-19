package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/events"
	"damask/server/internal/services"

	pdfcpuapi "github.com/pdfcpu/pdfcpu/pkg/api"
	pdftypes "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

type stackMergePayload struct {
	WorkspaceID    string   `json:"workspace_id"`
	CreatedBy      string   `json:"created_by"`
	AssetIDs       []string `json:"asset_ids"`
	OutputType     string   `json:"output_type"`
	OutputFilename string   `json:"output_filename"`
	GifFrameMs     int      `json:"gif_frame_ms"`
}

func (s *JobServer) jobStackMerge(ctx context.Context, job dbgen.Job) error {
	var p stackMergePayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "damask-stack-merge-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	type entry struct{ storageKey, ext string }
	var entries []entry
	for _, assetID := range p.AssetIDs {
		ver, err := s.db.GetCurrentVersion(ctx, assetID)
		if err != nil {
			slog.Warn("stack_merge: skip asset (no current version)", "asset_id", assetID)
			continue
		}
		ext := filepath.Ext(ver.StorageKey)
		if ext == "" {
			ext = ".bin"
		}
		entries = append(entries, entry{storageKey: ver.StorageKey, ext: ext})
	}
	if len(entries) == 0 {
		return fmt.Errorf("no processable assets in stack")
	}

	var localPaths []string
	for i, e := range entries {
		rc, err := s.storage.Get(e.storageKey)
		if err != nil {
			slog.Warn("stack_merge: skip asset (storage error)", "key", e.storageKey, "err", err)
			continue
		}
		path := filepath.Join(tmpDir, fmt.Sprintf("%04d%s", i, e.ext))
		f, err := os.Create(path)
		if err != nil {
			_ = rc.Close()
			continue
		}
		if _, err := io.Copy(f, rc); err != nil {
			_ = f.Close()
			_ = rc.Close()
			slog.Warn("stack_merge: copy error", "key", e.storageKey, "err", err)
			continue
		}
		_ = f.Close()
		_ = rc.Close()
		localPaths = append(localPaths, path)
	}
	if len(localPaths) == 0 {
		return fmt.Errorf("no assets could be downloaded")
	}

	var outPath, outExt string
	switch p.OutputType {
	case "gif":
		outPath = filepath.Join(tmpDir, "output.gif")
		outExt = ".gif"
		if err := buildGIF(localPaths, outPath, p.GifFrameMs); err != nil {
			return fmt.Errorf("build gif: %w", err)
		}
	case "pdf":
		outPath = filepath.Join(tmpDir, "output.pdf")
		outExt = ".pdf"
		if err := buildPDF(localPaths, outPath); err != nil {
			return fmt.Errorf("build pdf: %w", err)
		}
	default:
		return fmt.Errorf("unsupported output type: %s", p.OutputType)
	}

	filename := p.OutputFilename
	if filename == "" {
		filename = "stack-merge"
	}

	asset, ferr := services.CreateAsset(ctx, s.db, s.sqlDB, s.storage, s.queue, p.WorkspaceID, outPath, services.AssetOptions{
		UserID:       p.CreatedBy,
		OriginalName: filename + outExt,
	})
	if ferr != nil {
		return fmt.Errorf("create asset: %s", ferr.Message)
	}

	resultBytes, _ := json.Marshal(map[string]string{"asset_id": asset.ID})
	resultStr := string(resultBytes)
	if err := s.db.CompleteJobWithResult(ctx, dbgen.CompleteJobWithResultParams{
		Result: &resultStr,
		ID:     job.ID,
	}); err != nil {
		slog.Warn("stack_merge: could not persist result", "err", err)
	}

	s.hub.Publish(p.WorkspaceID, events.Event{
		Type:    "stack_merge_done",
		AssetID: asset.ID,
		JobID:   job.ID,
	})

	return nil
}

func buildGIF(paths []string, outPath string, frameMs int) error {
	delay := frameMs / 10
	if delay <= 0 {
		delay = 50
	}

	outGIF := &gif.GIF{}
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		img, _, err := image.Decode(f)
		_ = f.Close()
		if err != nil {
			continue
		}
		bounds := img.Bounds()
		palettedImg := image.NewPaletted(bounds, palette.Plan9)
		draw.FloydSteinberg.Draw(palettedImg, bounds, img, bounds.Min)
		outGIF.Image = append(outGIF.Image, palettedImg)
		outGIF.Delay = append(outGIF.Delay, delay)
	}
	if len(outGIF.Image) == 0 {
		return fmt.Errorf("no valid images to encode as GIF")
	}
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()
	return gif.EncodeAll(out, outGIF)
}

func buildPDF(paths []string, outPath string) error {
	imageExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".tif": true, ".tiff": true, ".webp": true}
	var imgPaths []string
	for _, p := range paths {
		if imageExts[filepath.Ext(p)] {
			imgPaths = append(imgPaths, p)
		}
	}
	if len(imgPaths) == 0 {
		return buildRawTextPDFFile(paths, outPath)
	}

	imp, err := pdfcpuapi.Import("f:A4, pos:full, sc:1.0", pdftypes.POINTS)
	if err != nil {
		return fmt.Errorf("pdfcpu import config: %w", err)
	}
	return pdfcpuapi.ImportImagesFile(imgPaths, outPath, imp, nil)
}

func buildRawTextPDFFile(paths []string, outPath string) error {
	var lines []string
	for _, p := range paths {
		lines = append(lines, filepath.Base(p))
	}
	pdf := makeRawTextPDF(lines)
	return os.WriteFile(outPath, pdf, 0o644)
}

func makeRawTextPDF(lines []string) []byte {
	var content string
	y := 750
	for _, l := range lines {
		content += fmt.Sprintf("BT /F1 12 Tf 50 %d Td (%s) Tj ET\n", y, sanitizePDFString(l))
		y -= 20
	}
	streamLen := len(content)

	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	o1 := b.Len()
	b.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	o2 := b.Len()
	b.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")
	o3 := b.Len()
	b.WriteString("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>\nendobj\n")
	o4 := b.Len()
	fmt.Fprintf(&b, "4 0 obj\n<< /Length %d >>\nstream\n%sendstream\nendobj\n", streamLen, content)
	o5 := b.Len()
	b.WriteString("5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n")
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 6\n0000000000 65535 f \n%010d 00000 n \n%010d 00000 n \n%010d 00000 n \n%010d 00000 n \n%010d 00000 n \n", o1, o2, o3, o4, o5)
	fmt.Fprintf(&b, "trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", xref)
	return b.Bytes()
}

func sanitizePDFString(s string) string {
	var out []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '(' || c == ')' || c == '\\' {
			out = append(out, '\\')
		}
		if c >= 0x20 && c < 0x7f {
			out = append(out, c)
		}
	}
	return string(out)
}
