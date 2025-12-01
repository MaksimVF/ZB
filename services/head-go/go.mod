module github.com/yourorg/head

go 1.21

require (
    github.com/sony/gobreaker v2.0.0+incompatible
    go.opentelemetry.io/otel v1.22.0
    go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.22.0
    go.opentelemetry.io/otel/sdk/resource v1.22.0
    go.opentelemetry.io/otel/sdk/trace v1.22.0
    go.opentelemetry.io/otel/trace v1.22.0
    google.golang.org/grpc v1.58.3
    github.com/grpc-ecosystem/go-grpc-middleware v2.1.0
    github.com/grpc-ecosystem/go-grpc-prometheus v2.0.0
    github.com/prometheus/client_golang v1.17.0
)
