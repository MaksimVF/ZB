







module github.com/MaksimVF/ZB/services/secrets-service

go 1.21

require (
github.com/go-redis/redis/v8 v8.11.5
github.com/hashicorp/vault/api v1.10.0
google.golang.org/grpc v1.56.3
google.golang.org/grpc/credentials v1.56.3
)

replace llm-gateway-pro/services/secret-service/pb => ../pb

