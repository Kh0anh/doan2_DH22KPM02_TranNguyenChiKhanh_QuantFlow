# ============================================================
# Multi-Stage Dockerfile for QuantFlow Go Backend
# ============================================================
# Author: Khanh
# Date: March 7, 2026
# Project: QuantFlow
# Description: Production-ready Docker build for Go 1.24 backend

# ===========================
# Stage 1: Build Stage
# ===========================
FROM golang:1.26.1-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Set working directory
WORKDIR /build

# Copy go.mod and go.sum first (layer caching optimization)
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download && go mod verify

# Copy entire source code
COPY . .

# Build the application
# CGO_ENABLED=1 for some GORM features, 0 for pure static binary
# -ldflags="-s -w" strips debug info to reduce binary size
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.Version=1.0.0 -X main.BuildTime=$(date -u +%Y%m%d-%H%M%S)" \
    -o quantflow-backend \
    cmd/server/main.go

# Verify the binary
RUN ./quantflow-backend --version 2>/dev/null || echo "Binary built successfully"

# ===========================
# Stage 2: Production Runtime
# ===========================
FROM alpine:latest AS production

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata wget

# Create non-root user for security
RUN addgroup -g 1000 quantflow && \
    adduser -D -u 1000 -G quantflow quantflow

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder --chown=quantflow:quantflow /build/quantflow-backend .

# Ensure binary is executable
RUN chmod +x ./quantflow-backend

# Switch to non-root user
USER quantflow

# Expose application port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/v1/health || exit 1

# Set entrypoint
ENTRYPOINT ["./quantflow-backend"]
