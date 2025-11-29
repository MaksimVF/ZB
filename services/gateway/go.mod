





module llm-gateway-pro/services/gateway

go 1.21

require (
	github.com/gorilla/mux v1.8.0
	github.com/lib/pq v1.10.9
	github.com/prometheus/client_golang v1.16.0
	github.com/rs/zerolog v1.30.0
	github.com/stretchr/testify v1.8.4
)

replace llm-gateway-pro/services/gateway/internal/secrets => ./internal/secrets
replace llm-gateway-pro/services/gateway/internal/handlers => ./internal/handlers
replace llm-gateway-pro/services/gateway/internal/billing => ./internal/billing
replace llm-gateway-pro/services/gateway/internal/providers => ./internal/providers



