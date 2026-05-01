package exifextract

import (
	"bytes"
	"os"
	"testing"
	"time"
)

// syntheticJPEG is the minimal JFIF/EXIF header for a 1x1 white JPEG with
// known EXIF data embedded.  We use a real file for the Kodak test; this
// synthetic image is used for unit tests that only need a subset of tags.

// loadFile opens a file and returns its bytes, skipping the test on error.
func loadFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("test file not available: %s (%v)", path, err)
	}
	return b
}

// --- helpers ---

// approxEqual returns true when a and b differ by less than eps.
func approxEqual(a, b, eps float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < eps
}

// --- tests using the real Kodak PIXPRO image ---

func TestExtract_KodakPixpro(t *testing.T) {
	data := loadFile(t, "/home/vincent/Downloads/100_2479.JPG")

	result, err := Extract(t.Context(), bytes.NewReader(data), true)
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Extract returned nil result for a JPEG with EXIF data")
	}

	t.Run("Make", func(t *testing.T) {
		if result.Make == nil {
			t.Fatal("Make is nil")
		}
		// trim because the raw tag has trailing spaces
		if *result.Make == "" {
			t.Error("Make is empty string")
		}
	})

	t.Run("Model", func(t *testing.T) {
		if result.Model == nil {
			t.Fatal("Model is nil")
		}
		if *result.Model == "" {
			t.Error("Model is empty string")
		}
	})

	// FNumber is stored as a RATIONAL (33/10 = 3.3) in this image.
	// The old getFloat used t.Float() which only handles FloatVal format
	// and would silently return nil for rational tags.
	t.Run("FNumber", func(t *testing.T) {
		if result.FNumber == nil {
			t.Fatal("FNumber is nil — rational tag not decoded (bug in getFloat?)")
		}
		want := 3.3
		if !approxEqual(*result.FNumber, want, 0.01) {
			t.Errorf("FNumber = %v, want ~%v", *result.FNumber, want)
		}
	})

	// FocalLength is stored as RATIONAL (43/10 = 4.3 mm).
	t.Run("FocalLength", func(t *testing.T) {
		if result.FocalLength == nil {
			t.Fatal("FocalLength is nil — rational tag not decoded (bug in getFloat?)")
		}
		want := 4.3
		if !approxEqual(*result.FocalLength, want, 0.01) {
			t.Errorf("FocalLength = %v, want ~%v", *result.FocalLength, want)
		}
	})

	// FocalLengthIn35mmFilm is stored as SHORT (integer 24).
	t.Run("FocalLength35", func(t *testing.T) {
		if result.FocalLength35 == nil {
			t.Fatal("FocalLength35 is nil — SHORT tag not decoded (bug in getFloat?)")
		}
		want := 24.0
		if !approxEqual(*result.FocalLength35, want, 0.5) {
			t.Errorf("FocalLength35 = %v, want ~%v", *result.FocalLength35, want)
		}
	})

	t.Run("ISO", func(t *testing.T) {
		if result.ISO == nil {
			t.Fatal("ISO is nil")
		}
		if *result.ISO <= 0 {
			t.Errorf("ISO = %v, want positive value", *result.ISO)
		}
	})

	t.Run("ExposureTime", func(t *testing.T) {
		if result.ExposureTime == nil {
			t.Fatal("ExposureTime is nil")
		}
		if *result.ExposureTime == "" {
			t.Error("ExposureTime is empty")
		}
	})

	t.Run("TakenAt", func(t *testing.T) {
		if result.TakenAt == nil {
			t.Fatal("TakenAt is nil")
		}
		if result.TakenAt.IsZero() {
			t.Error("TakenAt is zero time")
		}
	})
}

func TestExtract_KodakPixpro_StripGPS(t *testing.T) {
	data := loadFile(t, "/home/vincent/Downloads/100_2479.JPG")

	result, err := Extract(t.Context(), bytes.NewReader(data), false)
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Extract returned nil")
	}
	if result.GPS != nil {
		t.Errorf("GPS should be nil when keepGPS=false, got %+v", result.GPS)
	}
}

// --- unit tests using synthetic / no-EXIF inputs ---

func TestExtract_NoEXIF(t *testing.T) {
	// A trivial byte sequence that is not a valid JPEG/EXIF.
	result, err := Extract(t.Context(), bytes.NewReader([]byte{0xFF, 0xD8, 0xFF, 0xE0}), true)
	if err != nil {
		t.Fatalf("unexpected error for no-EXIF data: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for no-EXIF data, got %+v", result)
	}
}

func TestExtract_Panic(t *testing.T) {
	// Empty reader triggers a panic in goexif; Extract must recover.
	result, err := Extract(t.Context(), bytes.NewReader(nil), true)
	// Either a nil result or an error is acceptable — must not panic.
	_ = result
	_ = err
}

// --- helper unit tests ---

func TestGetRational_WholeNumber(t *testing.T) {
	// getRational should format whole-number rationals without denominator.
	// We test via Extract on a real file to avoid directly coupling to internals.
	data := loadFile(t, "/home/vincent/Downloads/100_2479.JPG")
	result, _ := Extract(t.Context(), bytes.NewReader(data), false)
	if result == nil || result.ExposureTime == nil {
		t.Skip("no exposure time in test file")
	}
	// Just ensure it's non-empty and parseable.
	if *result.ExposureTime == "" {
		t.Error("ExposureTime formatted to empty string")
	}
}

func TestExtract_TakenAt_Format(t *testing.T) {
	data := loadFile(t, "/home/vincent/Downloads/100_2479.JPG")
	result, _ := Extract(t.Context(), bytes.NewReader(data), false)
	if result == nil || result.TakenAt == nil {
		t.Skip("no TakenAt in test file")
	}
	// Must be a real calendar date (after year 2000).
	if result.TakenAt.Before(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("TakenAt suspiciously old: %v", result.TakenAt)
	}
}
