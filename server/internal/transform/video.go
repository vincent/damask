package transform

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"time"
)

// VideoThumbnailParams defines parameters for extracting a video frame.
type VideoThumbnailParams struct {
	// Timestamp in seconds. If <=0, defaults to 1s.
	Timestamp float64 `json:"timestamp"`
}

// TranscodeParams defines parameters for video transcoding.
type TranscodeParams struct {
	Format     string `json:"format"`      // mp4 | webm
	Resolution string `json:"resolution"`  // 1080p | 720p | 480p | "" (unchanged)
	StripAudio bool   `json:"strip_audio"`
}

// ExtractVideoThumbnail runs ffmpeg to extract a single frame from a video file.
// srcPath must be a filesystem path to the source video.
// Returns JPEG bytes.
func ExtractVideoThumbnail(ctx context.Context, srcPath string, p VideoThumbnailParams) ([]byte, error) {
	ts := p.Timestamp
	if ts <= 0 {
		ts = 1.0
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var buf bytes.Buffer
	var stderr bytes.Buffer

	// ffmpeg -ss {ts} -i input -frames:v 1 -vcodec mjpeg -q:v 3 -f image2pipe pipe:1
	cmd := exec.CommandContext(ctx,
		"ffmpeg",
		"-ss", strconv.FormatFloat(ts, 'f', 3, 64),
		"-i", srcPath,
		"-frames:v", "1",
		"-vcodec", "mjpeg",
		"-q:v", "3",
		"-f", "image2pipe",
		"pipe:1",
	)
	cmd.Stdout = &buf
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg thumbnail: %w — stderr: %s", err, stderr.String())
	}
	return buf.Bytes(), nil
}

// TranscodeVideo transcodes a video using ffmpeg, writing the result to dstPath.
// Both srcPath and dstPath must be filesystem paths.
func TranscodeVideo(ctx context.Context, srcPath, dstPath string, p TranscodeParams) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	args := []string{"-y", "-i", srcPath}

	// Video codec
	switch p.Format {
	case "webm":
		args = append(args, "-c:v", "libvpx-vp9")
	default: // mp4
		args = append(args, "-c:v", "libx264", "-movflags", "+faststart", "-preset", "fast")
	}

	// Audio
	if p.StripAudio {
		args = append(args, "-an")
	} else {
		switch p.Format {
		case "webm":
			args = append(args, "-c:a", "libopus")
		default:
			args = append(args, "-c:a", "aac")
		}
	}

	// Resolution scaling
	if scale := resolutionScale(p.Resolution); scale != "" {
		args = append(args, "-vf", "scale="+scale)
	}

	args = append(args, dstPath)

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg transcode: %w — stderr: %s", err, stderr.String())
	}
	return nil
}

// resolutionScale returns the ffmpeg scale filter value for a named resolution.
func resolutionScale(res string) string {
	switch res {
	case "1080p":
		return "-2:1080"
	case "720p":
		return "-2:720"
	case "480p":
		return "-2:480"
	default:
		return ""
	}
}

// FFmpegAvailable reports whether ffmpeg is in PATH.
func FFmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

// TranscodeExtension returns the output file extension for the given format.
func TranscodeExtension(format string) string {
	switch format {
	case "webm":
		return ".webm"
	default:
		return ".mp4"
	}
}
