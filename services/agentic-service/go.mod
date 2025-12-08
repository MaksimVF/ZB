


module github.com/MaksimVF/ZB/services/agentic-service

go 1.22

require (
	github.com/go-redis/redis/v8 v8.11.5
	github.com/gorilla/mux v1.8.0
	google.golang.org/grpc v1.56.3
	google.golang.org/protobuf v1.31.0
)

replace github.com/MaksimVF/ZB/services/secrets-service/pb => ../secrets-service/pb
replace github.com/MaksimVF/ZB/services/head-go/gen => ../head-go/gen

