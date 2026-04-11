package exifextract

import (
	"fmt"
	"strings"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

func getString(x *exif.Exif, tag exif.FieldName) *string {
	t, err := x.Get(tag)
	if err != nil {
		return nil
	}
	s, err := t.StringVal()
	if err != nil {
		return nil
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func getRational(x *exif.Exif, tag exif.FieldName) *string {
	t, err := x.Get(tag)
	if err != nil {
		return nil
	}
	num, den, err2 := t.Rat2(0)
	if err2 != nil {
		return nil
	}
	var s string
	if den == 1 {
		s = fmt.Sprintf("%d", num)
	} else {
		s = fmt.Sprintf("%d/%d", num, den)
	}
	return &s
}

func getGPSAltitude(x *exif.Exif) float64 {
	t, err := x.Get(exif.GPSAltitude)
	if err != nil {
		return 0
	}
	f, err := t.Float(0)
	if err != nil {
		return 0
	}
	return f
}

func getFloat(x *exif.Exif, tag exif.FieldName) *float64 {
	t, err := x.Get(tag)
	if err != nil {
		return nil
	}
	f, err := t.Float(0)
	if err != nil {
		return nil
	}
	return &f
}

func getInt(x *exif.Exif, tag exif.FieldName) *int64 {
	t, err := x.Get(tag)
	if err != nil {
		return nil
	}
	v, err := t.Int(0)
	if err != nil {
		return nil
	}
	i := int64(v)
	return &i
}

// parseDateTime tries DateTimeOriginal then DateTime.
func parseDateTime(x *exif.Exif) (time.Time, bool) {
	for _, tag := range []exif.FieldName{exif.DateTimeOriginal, exif.DateTime} {
		t, err := x.Get(tag)
		if err != nil {
			continue
		}
		s, err := t.StringVal()
		if err != nil {
			continue
		}
		// EXIF datetime format: "2006:01:02 15:04:05"
		parsed, err := time.Parse("2006:01:02 15:04:05", strings.TrimSpace(s))
		if err != nil {
			continue
		}
		return parsed, true
	}
	return time.Time{}, false
}

func getFlashString(x *exif.Exif) *string {
	t, err := x.Get(exif.Flash)
	if err != nil {
		return nil
	}
	v, err := t.Int(0)
	if err != nil {
		return nil
	}
	// Bit 0: flash fired
	var s string
	if v&0x1 != 0 {
		s = "Fired"
	} else {
		s = "No flash"
	}
	return &s
}

func getWhiteBalanceString(x *exif.Exif) *string {
	t, err := x.Get(exif.WhiteBalance)
	if err != nil {
		return nil
	}
	v, err := t.Int(0)
	if err != nil {
		return nil
	}
	var s string
	switch v {
	case 0:
		s = "Auto"
	case 1:
		s = "Manual"
	default:
		s = fmt.Sprintf("%d", v)
	}
	return &s
}

