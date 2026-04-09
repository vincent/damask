// Package transform handles image/video processing pipelines.
// Implemented in Phase 4.
package transform

import (
	"os/exec"
	"strings"
)

// FFmpegAvailable reports whether ffmpeg is in PATH.
func FFmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

// FormatExtension maps a format name to a file extension.
func FormatExtension(format string) string {
	switch strings.ToLower(format) {
	case "webm":
		return ".webm"
	case "mp4":
		return ".mp4"
	case "png":
		return ".png"
	case "tiff":
		return ".tiff"
	default:
		return ".jpg"
	}
}
