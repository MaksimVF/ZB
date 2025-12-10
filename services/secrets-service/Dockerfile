






# Use the official Golang image as the base image
FROM golang:1.25-alpine

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy the go.mod and go.sum files
COPY go.mod go.sum ./

# Download all dependencies and update go.sum
RUN go mod download \
    && go get google.golang.org/grpc \
    && go get github.com/hashicorp/vault/api \
    && go get github.com/prometheus/client_golang/prometheus \
    && go get github.com/rs/zerolog \
    && go get go.opentelemetry.io/otel \
    && go get go.opentelemetry.io/otel/exporters/stdout/stdouttrace \
    && go get go.opentelemetry.io/otel/sdk/resource \
    && go get go.opentelemetry.io/otel/sdk/trace \
    && go get go.opentelemetry.io/otel/semconv/v1.4.0

# Copy the source code into the container
COPY . .

# Build the Go app
RUN go build -o secret-service .

# Expose ports
EXPOSE 50053 8082

# Command to run the executable
CMD ["./secret-service"]












