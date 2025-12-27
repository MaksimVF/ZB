# Build stage
FROM golang:1.23-alpine AS builder
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -extldflags '-static'" \
    -a -installsuffix cgo \
    -o tail ./cmd/tail

# Final stage with distroless image for security
FROM gcr.io/distroless/static-debian12
WORKDIR /

# Copy binary and CA certificates
COPY --from=builder /app/tail /tail
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Create non-root user (distroless already runs as non-root)
USER 65534:65534

# Expose port
EXPOSE 8443

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/tail", "--health-check"]

# Run the binary
ENTRYPOINT ["/tail"]
