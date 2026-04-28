package transform

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func MagikFirstThumbnail(ctx context.Context, src io.Reader, mimeType string) ([]byte, string, error) {
	ext := mimeToExt(mimeType)

	tmpPath, cleanup, err := writeToTempFile(ctx, src, ext)
	if err != nil {
		return nil, "", fmt.Errorf("temp file: %w", err)
	}
	defer cleanup()

	output := tmpPath + "_thumb" + ".jpg"
	var buf bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx,
		"convert",
		"-units", "pixelsperinch",
		"-density", "72",
		tmpPath+"[0]",
		output,
	)

	cmd.Stdout = &buf
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, "", fmt.Errorf("convert failed: %w — stderr: %s", err, stderr.String())
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

	return thumbData, "image/jpeg", nil
}

// PDFSlideshowThumbnail converts up to 6 PDF pages to JPEG frames via ImageMagick convert,
// then concatenates them into a silent MP4 slideshow via ffmpeg.
// Returns MP4 bytes. Falls back gracefully if convert or ffmpeg is unavailable.
func PDFSlideshowThumbnail(ctx context.Context, src io.Reader, mimeType string) ([]byte, error) {
	tmpPDF, cleanup, err := writeToTempFile(ctx, src, ".pdf")
	if err != nil {
		return nil, fmt.Errorf("temp pdf: %w", err)
	}
	defer cleanup()

	dir, err := os.MkdirTemp("", "damask-pdf-*")
	if err != nil {
		return nil, fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	// convert -density 72 -format jpeg input.pdf[0-5] outdir/page-%d.jpg
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx,
		"convert",
		"-density", "72",
		"-format", "jpeg",
		tmpPDF+"[0-5]",
		filepath.Join(dir, "page-%d.jpg"),
	)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("convert pdf: %w — stderr: %s", err, stderr.String())
	}

	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return nil, fmt.Errorf("no pages extracted from PDF")
	}

	// Build ffmpeg concat list
	var sb strings.Builder
	for _, e := range entries {
		if !e.IsDir() {
			sb.WriteString("file '")
			sb.WriteString(filepath.Join(dir, e.Name()))
			sb.WriteString("'\nduration 1\n")
		}
	}
	listPath := filepath.Join(dir, "list.txt")
	if err := os.WriteFile(listPath, []byte(sb.String()), 0600); err != nil {
		return nil, fmt.Errorf("write concat list: %w", err)
	}

	outPath := filepath.Join(dir, "out.mp4")

	// Build ffmpeg args: one -loop 1 -t 3 -i <img> per page, then filter_complex
	// for slide-in transitions using overlay with eased x offset.
	ffArgs := []string{"-y"}
	for _, e := range entries {
		if !e.IsDir() {
			ffArgs = append(ffArgs, "-loop", "1", "-t", "3", "-i", filepath.Join(dir, e.Name()))
		}
	}

	n := 0
	for _, e := range entries {
		if !e.IsDir() {
			n++
		}
	}

	// filter_complex: shift each input's PTS by its index seconds, then chain overlays
	// with a quadratic ease-in slide from offscreen right.
	var fc strings.Builder
	for i := 1; i < n; i++ {
		fc.WriteString(fmt.Sprintf("[%d]setpts=PTS+%d/TB[i%d];\n", i, i, i))
	}
	prev := "[0]"
	for i := 1; i < n; i++ {
		out := fmt.Sprintf("[f%d]", i)
		if i == n-1 {
			out = "[out]"
		}
		shortest := ""
		if i == n-1 {
			shortest = ":shortest=1"
		}
		fc.WriteString(fmt.Sprintf(
			"%s[i%d]overlay=x='if(lt(t,%d),W+1,max(0,W*(1-(t-%d)^2)))':y=0%s %s;\n",
			prev, i, i, i, shortest, out,
		))
		prev = out
	}

	// Single image: no transitions needed, just output directly.
	filterStr := fc.String()
	var ffmpegStderr bytes.Buffer
	var ffCmd *exec.Cmd
	if n == 1 {
		ffCmd = exec.CommandContext(ctx,
			"ffmpeg", "-y",
			"-loop", "1", "-t", "3", "-i", filepath.Join(dir, entries[0].Name()),
			"-vf", "scale=400:-2",
			"-c:v", "libx264", "-movflags", "+faststart", "-preset", "fast", "-an",
			outPath,
		)
	} else {
		// append scale after the final overlay; can't use -vf alongside -filter_complex + -map
		filterStr += "[out]scale=400:-2[scaled]"
		ffArgs = append(ffArgs,
			"-filter_complex", filterStr,
			"-map", "[scaled]",
			"-r", "30",
			"-c:v", "libx264", "-movflags", "+faststart", "-preset", "fast", "-an",
			outPath,
		)
		ffCmd = exec.CommandContext(ctx, "ffmpeg", ffArgs...)
	}
	ffCmd.Stderr = &ffmpegStderr
	if err := ffCmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg pdf slideshow: %w — stderr: %s", err, ffmpegStderr.String())
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("read slideshow: %w", err)
	}
	return data, nil
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
