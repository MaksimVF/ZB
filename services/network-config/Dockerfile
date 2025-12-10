


FROM golang:1.25-alpine

WORKDIR /app

# Copy only the go.mod and go.sum files first
COPY go.mod ./
COPY go.sum ./

# Download all dependencies and update go.sum
RUN go mod download \
    && go get github.com/go-redis/redis/v8 \
    && go get github.com/gorilla/mux \
    && go get go.uber.org/zap

# Copy the rest of the source code
COPY . .

# Build the Go app
RUN go build -o /network-config-service

EXPOSE 50060

CMD ["/network-config-service"]


