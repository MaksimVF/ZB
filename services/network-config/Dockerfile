


FROM golang:1.25-alpine

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

# Install missing dependencies that are not in go.sum
RUN go get github.com/go-redis/redis/v8 \
    && go get github.com/gorilla/mux \
    && go get go.uber.org/zap

COPY . .

RUN go build -o /network-config-service

EXPOSE 50060

CMD ["/network-config-service"]


