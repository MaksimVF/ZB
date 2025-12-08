

# Agentic Service Dockerfile
FROM golang:1.22-alpine

WORKDIR /app

# Copy source code
COPY . .

# Build the service
RUN go mod tidy && \
    go build -o /agentic-service main.go

# Create certs directory
RUN mkdir -p /certs

# Run the service
CMD ["/agentic-service"]

EXPOSE 8081

