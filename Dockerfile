FROM golang:1.24-alpine AS builder

# Install FFmpeg and build dependencies
RUN apk add --no-cache ffmpeg git

WORKDIR /app

# Set GOPRIVATE to bypass Go proxy for private repos
ENV GOPRIVATE=github.com/Tungwong-Project/*
ENV GONOSUMCHECK=github.com/Tungwong-Project/*

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .
RUN go mod tidy

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o worker main.go

# Final stage
FROM alpine:latest

# Install FFmpeg runtime
RUN apk add --no-cache ffmpeg

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/worker .

# Create necessary directories
RUN mkdir -p /app/uploads/videos /app/outputs/hls /app/outputs/thumbnails

EXPOSE 9090

CMD ["./worker"]
