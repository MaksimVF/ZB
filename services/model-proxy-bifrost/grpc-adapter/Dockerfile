


# gRPC adapter for Bifrost model proxy
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the adapter
RUN CGO_ENABLED=0 GOOS=linux go build -o /grpc-adapter main.go

# Final stage
FROM alpine:latest

WORKDIR /app

# Copy the built binary
COPY --from=builder /grpc-adapter /grpc-adapter

# Copy proto files for reference
COPY ../../proto/model.proto /model.proto

# Set environment variables
ENV BIFROST_URL=http://localhost:8100
ENV GRPC_PORT=50061

# Expose gRPC port
EXPOSE 50061

# Start the gRPC adapter
CMD ["/grpc-adapter"]


