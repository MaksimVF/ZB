




import time
import logging
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.grpc import GrpcInstrumentor
from opentelemetry.instrumentation.flask import FlaskInstrumentor
from opentelemetry.instrumentation.redis import RedisInstrumentor

# Configure tracing
def configure_tracing(service_name: str, endpoint: str = "http://jaeger:4317"):
    """Configure OpenTelemetry tracing"""
    # Set up trace provider
    trace.set_tracer_provider(TracerProvider())

    # Set up OTLP exporter
    otlp_exporter = OTLPSpanExporter(endpoint=endpoint)

    # Set up span processor
    span_processor = BatchSpanProcessor(otlp_exporter)
    trace.get_tracer_provider().add_span_processor(span_processor)

    # Instrument libraries
    GrpcInstrumentor().instrument()
    FlaskInstrumentor().instrument()
    RedisInstrumentor().instrument()

    # Create tracer
    tracer = trace.get_tracer(service_name)

    return tracer

# Create tracers for each service
billing_tracer = configure_tracing('billing_core')
pricing_tracer = configure_tracing('pricing_service')
exchange_tracer = configure_tracing('exchange_service')
monitoring_tracer = configure_tracing('monitoring_service')
admin_tracer = configure_tracing('admin_service')

# Tracing decorator
def trace_method(tracer, method_name: str):
    """Decorator for tracing methods"""
    def decorator(f):
        def wrapper(*args, **kwargs):
            with tracer.start_as_current_span(method_name) as span:
                try:
                    result = f(*args, **kwargs)
                    span.set_status(trace.StatusCode.OK)
                    return result
                except Exception as e:
                    span.set_status(trace.StatusCode.ERROR)
                    span.record_exception(e)
                    raise
        return wrapper
    return decorator

# Example usage
@trace_method(billing_tracer, 'charge_operation')
def charge(user_id: str, amount: float):
    """Example traced method"""
    # Implementation here
    pass









