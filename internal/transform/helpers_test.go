package transform_test

import (
	"bytes"
	"context"
	"damask/server/internal/transform"
	"errors"
	"image/color"
	"io"

	"github.com/disintegration/imaging"
)

// mockStorage implements storage.Storage for testing.
type mockStorage struct {
	data map[string][]byte
	err  error
}

func (m *mockStorage) Get(key string) (io.ReadCloser, error) {
	if m.err != nil {
		return nil, m.err
	}
	if data, exists := m.data[key]; exists {
		return io.NopCloser(bytes.NewReader(data)), nil
	}
	return nil, errors.New("not found")
}

func (m *mockStorage) Put(key string, r io.Reader) error {
	return errors.New("not implemented")
}

func (m *mockStorage) Delete(key string) error {
	return errors.New("not implemented")
}

func (m *mockStorage) List(prefix string) ([]string, error) {
	return nil, errors.New("not implemented")
}

type mockTransformer struct{}

func (t *mockTransformer) FFmpegAvailable() bool       { return true }
func (t *mockTransformer) ImageMagickAvailable() bool  { return true }
func (t *mockTransformer) LibreOfficeAvailable() bool  { return true }
func (t *mockTransformer) CheckExternalDeps() []string { return []string{} }
func (t *mockTransformer) pixel(format imaging.Format) []byte {
	var buf bytes.Buffer
	img := imaging.New(1, 1, color.Black)
	_ = imaging.Encode(&buf, img, format)
	return buf.Bytes()
}
func (t *mockTransformer) GenerateImageOfText(ctx context.Context, opts transform.ImageOfTextOptions) ([]byte, error) {
	return t.pixel(imaging.PNG), nil
}
func (t *mockTransformer) PDFSlideshowThumbnail(ctx context.Context, src io.Reader, mimeType string) ([]byte, error) {
	return t.pixel(imaging.JPEG), nil
}
func (t *mockTransformer) DocumentThumbnail(ctx context.Context, src io.Reader, mimeType string) ([]byte, error) {
	return t.pixel(imaging.JPEG), nil
}
func (t *mockTransformer) ImageWatermark(src io.Reader, wm io.Reader, p transform.WatermarkParams) ([]byte, string, error) {
	return t.pixel(imaging.JPEG), "image/jpeg", nil
}
func (t *mockTransformer) ImageResize(src io.Reader, p transform.ResizeParams) ([]byte, string, error) {
	return t.pixel(imaging.JPEG), "image/jpeg", nil
}
func (t *mockTransformer) ImageConvert(src io.Reader, p transform.ConvertParams) ([]byte, string, error) {
	return t.pixel(imaging.JPEG), "image/jpeg", nil
}
func (t *mockTransformer) ImageCrop(src io.Reader, p transform.CropParams) ([]byte, string, error) {
	return t.pixel(imaging.JPEG), "image/jpeg", nil
}
func (t *mockTransformer) ImageSmartCrop(src io.Reader, p transform.SmartCropParams) ([]byte, string, error) {
	return t.pixel(imaging.JPEG), "image/jpeg", nil
}
func (t *mockTransformer) ImagePreview(src io.Reader, p transform.PreviewParams) ([]byte, string, error) {
	return t.pixel(imaging.JPEG), "image/jpeg", nil
}
func (t *mockTransformer) RemoveBackground(ctx context.Context, imageData []byte, apiKey string) ([]byte, error) {
	return t.pixel(imaging.JPEG), nil
}
func (t *mockTransformer) AudioWaveform(ctx context.Context, src io.Reader, mimeType string) ([]byte, string, error) {
	return t.pixel(imaging.JPEG), "image/jpeg", nil
}
func (t *mockTransformer) ExtractAudio(ctx context.Context, srcPath, dstPath string, p transform.AudioParams) error {
	return nil
}
func (t *mockTransformer) TranscodeAudio(ctx context.Context, srcPath, dstPath string, p transform.AudioParams) error {
	return nil
}
func (t *mockTransformer) NormalizeAudio(ctx context.Context, srcPath, dstPath string, p transform.AudioParams) error {
	return nil
}
func (t *mockTransformer) VideoExtractResolution(ctx context.Context, srcPath string) (*transform.VideoResolution, error) {
	return &transform.VideoResolution{Width: 1, Height: 1}, nil
}
func (t *mockTransformer) VideoExtractThumbnail(ctx context.Context, srcPath string, p transform.VideoThumbnailParams) ([]byte, error) {
	return t.pixel(imaging.JPEG), nil
}
func (t *mockTransformer) VideoClipThumbnail(ctx context.Context, srcPath string, p transform.VideoClipParams) ([]byte, error) {
	return t.pixel(imaging.JPEG), nil
}
func (t *mockTransformer) VideoTranscode(ctx context.Context, srcPath, dstPath string, p transform.TranscodeParams) error {
	return nil
}
func (t *mockTransformer) VideoWatermark(ctx context.Context, srcPath, dstPath string, wm io.Reader, p transform.VideoWatermarkParams) error {
	return nil
}
