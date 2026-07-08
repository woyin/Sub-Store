# Sub-Store Go - Multi-stage build
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags='-w -s' -o sub-store .

# Final stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS requests
RUN apk add --no-cache ca-certificates

# Copy binary from builder
COPY --from=builder /app/sub-store /app/sub-store

# Create data directory
RUN mkdir -p /app/data

# Environment variables
ENV SUB_STORE_BACKEND_API_HOST=0.0.0.0
ENV SUB_STORE_BACKEND_API_PORT=3000
ENV SUB_STORE_DATA_BASE_PATH=/app/data

# Expose port
EXPOSE 3000

# Run the application
ENTRYPOINT ["/app/sub-store"]
