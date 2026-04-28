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

// ImageMagickAvailable reports whether the ImageMagick `convert` binary is in PATH.
func ImageMagickAvailable() bool {
	_, err := exec.LookPath("convert")
	return err == nil
}

// CheckExternalDeps returns the names of required external binaries that are missing.
// ffmpeg is required for video thumbnails and PDF slideshows.
// convert (ImageMagick) is required for image and PDF thumbnails.
func CheckExternalDeps() []string {
	var missing []string
	for _, bin := range []string{"ffmpeg", "convert"} {
		if _, err := exec.LookPath(bin); err != nil {
			missing = append(missing, bin)
		}
	}
	return missing
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
