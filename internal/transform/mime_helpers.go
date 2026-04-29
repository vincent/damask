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

// IsDocumentMime reports whether mime is an office document type handled by LibreOffice.
func IsDocumentMime(mime string) bool {
	switch mime {
	case
		"application/vnd.oasis.opendocument.presentation",
		"application/vnd.ms-powerpoint",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"application/vnd.oasis.opendocument.text",
		"application/msword",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/rtf",
		"text/html",
		"application/vnd.oasis.opendocument.spreadsheet",
		"application/vnd.ms-excel",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"text/csv":
		return true
	}
	return false
}
