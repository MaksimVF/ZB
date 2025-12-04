


FROM golang:1.21-alpine

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

RUN go build -o /network-config-service

EXPOSE 50060

CMD ["/network-config-service"]


