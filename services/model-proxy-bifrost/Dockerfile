

# Multi-stage build for Bifrost-based model proxy replacement
# Stage 1: Build gRPC adapter
FROM golang:1.21-alpine AS grpc-adapter-builder

WORKDIR /app

# Copy gRPC adapter source
COPY grpc-adapter/go.mod grpc-adapter/go.sum grpc-adapter/main.go ./
COPY ../../proto/model.proto ./model.proto

# Build the gRPC adapter
RUN CGO_ENABLED=0 GOOS=linux go build -o /grpc-adapter main.go

# Stage 2: Final image with Bifrost and gRPC adapter
FROM maximhq/bifrost:latest

# Copy custom configuration
COPY config.json /app/data/config.json

# Copy gRPC adapter from builder stage
COPY --from=grpc-adapter-builder /grpc-adapter /grpc-adapter
COPY grpc-adapter/Dockerfile /grpc-adapter-dockerfile

# Copy proto file for reference
COPY ../../proto/model.proto /model.proto

# Copy TLS certificates for mTLS
COPY ../../certs/ca.pem /certs/ca.pem
COPY ../../certs/model-proxy.pem /certs/model-proxy.pem
COPY ../../certs/model-proxy-key.pem /certs/model-proxy-key.pem

# Set environment variables
ENV BIFROST_HOST=0.0.0.0
ENV PORT=8100
ENV LOG_LEVEL=info
ENV BIFROST_URL=http://localhost:8100

# Expose ports (HTTP API and gRPC)
EXPOSE 8100 50061 50062

# Start both Bifrost and gRPC adapter using a simple shell script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

CMD ["/entrypoint.sh"]

