package transform

import (
	"strings"
)

// IsImageMime reports whether mime is an image/* type.
func IsImageMime(mime string) bool {
	return strings.HasPrefix(mime, "image/")
}

// IsVideoMime reports whether mime is a video/* type.
func IsVideoMime(mime string) bool {
	return strings.HasPrefix(mime, "video/")
}

// IsAudioMime reports whether mime is an audio/* type.
func IsAudioMime(mime string) bool {
	return strings.HasPrefix(mime, "audio/")
}

// IsPdfMime reports whether mime is application/pdf.
func IsPdfMime(mime string) bool {
	return mime == "application/pdf"
}

// IsTextMime reports whether mime is text/*.
func IsTextMime(mime string) bool {
	return strings.HasPrefix(mime, "text/")
}

// IsFontMime reports whether mime is a font/* type.
func IsFontMime(mime string) bool {
	return strings.HasPrefix(mime, "font/")
}
