package transform

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

func generateImageOfText(textContent string, fgColorHex string, bgColorHex string, ttfFont string, fontSize float64) ([]byte, error) {
	text := "The quick brown fo\njumps over the lazy dog."
	dpi := 92.0

	ttfFont := goregular.TTF

	fgColor := color.RGBA{0xff, 0xff, 0xff, 0xff}
	if len(fgColorHex) == 7 {
		_, err := fmt.Sscanf(fgColorHex, "#%02x%02x%02x", &fgColor.R, &fgColor.G, &fgColor.B)
		if err != nil {
			log.Println(err)
			fgColor = color.RGBA{0x2e, 0x34, 0x36, 0xff}
		}
	}

	bgColor := color.RGBA{0x30, 0x0a, 0x24, 0xff}
	if len(bgColorHex) == 7 {
		_, err := fmt.Sscanf(bgColorHex, "#%02x%02x%02x", &bgColor.R, &bgColor.G, &bgColor.B)
		if err != nil {
			log.Println(err)
			bgColor = color.RGBA{0x30, 0x0a, 0x24, 0xff}
		}
	}

	// parse the font
	f, err := opentype.Parse(ttfFont)
	if err != nil {
		log.Fatalf("font parse: %v", err)
	}
	// build the font face (collection of glyphs for specified size and DPI)
	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    fontSize,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Fatalf("create face: %v", err)
	}

	margin := int(math.Ceil(fontSize * dpi / 72.))
	metrics := face.Metrics()
	height := (int(metrics.Height)+63)/64 + 2*margin
	startingDotX := fixed.I(margin)
	startingDotY := (metrics.Ascent+63)&^63 + fixed.I(margin)
	width := (int(font.MeasureString(face, text))+63)/64 + 2*margin

	// Create an image with 16bit gray colors of specified size filled with white
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{color.RGBA{255, 255, 255, 255}}, image.Point{}, draw.Src)

	// Instantiate a font face drawer and draw text
	d := font.Drawer{
		Dst:  dst,
		Src:  image.Black,
		Face: face,
		Dot:  fixed.Point26_6{X: startingDotX, Y: startingDotY},
	}
	d.DrawString(text)

	loadedFont, err := loadFont()
	if err != nil {
		log.Println(err)
		return nil, err
	}

	code := strings.Replace(textContent, "\t", "    ", -1) // convert tabs into spaces
	text := strings.Split(code, "\n")                      // split newlines into arrays

	fg := image.NewUniform(fgColor)
	bg := image.NewUniform(bgColor)
	rgba := image.NewRGBA(image.Rect(0, 0, 1200, 630))
	draw.Draw(rgba, rgba.Bounds(), bg, image.Pt(0, 0), draw.Src)
	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetFont(loadedFont)
	c.SetFontSize(fontSize)
	c.SetClip(rgba.Bounds())
	c.SetDst(rgba)
	c.SetSrc(fg)
	c.SetHinting(font.HintingNone)

	textXOffset := 50
	textYOffset := 10 + int(c.PointToFixed(fontSize)>>6) // Note shift/truncate 6 bits first

	pt := freetype.Pt(textXOffset, textYOffset)
	for _, s := range text {
		_, err = c.DrawString(strings.Replace(s, "\r", "", -1), pt)
		if err != nil {
			return nil, err
		}
		pt.Y += c.PointToFixed(fontSize * 1.5)
	}

	b := new(bytes.Buffer)
	if err := png.Encode(b, rgba); err != nil {
		log.Println("unable to encode image.")
		return nil, err
	}
	return b.Bytes(), nil
}
