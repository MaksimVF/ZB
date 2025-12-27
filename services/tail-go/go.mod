module github.com/MaksimVF/ZB/services/tail-go

go 1.23

require (
	github.com/MaksimVF/ZB/services/secrets-service v0.0.0-20251227074416-d7cb3e0c8e04
	github.com/go-redis/redis/v8 v8.11.5
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/google/uuid v1.6.0
	github.com/gorilla/mux v1.8.1
	github.com/prometheus/client_golang v1.20.5
	github.com/redis/go-redis/v9 v9.7.0
	github.com/spf13/viper v1.19.0
	go.uber.org/zap v1.27.0
	google.golang.org/grpc v1.67.1
	google.golang.org/protobuf v1.36.1
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	golang.org/x/net v0.28.0 // indirect
	golang.org/x/sys v0.24.0 // indirect
	golang.org/x/text v0.17.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240814211410-ddb44dafa142 // indirect
)
