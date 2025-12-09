module github.com/MaksimVF/ZB/services/secrets-service

go 1.25

require (
	github.com/go-redis/redis/v8 v8.11.5
	github.com/hashicorp/vault/api v1.10.0
	github.com/prometheus/client_golang v1.16.0
	github.com/rs/zerolog v1.30.0
	google.golang.org/grpc v1.56.3
)

require github.com/go-chi/chi/v5 v5.2.3 // indirect

replace github.com/MaksimVF/ZB/services/secrets-service/pb => ./pb
