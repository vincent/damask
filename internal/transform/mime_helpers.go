package transform

import (
	"strings"
)

// isImageMime reports whether mime is an image/* type.
func isImageMime(mime string) bool {
	return strings.HasPrefix(mime, "image/")
}

// isVideoMime reports whether mime is a video/* type.
func isVideoMime(mime string) bool {
	return strings.HasPrefix(mime, "video/")
}

// isAudioMime reports whether mime is an audio/* type.
func isAudioMime(mime string) bool {
	return strings.HasPrefix(mime, "audio/")
}

// isPdfMime reports whether mime is application/pdf.
func isPdfMime(mime string) bool {
	return mime == "application/pdf"
}

// isTextMime reports whether mime is text/*.
func isTextMime(mime string) bool {
	return strings.HasPrefix(mime, "text/")
}

// isFontMime reports whether mime is a font/* type.
func isFontMime(mime string) bool {
	return strings.HasPrefix(mime, "font/")
}
