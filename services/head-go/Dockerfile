FROM golang:1.21-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/head ./cmd/head
FROM alpine:3.18
RUN apk add --no-cache ca-certificates
COPY --from=build /bin/head /bin/head
EXPOSE 50055 9001
ENTRYPOINT ["/bin/head"]
