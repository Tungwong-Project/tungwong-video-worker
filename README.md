# Tungwong Video Worker

Video encoding worker service for Tungwong platform. Consumes video upload events from NATS JetStream, encodes videos to HLS format using FFmpeg, and reports status via gRPC to video-management-api.

## Features

- ✅ **NATS JetStream Consumer** - Reliable message processing with automatic retries
- ✅ **FFmpeg HLS Encoding** - Convert videos to adaptive streaming format (.m3u8)
- ✅ **Thumbnail Generation** - Automatic thumbnail extraction
- ✅ **gRPC Status Reporting** - Real-time status updates to video-management API
- ✅ **Graceful Shutdown** - Clean worker termination with message acknowledgment
- ✅ **Automatic Retry** - Failed jobs are retried with exponential backoff
- ✅ **Error Handling** - Comprehensive failure tracking and reporting

## Architecture

```
┌──────────────────┐         ┌──────────────────┐         ┌──────────────────┐
│  Video Upload    │────────>│   NATS JetStream │────────>│  Video Worker    │
│     API          │  Publish│                  │ Consume │                  │
└──────────────────┘         └──────────────────┘         └────────┬─────────┘
                                                                    │
                                                                    │ FFmpeg
                                                                    │ Encode
                                                                    ↓
                                                          ┌──────────────────┐
                                                          │   HLS Output     │
                                                          │  (.m3u8 + .ts)   │
                                                          └──────────────────┘
                                                                    │
                                                                    │ gRPC
                                                                    │ Report
                                                                    ↓
                                                          ┌──────────────────┐
                                                          │ Video Management │
                                                          │      API         │
                                                          └──────────────────┘
```

## Prerequisites

- Go 1.22+
- FFmpeg (with libx264 and AAC support)
- NATS Server with JetStream enabled
- Access to Video Management gRPC API

## Installation

### Install FFmpeg

**Ubuntu/Debian:**
```bash
sudo apt update
sudo apt install ffmpeg
```

**macOS:**
```bash
brew install ffmpeg
```

**Windows:**
Download from [ffmpeg.org](https://ffmpeg.org/download.html)

### Install Dependencies

```bash
cd tungwong-video-worker
go mod download
```

## Configuration

Copy `.env.example` to `.env` and configure:

```env
# NATS
NATS_URL=nats://localhost:4222
NATS_STREAM=VIDEO_UPLOADS
NATS_SUBJECT=video.upload.created
NATS_CONSUMER=video-worker-group
NATS_DURABLE=video-worker

# gRPC
VIDEO_MANAGEMENT_GRPC_URL=localhost:50051

# Worker
WORKER_ID=worker-1
MAX_CONCURRENT_JOBS=3

# FFmpeg
FFMPEG_HLS_TIME=10          # Segment duration (seconds)
FFMPEG_PRESET=medium        # ultrafast, fast, medium, slow
FFMPEG_CRF=23              # Quality (18-28, lower=better)

# Paths
INPUT_VIDEO_PATH=./uploads/videos
OUTPUT_HLS_PATH=./outputs/hls
OUTPUT_THUMBNAIL_PATH=./outputs/thumbnails

# Retry
MAX_RETRIES=3
RETRY_BACKOFF_SECONDS=60

# Logging
LOG_LEVEL=info
```

## Running

### Development

```bash
go run main.go
```

### Production

```bash
go build -o worker main.go
./worker
```

### Docker

```bash
docker build -t tungwong-video-worker .
docker run -d \
  --name video-worker \
  -v /path/to/uploads:/app/uploads \
  -v /path/to/outputs:/app/outputs \
  --env-file .env \
  tungwong-video-worker
```

## Message Format

NATS messages should be published in this JSON format:

```json
{
  "video_id": "550e8400-e29b-41d4-a716-446655440000",
  "file_name": "my-video.mp4",
  "upload_file_path": "/uploads/videos/550e8400_1234567890.mp4",
  "original_format": "mp4",
  "uploader_id": "user-uuid",
  "title": "My Awesome Video",
  "description": "Video description"
}
```

## Processing Flow

1. **Consume Message** - Receive video upload event from NATS JetStream
2. **Mark Processing** - Call gRPC `MarkVideoProcessing()` (heartbeat)
3. **Encode Video** - FFmpeg converts to HLS format
4. **Generate Thumbnail** - Extract thumbnail at 5 seconds
5. **Report Success** - Call gRPC `UpdateVideoStatus()` with HLS path
6. **Acknowledge** - Ack NATS message to remove from queue

### On Failure

1. **Cleanup** - Remove partial output files
2. **Report Failure** - Call gRPC `HandleVideoFailure()`
3. **Retry or Terminate** - Based on retry count

## gRPC Contracts

### MarkVideoProcessing
Heartbeat to prevent timeout-based rollback
```protobuf
message MarkVideoProcessingRequest {
  string video_id = 1;
  string worker_id = 2;
  google.protobuf.Timestamp started_at = 3;
}
```

### UpdateVideoStatus
Report successful completion
```protobuf
message UpdateVideoStatusRequest {
  string video_id = 1;
  string status = 2;              // "done"
  string hls_path = 3;
  string thumbnail_path = 4;
  int32 duration = 5;
  google.protobuf.Timestamp completed_at = 6;
}
```

### HandleVideoFailure
Report processing failure
```protobuf
message HandleVideoFailureRequest {
  string video_id = 1;
  string failure_reason = 2;
  string error_code = 3;
  bool should_retry = 4;
  int32 retry_count = 5;
  google.protobuf.Timestamp failed_at = 6;
}
```

## Error Handling

| Error | Action | Retry? |
|-------|--------|--------|
| FFmpeg encoding failed | Report failure | Yes (3x) |
| File not found | Report failure | No |
| gRPC connection error | Retry gRPC call | Yes |
| Worker crash | Video-management timeout | Auto-rollback |

## Monitoring

### Logs

Worker logs include structured fields:
- `video_id` - Video being processed
- `worker_id` - Worker instance
- `duration` - Processing time
- `error` - Error details

Example:
```
INFO[2026-01-30 10:15:23] Starting video processing video_id=550e8400-e29b-41d4-a716-446655440000
INFO[2026-01-30 10:18:45] Video processing completed successfully duration=3m22s video_id=550e8400-e29b-41d4-a716-446655440000
```

## Scaling

Run multiple workers for parallel processing:

```bash
# Worker 1
WORKER_ID=worker-1 go run main.go

# Worker 2
WORKER_ID=worker-2 go run main.go

# Worker 3
WORKER_ID=worker-3 go run main.go
```

NATS JetStream distributes messages across workers automatically.

## Troubleshooting

### FFmpeg not found
```bash
# Verify FFmpeg installation
ffmpeg -version
which ffmpeg
```

### NATS connection refused
```bash
# Check NATS server is running
nats-server -js

# Verify JetStream enabled
nats stream ls
```

### gRPC connection error
```bash
# Check video-management API is running
grpcurl -plaintext localhost:50051 list
```

## License

MIT
