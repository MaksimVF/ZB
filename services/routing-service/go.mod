module github.com/MaksimVF/ZB/services/routing-service

go 1.21

require (
	github.com/go-redis/redis/v8 v8.11.5
	github.com/gorilla/mux v1.8.0
	go.uber.org/zap v1.21.0
	google.golang.org/grpc v1.44.0
)

replace github.com/MaksimVF/ZB => ../../..
