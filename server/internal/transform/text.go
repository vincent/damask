package transform

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

func GenerateImageOfText(ctx context.Context, textContent string, fgColorHex string, bgColorHex string, ttfFontName string, fontSize float64) ([]byte, error) {
	dpi := 92.0
	imgWidth := 400
	imgHeight := 400
	ttfFont := goregular.TTF

	fgColor := color.RGBA{0, 0, 0, 255}
	fmt.Sscanf(fgColorHex, "#%02x%02x%02x", &fgColor.R, &fgColor.G, &fgColor.B)

	bgColor := color.RGBA{255, 255, 255, 255}
	fmt.Sscanf(bgColorHex, "#%02x%02x%02x", &bgColor.R, &bgColor.G, &bgColor.B)

	lines := strings.Split(textContent, "\n")
	maxLen := 0
	for _, line := range lines {
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}
	if maxLen == 0 {
		maxLen = 1
	}

	if fontSize <= 0 {
		fontSize = max(8, min(32, float64(imgWidth)/float64(maxLen)*1.5))
	}

	f, err := opentype.Parse(ttfFont)
	if err != nil {
		return nil, err
	}

	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    fontSize,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, err
	}

	metrics := face.Metrics()
	lineHeight := (int(metrics.Height) + 63) / 64
	ascent := (metrics.Ascent + 63) &^ 63

	margin := 10
	maxWidth := imgWidth - 2*margin
	maxHeight := imgHeight - 2*margin

	dst := image.NewRGBA(image.Rect(0, 0, imgWidth, imgHeight))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

	d := &font.Drawer{
		Dst:  dst,
		Src:  &image.Uniform{fgColor},
		Face: face,
	}

	var wrappedLines []string
	for _, line := range strings.Split(textContent, "\n") {
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
