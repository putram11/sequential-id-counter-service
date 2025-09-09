# Build stage
FROM golang:1.21-alpine AS builder

# Install dependencies
RUN apk add --no-cache git make bash

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the API service
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o bin/api cmd/api/main.go

# Build the worker service
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o bin/worker cmd/worker/main.go

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S appuser && \
    adduser -u 1001 -S appuser -G appuser

WORKDIR /app

# Copy binaries from builder stage
COPY --from=builder /app/bin/api .
COPY --from=builder /app/bin/worker .

# Copy configuration files if any
COPY --from=builder /app/config ./config

# Change ownership to non-root user
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose ports
EXPOSE 8080 9090 2112 8081

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8081/health || exit 1

# Run the application
CMD ["./sequential-id-service"]
