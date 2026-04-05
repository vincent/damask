package api

import (
	"fmt"
	"strings"
	"time"
)

// isImageMime reports whether mime is an image/* type.
func isImageMime(mime string) bool {
	return strings.HasPrefix(mime, "image/")
}

// parseSQLiteTime parses a timestamp string as produced by SQLite's datetime('now').
// It accepts both the SQLite default format ("2006-01-02 15:04:05") and RFC3339.
func parseSQLiteTime(s string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unrecognised time format: %q", s)
}
