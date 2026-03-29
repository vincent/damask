// Package transform handles image and video processing pipelines.
package transform

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"image"
	"io"
	"strings"

	"github.com/disintegration/imaging"
)

//go:embed watermark.png
var watermarkBytes []byte

// WatermarkParams defines parameters for image resize/fit transforms.
type WatermarkParams struct {
	Opacity int    `json:"opacity"` // 0–100, default 50
	Format  string `json:"format"`  // jpeg | png | tiff
	Quality int    `json:"quality"` // 1–100 (for jpeg)
}

// ResizeParams defines parameters for image resize/fit transforms.
type ResizeParams struct {
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Fit     string `json:"fit"`     // cover | contain | fill
	Quality int    `json:"quality"` // 1–100, default 85
	Format  string `json:"format"`  // jpeg | png | tiff (webp unsupported without CGO)
}

// ConvertParams defines parameters for image format conversion.
type ConvertParams struct {
	Format  string `json:"format"`  // jpeg | png | tiff
	Quality int    `json:"quality"` // 1–100 (for jpeg)
}

// CropParams defines parameters for an image crop operation.
type CropParams struct {
	X       int    `json:"x"`
	Y       int    `json:"y"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Quality int    `json:"quality"`
	Format  string `json:"format"`
}

// PreviewParams defines parameters for the low-res in-memory preview.
type PreviewParams struct {
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Fit     string `json:"fit"`
	Quality int    `json:"quality"`
	Format  string `json:"format"`
}

// Watermark reads an image, resizes it according to params, and returns encoded bytes.
func Watermark(src io.Reader, p WatermarkParams) ([]byte, string, error) {
	if p.Opacity <= 0 || p.Opacity > 100 {
		p.Opacity = 50
	}

	watermarkImg, err := imaging.Decode(bytes.NewReader(watermarkBytes), imaging.AutoOrientation(true))
	if err != nil {
		return nil, "", fmt.Errorf("decode watermark: %w", err)
	}

	img, err := imaging.Decode(src, imaging.AutoOrientation(true))
	if err != nil {
		return nil, "", fmt.Errorf("decode image: %w", err)
	}

	result := overlayWatermark(img, watermarkImg, float64(p.Opacity)/100.0)

	return encodeImage(result, p.Format, p.Quality)
}

func overlayWatermark(originalImg, watermarkImg image.Image, opacity float64) image.Image {

	halfImageWidth := int(float64(originalImg.Bounds().Dx()) * 0.50)
	halfImageHeight := int(float64(originalImg.Bounds().Dy()) * 0.50)

	watermarkResized := imaging.Fit(watermarkImg, halfImageWidth, halfImageHeight, imaging.Lanczos)

	offsetX := int(halfImageWidth / 2)
	offsetY := int(halfImageHeight / 2)

	return imaging.Overlay(originalImg, watermarkResized, image.Pt(offsetX, offsetY), opacity)
}

// Resize reads an image, resizes it according to params, and returns encoded bytes.
func Resize(src io.Reader, p ResizeParams) ([]byte, string, error) {
	if p.Width <= 0 && p.Height <= 0 {
		return nil, "", errors.New("width or height must be > 0")
	}
	if p.Quality <= 0 || p.Quality > 100 {
		p.Quality = 85
	}

	img, err := imaging.Decode(src, imaging.AutoOrientation(true))
	if err != nil {
		return nil, "", fmt.Errorf("decode image: %w", err)
	}

	var result image.Image
	switch strings.ToLower(p.Fit) {
	case "fill":
		w, h := ensureDimensions(img, p.Width, p.Height)
		result = imaging.Fill(img, w, h, imaging.Center, imaging.Lanczos)
	case "cover":
		w, h := ensureDimensions(img, p.Width, p.Height)
		result = imaging.Fill(img, w, h, imaging.Center, imaging.Lanczos)
	default: // contain
		result = imaging.Fit(img, p.Width, p.Height, imaging.Lanczos)
	}

	return encodeImage(result, p.Format, p.Quality)
}

// Convert reads an image and re-encodes it in the target format.
func Convert(src io.Reader, p ConvertParams) ([]byte, string, error) {
	if p.Quality <= 0 || p.Quality > 100 {
		p.Quality = 85
	}
	img, err := imaging.Decode(src, imaging.AutoOrientation(true))
	if err != nil {
		return nil, "", fmt.Errorf("decode image: %w", err)
	}
	return encodeImage(img, p.Format, p.Quality)
}

// Crop reads an image, crops the specified rectangle, and returns encoded bytes.
func Crop(src io.Reader, p CropParams) ([]byte, string, error) {
	if p.Width <= 0 || p.Height <= 0 {
		return nil, "", errors.New("width and height must be > 0")
	}
	if p.Quality <= 0 || p.Quality > 100 {
		p.Quality = 85
	}
	img, err := imaging.Decode(src, imaging.AutoOrientation(true))
	if err != nil {
		return nil, "", fmt.Errorf("decode image: %w", err)
	}
	cropped := imaging.Crop(img, image.Rect(p.X, p.Y, p.X+p.Width, p.Y+p.Height))
	return encodeImage(cropped, p.Format, p.Quality)
}

// Preview generates a small in-memory preview (max 800px) for the UI.
func Preview(src io.Reader, p PreviewParams) ([]byte, string, error) {
	if p.Quality <= 0 || p.Quality > 100 {
		p.Quality = 80
	}
	// Cap preview at 800px.
	if p.Width > 800 {
		p.Width = 800
	}
	if p.Height > 800 {
		p.Height = 800
	}
	if p.Width <= 0 && p.Height <= 0 {
		p.Width = 800
	}

	img, err := imaging.Decode(src, imaging.AutoOrientation(true))
	if err != nil {
		return nil, "", fmt.Errorf("decode image: %w", err)
	}

	var result image.Image
	switch strings.ToLower(p.Fit) {
	case "fill", "cover":
		w, h := ensureDimensions(img, p.Width, p.Height)
		result = imaging.Fill(img, w, h, imaging.Center, imaging.Lanczos)
	default:
		result = imaging.Fit(img, p.Width, p.Height, imaging.Lanczos)
	}

	if p.Format == "" {
		p.Format = "jpeg"
	}
	return encodeImage(result, p.Format, p.Quality)
}

// encodeImage encodes an image to the given format and returns the bytes + content-type.
func encodeImage(img image.Image, format string, quality int) ([]byte, string, error) {
	var buf bytes.Buffer
	var contentType string
	switch strings.ToLower(format) {
	case "png":
		if err := imaging.Encode(&buf, img, imaging.PNG); err != nil {
			return nil, "", fmt.Errorf("encode png: %w", err)
		}
		contentType = "image/png"
	case "tiff":
		if err := imaging.Encode(&buf, img, imaging.TIFF); err != nil {
			return nil, "", fmt.Errorf("encode tiff: %w", err)
		}
		contentType = "image/tiff"
	default: // jpeg
		if err := imaging.Encode(&buf, img, imaging.JPEG, imaging.JPEGQuality(quality)); err != nil {
			return nil, "", fmt.Errorf("encode jpeg: %w", err)
		}
		contentType = "image/jpeg"
	}
	return buf.Bytes(), contentType, nil
}

// ensureDimensions returns both dimensions; if one is 0, it uses the other.
func ensureDimensions(img image.Image, w, h int) (int, int) {
	if w <= 0 {
		w = img.Bounds().Dx()
	}
	if h <= 0 {
		h = img.Bounds().Dy()
	}
	return w, h
}

// FormatExtension maps a format name to a file extension.
func FormatExtension(format string) string {
	switch strings.ToLower(format) {
	case "png":
		return ".png"
	case "tiff":
		return ".tiff"
	default:
		return ".jpg"
	}
}
