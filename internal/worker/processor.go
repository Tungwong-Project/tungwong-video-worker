package worker

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/Tungwong-Project/tungwong-video-worker/configs"
	"github.com/Tungwong-Project/tungwong-video-worker/internal/ffmpeg"
	"github.com/Tungwong-Project/tungwong-video-worker/internal/grpc"
	"github.com/Tungwong-Project/tungwong-video-worker/internal/models"
	"github.com/sirupsen/logrus"
)

type Processor struct {
	encoder      *ffmpeg.Encoder
	grpcClient   *grpc.VideoManagementClient
	config       *configs.Config
	logger       *logrus.Logger
	retryTracker map[string]int // Track retry counts per video
}

func NewProcessor(
	encoder *ffmpeg.Encoder,
	grpcClient *grpc.VideoManagementClient,
	config *configs.Config,
	logger *logrus.Logger,
) *Processor {
	return &Processor{
		encoder:      encoder,
		grpcClient:   grpcClient,
		config:       config,
		logger:       logger,
		retryTracker: make(map[string]int),
	}
}

// Process handles the complete video processing workflow
func (p *Processor) Process(ctx context.Context, msg *models.VideoUploadMessage) error {
	videoID := msg.VideoID

	p.logger.WithFields(logrus.Fields{
		"video_id": videoID,
		"title":    msg.Title,
		"file":     msg.FileName,
	}).Info("Starting video processing")

	// Step 1: Mark video as processing (heartbeat to prevent timeout)
	if err := p.grpcClient.MarkVideoProcessing(ctx, videoID); err != nil {
		p.logger.WithError(err).Error("Failed to mark video as processing")
		// Continue anyway - this is just a heartbeat
	}

	// Step 2: Build input file path
	inputPath := filepath.Join(p.config.Paths.InputVideoPath, filepath.Base(msg.UploadFilePath))

	// Step 3: Encode video to HLS
	result, err := p.encoder.EncodeToHLS(inputPath, videoID)
	if err != nil {
		return p.handleFailure(ctx, videoID, err, "ENCODING_FAILED")
	}

	// Step 4: Update video status to done
	err = p.grpcClient.UpdateVideoStatus(
		ctx,
		videoID,
		result.HLSPath,
		result.ThumbnailPath,
		result.Duration,
	)
	if err != nil {
		p.logger.WithError(err).Error("Failed to update video status, but encoding succeeded")
		// Retry the gRPC call
		err = p.grpcClient.WithRetry(ctx, func() error {
			return p.grpcClient.UpdateVideoStatus(ctx, videoID, result.HLSPath, result.ThumbnailPath, result.Duration)
		}, 3)
		if err != nil {
			return fmt.Errorf("failed to update video status after retries: %w", err)
		}
	}

	p.logger.WithField("video_id", videoID).Info("Video processing completed successfully")
	return nil
}

// handleFailure reports failure to video-management API
func (p *Processor) handleFailure(ctx context.Context, videoID string, err error, errorCode string) error {
	p.logger.WithError(err).WithField("video_id", videoID).Error("Video processing failed")

	// Track retry count
	retryCount := p.retryTracker[videoID]
	p.retryTracker[videoID] = retryCount + 1

	// Cleanup partial files
	p.encoder.Cleanup(videoID)

	// Report failure to video-management
	shouldRetry, grpcErr := p.grpcClient.HandleVideoFailure(
		ctx,
		videoID,
		err.Error(),
		errorCode,
		retryCount,
	)
	if grpcErr != nil {
		p.logger.WithError(grpcErr).Error("Failed to report video failure to management API")
		return fmt.Errorf("processing failed and couldn't report: %w", err)
	}

	if !shouldRetry {
		p.logger.WithField("video_id", videoID).Info("Max retries reached, video marked as permanently failed")
		delete(p.retryTracker, videoID) // Clean up tracker
	}

	return err
}
