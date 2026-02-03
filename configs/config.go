package configs

import (
	"os"
	"strconv"
)

type Config struct {
	NATS     NATSConfig
	GRPC     GRPCConfig
	Worker   WorkerConfig
	FFmpeg   FFmpegConfig
	Paths    PathsConfig
	Retry    RetryConfig
	LogLevel string
}

type NATSConfig struct {
	URL      string
	Stream   string
	Subject  string
	Consumer string
	Durable  string
}

type GRPCConfig struct {
	VideoManagementURL string
}

type WorkerConfig struct {
	ID                string
	MaxConcurrentJobs int
}

type FFmpegConfig struct {
	HLSTime int
	Preset  string
	CRF     int
}

type PathsConfig struct {
	InputVideoPath      string
	OutputHLSPath       string
	OutputThumbnailPath string
}

type RetryConfig struct {
	MaxRetries          int
	RetryBackoffSeconds int
}

func LoadConfig() *Config {
	return &Config{
		NATS: NATSConfig{
			URL:      getEnv("NATS_URL", "nats://localhost:4222"),
			Stream:   getEnv("NATS_STREAM", "VIDEO_UPLOADS"),
			Subject:  getEnv("NATS_SUBJECT", "video.upload.created"),
			Consumer: getEnv("NATS_CONSUMER", "video-worker-group"),
			Durable:  getEnv("NATS_DURABLE", "video-worker"),
		},
		GRPC: GRPCConfig{
			VideoManagementURL: getEnv("VIDEO_MANAGEMENT_GRPC_URL", "localhost:50051"),
		},
		Worker: WorkerConfig{
			ID:                getEnv("WORKER_ID", "worker-1"),
			MaxConcurrentJobs: getEnvAsInt("MAX_CONCURRENT_JOBS", 3),
		},
		FFmpeg: FFmpegConfig{
			HLSTime: getEnvAsInt("FFMPEG_HLS_TIME", 10),
			Preset:  getEnv("FFMPEG_PRESET", "medium"),
			CRF:     getEnvAsInt("FFMPEG_CRF", 23),
		},
		Paths: PathsConfig{
			InputVideoPath:      getEnv("INPUT_VIDEO_PATH", "./uploads/videos"),
			OutputHLSPath:       getEnv("OUTPUT_HLS_PATH", "./outputs/hls"),
			OutputThumbnailPath: getEnv("OUTPUT_THUMBNAIL_PATH", "./outputs/thumbnails"),
		},
		Retry: RetryConfig{
			MaxRetries:          getEnvAsInt("MAX_RETRIES", 3),
			RetryBackoffSeconds: getEnvAsInt("RETRY_BACKOFF_SECONDS", 60),
		},
		LogLevel: getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
