package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/Tungwong-Project/tungwong-video-worker/configs"
	"github.com/Tungwong-Project/tungwong-video-worker/internal/models"
	"github.com/sirupsen/logrus"
)

type Processor interface {
	Process(ctx context.Context, msg *models.VideoUploadMessage) error
}

type Consumer struct {
	nc        *nats.Conn
	js        nats.JetStreamContext
	sub       *nats.Subscription
	config    *configs.Config
	processor Processor
	logger    *logrus.Logger
}

func NewConsumer(config *configs.Config, processor Processor, logger *logrus.Logger) (*Consumer, error) {
	// Connect to NATS
	nc, err := nats.Connect(config.NATS.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Create JetStream context
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	return &Consumer{
		nc:        nc,
		js:        js,
		config:    config,
		processor: processor,
		logger:    logger,
	}, nil
}

func (c *Consumer) Start(ctx context.Context) error {
	c.logger.Info("Starting NATS consumer...")

	// Ensure stream exists
	if err := c.ensureStream(); err != nil {
		return fmt.Errorf("failed to ensure stream: %w", err)
	}

	// Subscribe to JetStream with durable consumer
	sub, err := c.js.QueueSubscribe(
		c.config.NATS.Subject,
		c.config.NATS.Consumer,
		c.handleMessage,
		nats.Durable(c.config.NATS.Durable),
		nats.ManualAck(),
		nats.AckWait(10*time.Minute), // Give 10 minutes for processing
		nats.MaxDeliver(c.config.Retry.MaxRetries+1),
	)
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	c.sub = sub
	c.logger.WithFields(logrus.Fields{
		"subject":  c.config.NATS.Subject,
		"consumer": c.config.NATS.Consumer,
	}).Info("NATS consumer started successfully")

	// Wait for context cancellation
	<-ctx.Done()
	return c.Stop()
}

func (c *Consumer) handleMessage(msg *nats.Msg) {
	c.logger.WithField("subject", msg.Subject).Debug("Received message")

	// Parse message
	var videoMsg models.VideoUploadMessage
	if err := json.Unmarshal(msg.Data, &videoMsg); err != nil {
		c.logger.WithError(err).Error("Failed to unmarshal message")
		msg.Term() // Terminate - bad message format
		return
	}

	c.logger.WithFields(logrus.Fields{
		"video_id": videoMsg.VideoID,
		"title":    videoMsg.Title,
	}).Info("Processing video upload")

	// Process the video
	if err := c.processor.Process(context.Background(), &videoMsg); err != nil {
		c.logger.WithError(err).WithField("video_id", videoMsg.VideoID).Error("Failed to process video")

		// Check if we should retry
		meta, _ := msg.Metadata()
		if meta != nil && meta.NumDelivered >= uint64(c.config.Retry.MaxRetries+1) {
			c.logger.WithField("video_id", videoMsg.VideoID).Warn("Max retries reached, terminating message")
			msg.Term() // No more retries
		} else {
			// Nack for retry
			msg.Nak()
		}
		return
	}

	// Success - Ack the message
	if err := msg.Ack(); err != nil {
		c.logger.WithError(err).Error("Failed to ack message")
	} else {
		c.logger.WithField("video_id", videoMsg.VideoID).Info("Video processed successfully")
	}
}

func (c *Consumer) ensureStream() error {
	// Try to get stream info
	_, err := c.js.StreamInfo(c.config.NATS.Stream)
	if err == nil {
		c.logger.WithField("stream", c.config.NATS.Stream).Info("Stream already exists")
		return nil
	}

	// Create stream if it doesn't exist
	c.logger.WithField("stream", c.config.NATS.Stream).Info("Creating stream...")
	_, err = c.js.AddStream(&nats.StreamConfig{
		Name:      c.config.NATS.Stream,
		Subjects:  []string{c.config.NATS.Subject},
		Retention: nats.WorkQueuePolicy,
		MaxAge:    24 * time.Hour, // Keep messages for 24 hours
	})
	if err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}

	c.logger.Info("Stream created successfully")
	return nil
}

func (c *Consumer) Stop() error {
	c.logger.Info("Stopping NATS consumer...")

	if c.sub != nil {
		if err := c.sub.Drain(); err != nil {
			c.logger.WithError(err).Error("Failed to drain subscription")
		}
	}

	if c.nc != nil {
		c.nc.Close()
	}

	c.logger.Info("NATS consumer stopped")
	return nil
}
