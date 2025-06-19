# Build stage
FROM golang:1.23-alpine AS builder

# Install dependencies
RUN apk add --no-cache git make protoc protobuf-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Generate proto files
RUN make proto

# Generate swagger docs
RUN make swagger

# Build applications
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o bin/api ./api/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o bin/grpc ./grpc/main.go

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy binaries from builder
COPY --from=builder /app/bin/api /app/api
COPY --from=builder /app/bin/grpc /app/grpc

# Copy configuration files
COPY --from=builder /app/.env.example /app/.env.example

# Create required directories
RUN mkdir -p /app/uploads /app/videos /app/logs && \
    chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose ports
EXPOSE 8080 50051

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Default command (can be overridden)
CMD ["/app/api"]