# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

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
FROM alpine:3.19

# Install CA certificates for HTTPS calls
RUN apk --no-cache add ca-certificates tzdata

# Set timezone
ENV TZ=Asia/Shanghai

# Create non-root user
RUN adduser -D -g '' appuser

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
    CMD wget --no-verbose --tries=1 --spider http://localhost:50052/Health || exit 1

# Run the application
ENTRYPOINT ["./txbuilder-ethereum"]
CMD ["-config", "/app/config/config.toml"]
