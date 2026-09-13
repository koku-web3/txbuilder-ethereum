# Build stage
FROM golang:1.26-bookworm AS builder

# Set proxy for build
ARG HTTP_PROXY
ARG HTTPS_PROXY
ENV http_proxy=$HTTP_PROXY
ENV https_proxy=$HTTPS_PROXY

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o txbuilder-ethereum ./cmd/txbuilder-ethereum/

# Runtime stage
FROM debian:bookworm-slim

# Install CA certificates for HTTPS calls
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    netcat \
    && rm -rf /var/lib/apt/lists/*

# Set timezone
ENV TZ=Asia/Shanghai

# Create non-root user
RUN groupadd -r appuser && useradd -r -g appuser appuser

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/txbuilder-ethereum .

# Copy default config (will be overridden by volume mount)
COPY --from=builder /app/config ./config

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose gRPC port
EXPOSE 50052

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD nc -z 127.0.0.1 50052 || exit 1

# Run the application
ENTRYPOINT ["./txbuilder-ethereum"]
CMD ["-config", "/app/config/config.toml"]
