package transform

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type VideoResolution struct {
	Width  int64 `json:"width"`
	Height int64 `json:"height"`
}

// VideoThumbnailParams defines parameters for extracting a video frame.
type VideoThumbnailParams struct {
	// Timestamp in seconds. If <=0, defaults to 1s.
	Timestamp float64 `json:"timestamp"`
}

// TranscodeParams defines parameters for video transcoding.
type TranscodeParams struct {
	Format     string `json:"format"`     // mp4 | webm
	Resolution string `json:"resolution"` // 1080p | 720p | 480p | "" (unchanged)
	StripAudio bool   `json:"strip_audio"`
}

// VideoExtractResolution runs ffprobe to extract a single frame from a video file.
// srcPath must be a filesystem path to the source video.
// Returns VideoResolution.
func (t *transformer) VideoExtractResolution(ctx context.Context, srcPath string) (*VideoResolution, error) {

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var buf bytes.Buffer
	var stderr bytes.Buffer

	// 	ffprobe -v error -select_streams v:0 -show_entries stream=width,height -of csv=s=x:p=0 srcPath
	cmd := exec.CommandContext(ctx,
		"ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=s=x:p=0",
		srcPath,
	)
	cmd.Stdout = &buf
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffprobe resolution: %w — stderr: %s", err, stderr.String())
	}

	maybeWxH := bytes.Trim(buf.Bytes(), "x \n\n")
	parts := bytes.Split(maybeWxH, []byte("x"))
	if len(parts) != 2 {
		return nil, fmt.Errorf("ffprobe resolution: unexpected output: %s", buf.String())
	}
	width, err := strconv.ParseInt(string(parts[0]), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("ffprobe resolution: invalid width: %w", err)
	}
	height, err := strconv.ParseInt(string(parts[1]), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("ffprobe resolution: invalid height: %w", err)
	}

	return &VideoResolution{Width: width, Height: height}, nil
}

// VideoExtractThumbnail runs ffmpeg to extract a single frame from a video file.
// srcPath must be a filesystem path to the source video.
// Returns JPEG bytes.
func (t *transformer) VideoExtractThumbnail(ctx context.Context, srcPath string, p VideoThumbnailParams) ([]byte, error) {
	if len(strings.TrimSpace(srcPath)) == 0 {
		return nil, fmt.Errorf("source path is empty")
	}

	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("source path does not exist: %s", srcPath)
	}

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

// gifFramerate returns avg_frame_rate for a GIF via ffprobe, or 0 on failure.
func gifFramerate(ctx context.Context, srcPath string) float64 {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ffprobe",
		"-v", "quiet",
		"-probesize", "20000000",
		"-select_streams", "v:0",
		"-show_entries", "stream=avg_frame_rate",
		"-of", "default=noprint_wrappers=1:nokey=1",
		srcPath,
	).Output()
	if err != nil {
		return 0
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "/", 2)
	if len(parts) != 2 {
		return 0
	}
	num, err1 := strconv.ParseFloat(parts[0], 64)
	den, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil || den == 0 {
		return 0
	}
	return num / den
}

// VideoClipParams defines parameters for creating a short video clip thumbnail.
type VideoClipParams struct {
	DurationSec int    // default 5
	Bitrate     string // default "200k"
	Width       int    // default 400
}

// VideoClipThumbnail produces a short silent MP4 clip starting at t=1s.
// Returns MP4 bytes.
func (t *transformer) VideoClipThumbnail(ctx context.Context, srcPath string, p VideoClipParams) ([]byte, error) {
	if len(strings.TrimSpace(srcPath)) == 0 {
		return nil, fmt.Errorf("source path is empty")
	}
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("source path does not exist: %s", srcPath)
	}

	dur := p.DurationSec
	if dur <= 0 {
		dur = 5
	}
	bitrate := p.Bitrate
	if bitrate == "" {
		bitrate = "200k"
	}
	width := p.Width
	if width <= 0 {
		width = 400
	}

	args := []string{"-y", "-ss", "1"}
	if strings.HasSuffix(strings.ToLower(srcPath), ".gif") {
		if fps := gifFramerate(ctx, srcPath); fps > 0 {
			args = append(args, "-r", strconv.FormatFloat(fps, 'f', -1, 64))
			dur = int(math.Round(fps))
		}
	}
	args = append(args, "-i", srcPath)

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	outPath := srcPath + "_clip.mp4"
	defer os.Remove(outPath)

	args = append(args,
		"-t", strconv.Itoa(dur),
		"-an",
		"-vf", fmt.Sprintf("scale=%d:-2", width),
		"-b:v", bitrate,
		"-c:v", "libx264",
		"-movflags", "+faststart",
		"-preset", "fast",
		outPath,
	)

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg clip: %w — stderr: %s", err, stderr.String())
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("read clip: %w", err)
	}
	return data, nil
}

// VideoTranscode transcodes a video using ffmpeg, writing the result to dstPath.
// Both srcPath and dstPath must be filesystem paths.
func (t *transformer) VideoTranscode(ctx context.Context, srcPath, dstPath string, p TranscodeParams) error {
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
	if scale := ffmpegResolutionScale(p.Resolution); scale != "" {
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

// ffmpegResolutionScale returns the ffmpeg scale filter value for a named resolution.
func ffmpegResolutionScale(res string) string {
	switch res {
	case "1080p":
		return "-2:1080"
	case "720p":
		return "-2:720"
	case "480p":
		return "-2:480"
	case "tiktok", "instagram", "youtube_shorts":
		return "1080:1920"
	case "youtube_standard":
		return "1920:1080"
	case "facebook":
		return "1080:1080"
	case "linkedin":
		return "1920:1080"
	default:
		return ""
	}
}
