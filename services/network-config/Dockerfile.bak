


FROM golang:1.25-alpine

WORKDIR /app

# Copy go.mod and go.sum first to cache dependencies
COPY go.mod ./
COPY go.sum ./

# Download and tidy dependencies to ensure go.sum is up to date
RUN go mod tidy

# Copy the rest of the application code
COPY . .

# Build the application
RUN go build -o /network-config-service

EXPOSE 50060

CMD ["/network-config-service"]


