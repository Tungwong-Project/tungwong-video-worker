package worker

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/Tungwong-Project/tungwong-video-worker/configs"
	"github.com/Tungwong-Project/tungwong-video-worker/internal/ffmpeg"
	"github.com/Tungwong-Project/tungwong-video-worker/internal/grpc"
	"github.com/Tungwong-Project/tungwong-video-worker/internal/nats"
	"github.com/sirupsen/logrus"
)

type Worker struct {
	config     *configs.Config
	consumer   *nats.Consumer
	processor  *Processor
	grpcClient *grpc.VideoManagementClient
	logger     *logrus.Logger
}

func NewWorker(config *configs.Config, logger *logrus.Logger) (*Worker, error) {
	// Initialize gRPC client
	grpcClient, err := grpc.NewVideoManagementClient(
		config.GRPC.VideoManagementURL,
		config.Worker.ID,
		logger,
	)
	if err != nil {
		return nil, err
	}

	// Initialize FFmpeg encoder
	encoder := ffmpeg.NewEncoder(&config.FFmpeg, &config.Paths, logger)

	// Initialize processor
	processor := NewProcessor(encoder, grpcClient, config, logger)

	// Initialize NATS consumer
	consumer, err := nats.NewConsumer(config, processor, logger)
	if err != nil {
		grpcClient.Close()
		return nil, err
	}

	return &Worker{
		config:     config,
		consumer:   consumer,
		processor:  processor,
		grpcClient: grpcClient,
		logger:     logger,
	}, nil
}

func (w *Worker) Start() error {
	w.logger.WithField("worker_id", w.config.Worker.ID).Info("Starting video worker")

	// Create context that cancels on SIGINT/SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		w.logger.Info("Received shutdown signal, gracefully stopping...")
		cancel()
	}()

	// Start consuming messages
	if err := w.consumer.Start(ctx); err != nil {
		return err
	}

	return nil
}

func (w *Worker) Stop() error {
	w.logger.Info("Stopping worker")

	if err := w.consumer.Stop(); err != nil {
		w.logger.WithError(err).Error("Error stopping consumer")
	}

	if err := w.grpcClient.Close(); err != nil {
		w.logger.WithError(err).Error("Error closing gRPC client")
	}

	w.logger.Info("Worker stopped successfully")
	return nil
}
