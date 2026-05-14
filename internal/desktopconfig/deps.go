package desktopconfig

import (
	"os/exec"
	"strings"
)

// Dep describes one external binary dependency.
type Dep struct {
	Name     string   `json:"name"`
	Binary   string   `json:"binary"`
	Required bool     `json:"required"`
	DocsURL  string   `json:"docsUrl"`
	Features []string `json:"features"`
}

// AllDeps returns the full list of deps Damask cares about.
func AllDeps() []Dep {
	return []Dep{
		{
			Name:    "FFmpeg",
			Binary:  "ffmpeg",
			DocsURL: "https://ffmpeg.org/download.html",
			Features: []string{
				"Video thumbnails",
				"Audio waveforms",
				"Video/audio transcoding",
				"GIF exports",
				"Stack merges",
			},
		},
		{
			Name:    "FFprobe",
			Binary:  "ffprobe",
			DocsURL: "https://ffmpeg.org/download.html",
			Features: []string{
				"Video/audio metadata",
			},
		},
		{
			Name:    "ImageMagick",
			Binary:  "convert",
			DocsURL: "https://imagemagick.org/script/download.php",
			Features: []string{
				"PDF thumbnails",
			},
		},
		{
			Name:    "Poppler",
			Binary:  "pdftoppm",
			DocsURL: "https://poppler.freedesktop.org",
			Features: []string{
				"Animated PDF thumbnail slideshows",
			},
		},
	}
}

// DepStatus is the result of checking one dep.
type DepStatus struct {
	Dep
	Found   bool   `json:"found"`
	Version string `json:"version"`
}

// Check runs exec.LookPath for every dep and attempts a version probe.
// Never returns an error — a missing binary is represented by Found: false.
func Check() []DepStatus {
	deps := AllDeps()
	out := make([]DepStatus, len(deps))
	for i, d := range deps {
		out[i] = DepStatus{Dep: d}
		path, err := exec.LookPath(d.Binary)
		if err != nil {
			continue
		}
		out[i].Found = true
		out[i].Version = probeVersion(path, d.Binary)
	}
	return out
}

// probeVersion runs `<binary> -version 2>&1` (or `-v` for pdftoppm),
// captures the first line, and trims to 80 chars.
func probeVersion(path, binary string) string {
	flag := "-version"
	if binary == "pdftoppm" {
		flag = "-v"
	}
	cmd := exec.Command(path, flag)
	cmd.Stderr = nil
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) == 0 {
		return ""
	}
	line := strings.SplitN(string(out), "\n", 2)[0]
	if len(line) > 80 {
		line = line[:80]
	}
	return strings.TrimSpace(line)
}
