package main

import (
	"os"

	"github.com/Tungwong-Project/tungwong-video-worker/configs"
	"github.com/Tungwong-Project/tungwong-video-worker/internal/worker"
	"github.com/Tungwong-Project/tungwong-video-worker/pkg/logger"
)

func main() {
	// Load configuration
	config := configs.LoadConfig()

	// Initialize logger
	log := logger.NewLogger(config.LogLevel)

	log.Info("🎬 Tungwong Video Worker Starting...")
	log.WithFields(map[string]interface{}{
		"worker_id":            config.Worker.ID,
		"max_concurrent_jobs":  config.Worker.MaxConcurrentJobs,
		"nats_url":             config.NATS.URL,
		"video_management_url": config.GRPC.VideoManagementURL,
	}).Info("Configuration loaded")

	// Create output directories
	dirs := []string{
		config.Paths.InputVideoPath,
		config.Paths.OutputHLSPath,
		config.Paths.OutputThumbnailPath,
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.WithError(err).Fatalf("Failed to create directory: %s", dir)
		}
	}

	// Initialize and start worker
	w, err := worker.NewWorker(config, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to create worker")
	}

	// Start worker (blocks until shutdown signal)
	if err := w.Start(); err != nil {
		log.WithError(err).Error("Worker stopped with error")
		os.Exit(1)
	}

	log.Info("Worker shutdown complete")
}
