package exifextract

import (
	"context"
	"damask/server/internal/telemetry"
	"fmt"
	"io"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/mknote"
)

func init() {
	exif.RegisterParsers(mknote.All...)
}

// ExtractedEXIF holds parsed EXIF data. All fields are pointers — nil means the tag
// was absent or unparseable, never a zero value.
type ExtractedEXIF struct {
	Make          *string
	Model         *string
	LensModel     *string
	Software      *string
	ExposureTime  *string // rational string, e.g. "1/250"
	FNumber       *float64
	ISO           *int64
	FocalLength   *float64
	FocalLength35 *float64
	Flash         *string
	WhiteBalance  *string
	TakenAt       *time.Time
	GPS           *GPSCoords // nil if absent or keepGPS=false
}

// GPSCoords holds GPS location data.
type GPSCoords struct {
	Lat      float64
	Lng      float64
	Altitude float64 // metres, 0 if tag absent
}

// Extract reads EXIF from r. keepGPS=false strips GPS regardless of file content.
// Returns nil, nil when the file has no EXIF data — callers should write a tombstone.
// Never panics: all goexif calls are wrapped in recover().
func Extract(ctx context.Context, r io.Reader, keepGPS bool) (result *ExtractedEXIF, err error) {
	_, span := telemetry.StartSpan(ctx, "services.exif.extract")
	defer telemetry.EndSpan(span, err)

	defer func() {
		if rec := recover(); rec != nil {
			result = nil
			err = fmt.Errorf("exif: panic during decode: %v", rec)
		}
	}()

	x, decErr := exif.Decode(r)
	if decErr != nil {
		if exif.IsCriticalError(decErr) {
			return nil, nil // no EXIF — not a failure
		}
		return nil, decErr
	}

	out := &ExtractedEXIF{}

	// Camera
	out.Make = getString(x, exif.Make)
	out.Model = getString(x, exif.Model)
	out.LensModel = getString(x, exif.LensModel)
	out.Software = getString(x, exif.Software)

	// Capture settings
	out.ExposureTime = getRational(x, exif.ExposureTime)
	out.FNumber = getFloat(x, exif.FNumber)
	out.ISO = getInt(x, exif.ISOSpeedRatings)
	out.FocalLength = getFloat(x, exif.FocalLength)
	out.FocalLength35 = getFloat(x, exif.FocalLengthIn35mmFilm)
	out.Flash = getFlashString(x)
	out.WhiteBalance = getWhiteBalanceString(x)

	// Date taken — prefer DateTimeOriginal
	if t, ok := parseDateTime(x); ok {
		out.TakenAt = &t
	}

	// GPS
	if keepGPS {
		if lat, lng, gpsErr := x.LatLong(); gpsErr == nil {
			coords := &GPSCoords{Lat: lat, Lng: lng}
			coords.Altitude = getGPSAltitude(x)
			out.GPS = coords
		}
	}

	return out, nil
}
