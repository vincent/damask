package transform

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type ImageOfTextOptions struct {
	TextContent string
	FgColorHex  string
	BgColorHex  string
	FontSize    float64
	Dpi         float64
	Width       int
	Height      int
	FontFile    io.ReadCloser // optional, if empty use default font
}

func (t *transformer) GenerateImageOfText(ctx context.Context, opts ImageOfTextOptions) ([]byte, error) {
	var err error
	var openTypeFont *opentype.Font
	if opts.FontFile == nil {
		openTypeFont, err = opentype.Parse(goregular.TTF)
		if err != nil {
			return nil, err
		}
	} else {
		fontData, err := io.ReadAll(opts.FontFile)
		if err != nil {
			return nil, fmt.Errorf("read font file: %w", err)
		}
		openTypeFont, err = opentype.Parse(fontData)
		if err != nil {
			return nil, fmt.Errorf("parse font file: %w", err)
		}
	}

	if opts.TextContent == "" {
		opts.TextContent = " " // avoid empty content which can cause issues with some font renderers
	}
	if opts.FgColorHex == "" {
		opts.FgColorHex = "#000000" // default to black
	}
	if opts.BgColorHex == "" {
		opts.BgColorHex = "#FFFFFF" // default to white
	}
	if opts.Dpi <= 0 {
		opts.Dpi = 72 // default DPI
	}
	if opts.Width <= 0 {
		opts.Width = 400 // default width
	}
	if opts.Height <= 0 {
		opts.Height = 400 // default height
	}

	fgColor := color.RGBA{0, 0, 0, 255}
	_, _ = fmt.Sscanf(opts.FgColorHex, "#%02x%02x%02x", &fgColor.R, &fgColor.G, &fgColor.B)

	bgColor := color.RGBA{255, 255, 255, 255}
	_, _ = fmt.Sscanf(opts.BgColorHex, "#%02x%02x%02x", &bgColor.R, &bgColor.G, &bgColor.B)

	lines := strings.Split(opts.TextContent, "\n")
	maxLen := 0
	for _, line := range lines {
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}
	if maxLen == 0 {
		maxLen = 1
	}

	if opts.FontSize <= 0 {
		//nolint:mnd // heuristic font size based on width and text length
		opts.FontSize = max(8, min(32, float64(opts.Width)/float64(maxLen)*1.5))
	}

	face, err := opentype.NewFace(openTypeFont, &opentype.FaceOptions{
		Size:    opts.FontSize,
		DPI:     opts.Dpi,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, err
	}

	metrics := face.Metrics()
	lineHeight := (int(metrics.Height) + 63) / 64 //nolint:mnd // round to nearest integer
	ascent := (metrics.Ascent + 63) &^ 63         //nolint:mnd // round to nearest integer

	margin := 10
	maxWidth := opts.Width - 2*margin
	maxHeight := opts.Height - 2*margin

	dst := image.NewRGBA(image.Rect(0, 0, opts.Width, opts.Height))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

	d := &font.Drawer{
		Dst:  dst,
		Src:  &image.Uniform{fgColor},
		Face: face,
	}

	var wrappedLines []string
	for line := range strings.SplitSeq(opts.TextContent, "\n") {
		words := strings.Fields(line)
		if len(words) == 0 {
			wrappedLines = append(wrappedLines, "")
			continue
		}

		current := words[0]
		for _, word := range words[1:] {
			test := current + " " + word
			if d.MeasureString(test).Round() <= maxWidth {
				current = test
			} else {
				wrappedLines = append(wrappedLines, current)
				current = word
			}
		}
		wrappedLines = append(wrappedLines, current)
	}

	// draw with clipping (discard overflow)
	y := fixed.I(margin) + ascent

	for _, line := range wrappedLines {
		if (y.Round() + lineHeight) > (margin + maxHeight) {
			break // stop drawing if overflow
		}

		d.Dot = fixed.Point26_6{
			X: fixed.I(margin),
			Y: y,
		}
		d.DrawString(line)
		y += fixed.I(lineHeight)
	}

	buf := new(bytes.Buffer)
	if err := png.Encode(buf, dst); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
