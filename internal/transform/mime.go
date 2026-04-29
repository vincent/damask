package transform

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// DetectMimeType sniffs the MIME type of the file at filePath.
// When content sniffing returns a generic type (zip, octet-stream, plain text),
// it falls back to extension-based lookup to correctly identify OOXML/ODF formats
// that are zip-based and would otherwise be misidentified.
func DetectMimeType(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	sniff := make([]byte, 512)
	n, _ := f.Read(sniff)
	mimeType := stripMimeParams(http.DetectContentType(sniff[:n]))

	if isGenericMime(mimeType) {
		if ext := filepath.Ext(filePath); ext != "" {
			if byExt := stripMimeParams(mime.TypeByExtension(ext)); byExt != "" {
				return byExt, nil
			}
		}
	}

	return mimeType, nil
}

func stripMimeParams(ct string) string {
	if idx := strings.Index(ct, ";"); idx != -1 {
		return strings.TrimSpace(ct[:idx])
	}
	return ct
}

func isGenericMime(ct string) bool {
	switch ct {
	case "application/zip", "application/octet-stream", "text/plain":
		return true
	}
	return false
}

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
