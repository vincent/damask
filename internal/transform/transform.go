// Package transform handles image/video processing pipelines.
// Implemented in Phase 4.
package transform

import (
	"context"
	"io"
	"os/exec"
	"strings"
)

type Transformer interface {
	FFmpegAvailable() bool
	ImageMagickAvailable() bool
	LibreOfficeAvailable() bool
	CheckExternalDeps() []string

	GenerateImageOfText(ctx context.Context, opts ImageOfTextOptions) ([]byte, error)
	PDFSlideshowThumbnail(ctx context.Context, src io.Reader, mimeType string) ([]byte, error)
	DocumentThumbnail(ctx context.Context, src io.Reader, mimeType string) ([]byte, error)

	ImageWatermark(src io.Reader, p WatermarkParams) ([]byte, string, error)
	ImageResize(src io.Reader, p ResizeParams) ([]byte, string, error)
	ImageConvert(src io.Reader, p ConvertParams) ([]byte, string, error)
	ImageCrop(src io.Reader, p CropParams) ([]byte, string, error)
	ImageSmartCrop(src io.Reader, p SmartCropParams) ([]byte, string, error)
	ImagePreview(src io.Reader, p PreviewParams) ([]byte, string, error)
	RemoveBackground(ctx context.Context, imageData []byte, apiKey string) ([]byte, error)

	AudioWaveform(ctx context.Context, src io.Reader, mimeType string) ([]byte, string, error)

	VideoExtractResolution(ctx context.Context, srcPath string) (*VideoResolution, error)
	VideoExtractThumbnail(ctx context.Context, srcPath string, p VideoThumbnailParams) ([]byte, error)
	VideoClipThumbnail(ctx context.Context, srcPath string, p VideoClipParams) ([]byte, error)
	VideoTranscode(ctx context.Context, srcPath, dstPath string, p TranscodeParams) error
}

func NewTransformer() Transformer {
	return &transformer{}
}

type transformer struct {
}

// FFmpegAvailable reports whether ffmpeg is in PATH.
func (t *transformer) FFmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

// ImageMagickAvailable reports whether the ImageMagick `convert` binary is in PATH.
func (t *transformer) ImageMagickAvailable() bool {
	_, err := exec.LookPath("convert")
	return err == nil
}

// LibreOfficeAvailable reports whether the `soffice` binary is in PATH.
func (t *transformer) LibreOfficeAvailable() bool {
	_, err := exec.LookPath("soffice")
	return err == nil
}

// CheckExternalDeps returns the names of required external binaries that are missing.
// ffmpeg is required for video thumbnails and PDF slideshows.
// convert (ImageMagick) is required for image and PDF thumbnails.
// soffice (LibreOffice) is required for office document thumbnails.
func (t *transformer) CheckExternalDeps() []string {
	var missing []string
	for _, bin := range []string{"ffmpeg", "convert", "soffice"} {
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
