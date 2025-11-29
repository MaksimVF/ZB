


module github.com/MaksimVF/ZB/services/auth-service

go 1.21

require (
	github.com/go-redis/redis/v8 v8.11.5
	github.com/golang-jwt/jwt/v5 v5.0.0
	github.com/gorilla/mux v1.8.0
	github.com/google/uuid v1.3.0
	github.com/prometheus/client_golang v1.16.0
	github.com/rs/zerolog v1.30.0
	github.com/stretchr/testify v1.8.4
	golang.org/x/crypto v0.14.0
	google.golang.org/grpc v1.56.3
	gorm.io/driver/postgres v1.5.2
	gorm.io/gorm v1.25.5
)

replace llm-gateway-pro/services/auth-service/pb => ../pb


