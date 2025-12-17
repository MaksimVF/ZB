module github.com/MaksimVF/ZB/services/agentic-service

go 1.22

require (
	github.com/go-redis/redis/v8 v8.11.5
	github.com/gorilla/mux v1.8.0
	google.golang.org/grpc v1.56.3
	google.golang.org/protobuf v1.31.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/go-chi/chi/v5 v5.2.3 // indirect
	github.com/redis/go-redis/v9 v9.17.2 // indirect
)

replace github.com/MaksimVF/ZB/services/head-go/gen => ../head-go/gen

replace github.com/MaksimVF/ZB/services/agentic-service/gen => ./gen

replace github.com/MaksimVF/ZB/services/agentic-service/grpc => ./grpc
