
package metrics

import (
    "context"
    "log"
    "os"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/propagation"
    "go.opentelemetry.io/otel/sdk/resource"
    "go.opentelemetry.io/otel/sdk/trace"
    "go.opentelemetry.io/otel/trace/noop"
    "google.golang.org/grpc/credentials"
)

var tracer = noop.NewTracerProvider().Tracer("head-go")

// InitializeTracing sets up OpenTelemetry tracing
func InitializeTracing(ctx context.Context) error {
    // Check if tracing is enabled
    if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
        log.Println("Tracing disabled: OTEL_EXPORTER_OTLP_ENDPOINT not set")
        return nil
    }

    // Create a secure gRPC connection to the collector
    var opts []otlptracegrpc.Option
    if os.Getenv("OTEL_EXPORTER_OTLP_INSECURE") != "true" {
        creds := credentials.NewClientTLSFromCert(nil, "")
        opts = append(opts, otlptracegrpc.WithTLSCredentials(creds))
    }

    // Create OTLP exporter
    exporter, err := otlptracegrpc.New(ctx, opts...)
    if err != nil {
        return err
    }

    // Create resource with service name
    res, err := resource.New(ctx,
        resource.WithAttributes(
            resource.String("service.name", "head-go"),
            resource.String("service.version", "1.0.0"),
        ),
    )
    if err != nil {
        return err
    }

    // Create tracer provider
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(res),
    )

    // Set global tracer provider and propagator
    otel.SetTracerProvider(tp)
    otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
        propagation.TraceContext{},
        propagation.Baggage{},
    ))

    // Set the global tracer
    tracer = tp.Tracer("head-go")

    log.Println("Tracing initialized successfully")
    return nil
}

// GetTracer returns the global tracer
func GetTracer() trace.Tracer {
    return tracer
}

// ShutdownTracing shuts down the tracer provider
func ShutdownTracing(ctx context.Context) error {
    tp, ok := otel.GetTracerProvider().(*trace.TracerProvider)
    if !ok {
        return nil
    }
    return tp.Shutdown(ctx)
}
