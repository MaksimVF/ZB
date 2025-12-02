


module llm-gateway-pro/services/agentic-service

go 1.22

require (
	github.com/go-redis/redis/v8 v8.11.5
	github.com/gorilla/mux v1.8.0
	google.golang.org/grpc v1.56.3
	google.golang.org/protobuf v1.31.0
)

replace llm-gateway-pro/services/secret-service/pb => ../secret-service/pb
replace llm-gateway-pro/services/head-go/gen => ../head-go/gen

