





FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy proto files
COPY services/rate-limiter/rate_limiter.proto .

# Install protoc and Go plugins
RUN apk add --no-cache protoc protoc-gen-go protoc-gen-go-grpc && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate Go code from proto
RUN protoc --go_out=. --go-grpc_out=. rate_limiter.proto

# Copy source code
COPY services/rate-limiter/ ./

# Build the application
RUN go mod init llm-gateway-pro/services/rate-limiter && \
    go mod tidy && \
    go build -o /rate-limiter ./main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /rate-limiter .

EXPOSE 50051

ENTRYPOINT ["./rate-limiter"]





