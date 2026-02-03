package grpc

import (
	"context"
	"fmt"
	"time"

	videov1 "github.com/Tungwong-Project/tungwong-protos/gen/go/video"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type VideoManagementClient struct {
	client   videov1.VideoManagementServiceClient
	conn     *grpc.ClientConn
	workerID string
	logger   *logrus.Logger
}

func NewVideoManagementClient(address, workerID string, logger *logrus.Logger) (*VideoManagementClient, error) {
	// Connect to gRPC server
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to video management service: %w", err)
	}

	client := videov1.NewVideoManagementServiceClient(conn)

	logger.WithField("address", address).Info("Connected to video management gRPC service")

	return &VideoManagementClient{
		client:   client,
		conn:     conn,
		workerID: workerID,
		logger:   logger,
	}, nil
}

// MarkVideoProcessing notifies video-management that worker started processing
func (c *VideoManagementClient) MarkVideoProcessing(ctx context.Context, videoID string) error {
	c.logger.WithField("video_id", videoID).Info("Marking video as processing")

	req := &videov1.MarkVideoProcessingRequest{
		VideoId:   videoID,
		WorkerId:  c.workerID,
		StartedAt: timestamppb.Now(),
	}

	resp, err := c.client.MarkVideoProcessing(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to mark video as processing: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("failed to mark video as processing: %s", resp.Message)
	}

	c.logger.WithField("video_id", videoID).Info("Video marked as processing")
	return nil
}

// UpdateVideoStatus updates video status after successful encoding
func (c *VideoManagementClient) UpdateVideoStatus(ctx context.Context, videoID, hlsPath, thumbnailPath string, duration int) error {
	c.logger.WithFields(logrus.Fields{
		"video_id": videoID,
		"hls_path": hlsPath,
		"duration": duration,
	}).Info("Updating video status to done")

	req := &videov1.UpdateVideoStatusRequest{
		VideoId:       videoID,
		Status:        "done",
		HlsPath:       hlsPath,
		ThumbnailPath: thumbnailPath,
		Duration:      int32(duration),
		CompletedAt:   timestamppb.Now(),
	}

	resp, err := c.client.UpdateVideoStatus(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to update video status: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("failed to update video status: %s", resp.Message)
	}

	c.logger.WithField("video_id", videoID).Info("Video status updated to done")
	return nil
}

// HandleVideoFailure reports video processing failure
func (c *VideoManagementClient) HandleVideoFailure(ctx context.Context, videoID, failureReason, errorCode string, retryCount int) (bool, error) {
	c.logger.WithFields(logrus.Fields{
		"video_id":       videoID,
		"failure_reason": failureReason,
		"retry_count":    retryCount,
	}).Warn("Handling video failure")

	req := &videov1.HandleVideoFailureRequest{
		VideoId:       videoID,
		FailureReason: failureReason,
		ErrorCode:     errorCode,
		ShouldRetry:   retryCount < 3, // Max 3 retries
		RetryCount:    int32(retryCount),
		FailedAt:      timestamppb.Now(),
	}

	resp, err := c.client.HandleVideoFailure(ctx, req)
	if err != nil {
		return false, fmt.Errorf("failed to handle video failure: %w", err)
	}

	if !resp.Success {
		return false, fmt.Errorf("failed to handle video failure: %s", resp.Message)
	}

	c.logger.WithFields(logrus.Fields{
		"video_id":     videoID,
		"should_retry": resp.ShouldRetry,
	}).Info("Video failure handled")

	return resp.ShouldRetry, nil
}

// Close closes the gRPC connection
func (c *VideoManagementClient) Close() error {
	c.logger.Info("Closing video management gRPC client")
	return c.conn.Close()
}

// WithRetry wraps a gRPC call with retry logic
func (c *VideoManagementClient) WithRetry(ctx context.Context, operation func() error, maxRetries int) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		err = operation()
		if err == nil {
			return nil
		}

		if i < maxRetries-1 {
			waitTime := time.Duration(i+1) * 5 * time.Second
			c.logger.WithFields(logrus.Fields{
				"attempt":   i + 1,
				"wait_time": waitTime,
			}).Warn("Retrying gRPC call")
			time.Sleep(waitTime)
		}
	}
	return fmt.Errorf("operation failed after %d retries: %w", maxRetries, err)
}
