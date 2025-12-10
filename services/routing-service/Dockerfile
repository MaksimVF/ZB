

# Use the official Golang image to create a binary
FROM golang:1.25-alpine AS builder

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download and tidy dependencies to ensure go.sum is up to date
RUN go mod tidy

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Build the Go app
RUN go build -o routing-service ./...

# Start a new stage from scratch
FROM alpine:latest

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/routing-service /usr/local/bin/routing-service

# Copy the certs
COPY certs /certs

# Expose the port the app runs on
EXPOSE 50061 8080

# Command to run the executable
CMD ["routing-service"]

