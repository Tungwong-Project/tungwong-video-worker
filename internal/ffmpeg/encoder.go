package ffmpeg

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Tungwong-Project/tungwong-video-worker/configs"
	"github.com/sirupsen/logrus"
)

type Encoder struct {
	config *configs.FFmpegConfig
	paths  *configs.PathsConfig
	logger *logrus.Logger
}

type EncodeResult struct {
	HLSPath       string
	ThumbnailPath string
	Duration      int // in seconds
}

func NewEncoder(config *configs.FFmpegConfig, paths *configs.PathsConfig, logger *logrus.Logger) *Encoder {
	return &Encoder{
		config: config,
		paths:  paths,
		logger: logger,
	}
}

// EncodeToHLS converts a video file to HLS format
func (e *Encoder) EncodeToHLS(inputPath, videoID string) (*EncodeResult, error) {
	e.logger.WithFields(logrus.Fields{
		"video_id": videoID,
		"input":    inputPath,
	}).Info("Starting HLS encoding")

	// Create output directory for this video
	outputDir := filepath.Join(e.paths.OutputHLSPath, videoID)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Output paths
	hlsPath := filepath.Join(outputDir, "playlist.m3u8")
	segmentPattern := filepath.Join(outputDir, "segment_%03d.ts")

	// Build FFmpeg command for HLS encoding
	args := []string{
		"-i", inputPath,
		"-c:v", "libx264",
		"-preset", e.config.Preset,
		"-crf", strconv.Itoa(e.config.CRF),
		"-c:a", "aac",
		"-b:a", "128k",
		"-hls_time", strconv.Itoa(e.config.HLSTime),
		"-hls_playlist_type", "vod",
		"-hls_segment_filename", segmentPattern,
		"-f", "hls",
		hlsPath,
	}

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stderr = os.Stderr // Show FFmpeg output

	e.logger.WithField("command", strings.Join(cmd.Args, " ")).Debug("Executing FFmpeg")

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg encoding failed: %w", err)
	}

	// Get video duration
	duration, err := e.getVideoDuration(inputPath)
	if err != nil {
		e.logger.WithError(err).Warn("Failed to get video duration, using 0")
		duration = 0
	}

	// Generate thumbnail
	thumbnailPath, err := e.generateThumbnail(inputPath, videoID)
	if err != nil {
		e.logger.WithError(err).Warn("Failed to generate thumbnail")
		thumbnailPath = ""
	}

	e.logger.WithFields(logrus.Fields{
		"video_id": videoID,
		"hls_path": hlsPath,
		"duration": duration,
	}).Info("HLS encoding completed")

	return &EncodeResult{
		HLSPath:       hlsPath,
		ThumbnailPath: thumbnailPath,
		Duration:      duration,
	}, nil
}

// getVideoDuration extracts video duration using ffprobe
func (e *Encoder) getVideoDuration(inputPath string) (int, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		inputPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	durationStr := strings.TrimSpace(string(output))
	durationFloat, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	return int(durationFloat), nil
}

// generateThumbnail creates a thumbnail from the video
func (e *Encoder) generateThumbnail(inputPath, videoID string) (string, error) {
	// Create thumbnail directory
	outputDir := filepath.Join(e.paths.OutputThumbnailPath, videoID)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	thumbnailPath := filepath.Join(outputDir, "thumbnail.jpg")

	// Extract frame at 5 seconds (or at 10% of video duration for shorter videos)
	cmd := exec.Command("ffmpeg",
		"-i", inputPath,
		"-ss", "00:00:05",
		"-vframes", "1",
		"-vf", "scale=1280:720:force_original_aspect_ratio=decrease",
		"-q:v", "2",
		thumbnailPath,
	)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("thumbnail generation failed: %w", err)
	}

	e.logger.WithFields(logrus.Fields{
		"video_id":       videoID,
		"thumbnail_path": thumbnailPath,
	}).Info("Thumbnail generated")

	return thumbnailPath, nil
}

// Cleanup removes temporary files if encoding fails
func (e *Encoder) Cleanup(videoID string) {
	dirs := []string{
		filepath.Join(e.paths.OutputHLSPath, videoID),
		filepath.Join(e.paths.OutputThumbnailPath, videoID),
	}

	for _, dir := range dirs {
		if err := os.RemoveAll(dir); err != nil {
			e.logger.WithError(err).WithField("dir", dir).Warn("Failed to cleanup directory")
		}
	}
}
