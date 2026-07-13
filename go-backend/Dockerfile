# Sub-Store Go - Multi-stage build
# Supports: linux/amd64, linux/arm64 (via Docker buildx)
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go.mod and go.sum first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application binary
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags='-w -s' -o sub-store ./cmd/sub-store/

# Final stage - minimal runtime image
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /app/sub-store /app/sub-store

# Create data directory
RUN mkdir -p /app/data

# Environment defaults
ENV SUB_STORE_BACKEND_API_HOST=0.0.0.0
ENV SUB_STORE_BACKEND_API_PORT=3000
ENV SUB_STORE_DATA_BASE_PATH=/app/data

# Expose port
EXPOSE 3000

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -q --spider http://localhost:3000/ || exit 1

# Run the application
ENTRYPOINT ["/app/sub-store"]
