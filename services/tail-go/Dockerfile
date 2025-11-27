FROM golang:1.21-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/tail ./
FROM alpine:3.18
RUN apk add --no-cache ca-certificates
COPY --from=build /bin/tail /bin/tail
EXPOSE 8000
ENTRYPOINT ["/bin/tail"]
