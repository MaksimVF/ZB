module github.com/MaksimVF/ZB/services/head-go

go 1.21

require (
    github.com/golang-jwt/jwt/v5 v5.2.0
    github.com/grpc-ecosystem/grpc-gateway/v2 v2.16.0
    github.com/sony/gobreaker v0.0.0-20220816042456-f289a7f34483
    github.com/stretchr/testify v1.8.4
    go.opentelemetry.io/otel v1.22.0
    go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.22.0
    go.opentelemetry.io/otel/sdk/resource v1.22.0
    go.opentelemetry.io/otel/sdk/trace v1.22.0
    go.opentelemetry.io/otel/trace v1.22.0
    google.golang.org/grpc v1.58.3
    github.com/grpc-ecosystem/go-grpc-middleware v1.4.0
    github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
    github.com/prometheus/client_golang v1.17.0
)
