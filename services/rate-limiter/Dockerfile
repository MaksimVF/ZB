# Multi-stage build for optimal image size
FROM golang:1.25-alpine AS builder

# Install dependencies
RUN apk add --no-cache protoc git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Generate protobuf Go code
RUN protoc --go_out=. --go-grpc_out=. rate_limiter.proto

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o rate-limiter .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS calls
RUN apk --no-cache add ca-certificates

WORKDIR /app/

# Create certs directory
RUN mkdir -p /certs

# Copy the binary from builder stage
COPY --from=builder /app/rate-limiter .

# Expose ports
EXPOSE 50051 8086

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8086/metrics || exit 1

# Run the application
ENTRYPOINT ["./rate-limiter"]
