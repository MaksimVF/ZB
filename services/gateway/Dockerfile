






FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy source code
COPY services/gateway/ ./

# Build the application
RUN go mod init llm-gateway-pro/services/gateway && \
    go mod tidy && \
    go build -o /gateway ./main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /gateway .

EXPOSE 8080

ENTRYPOINT ["./gateway"]






