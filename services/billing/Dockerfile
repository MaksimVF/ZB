

# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /billing-service

# Final stage
FROM alpine:latest

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /billing-service /app/billing-service

# Copy configuration files if any
# COPY config.yaml .

# Set environment variables
ENV PORT=50052
ENV REDIS_URL=redis:6379
ENV ENV=production

# Expose ports
EXPOSE 50052
EXPOSE 9090

# Health check
HEALTHCHECK --interval=30s --timeout=3s \
  CMD wget --no-verbose --tries=1 --spider http://localhost:9090/health || exit 1

# Run the application
CMD ["./billing-service"]


