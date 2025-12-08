



import time
from prometheus_client import start_http_server, Summary, Counter, Gauge, Histogram
from prometheus_client.core import REGISTRY

# Metrics setup
REQUEST_TIME = Summary('request_processing_seconds', 'Time spent processing request')
REQUESTS_TOTAL = Counter('http_requests_total', 'Total HTTP requests', ['method', 'endpoint', 'status'])
REQUESTS_IN_PROGRESS = Gauge('http_requests_in_progress', 'HTTP requests in progress')
REQUEST_DURATION = Histogram('http_request_duration_seconds', 'HTTP request duration', ['method', 'endpoint'])
USER_BALANCE = Gauge('user_balance', 'User balance', ['user_id'])
TOKENS_USED = Counter('tokens_used_total', 'Total tokens used', ['user_id', 'model'])
ERRORS_TOTAL = Counter('errors_total', 'Total errors', ['service', 'error_type'])
SERVICE_HEALTH = Gauge('service_health', 'Service health status', ['service'])

# Initialize metrics
def init_metrics(service_name: str, port: int = 8000):
    """Initialize Prometheus metrics"""
    # Start metrics server
    start_http_server(port)
    logger.info(f"Prometheus metrics server started on port {port}")

    # Set service health
    SERVICE_HEALTH.labels(service=service_name).set(1)

    # Register custom collectors if needed
    # REGISTRY.register(CustomCollector())

# Metrics middleware
def metrics_middleware(f):
    """Decorator for tracking metrics"""
    def wrapper(*args, **kwargs):
        method = request.method
        endpoint = request.path
        start_time = time.time()

        # Increment in-progress requests
        REQUESTS_IN_PROGRESS.inc()

        try:
            response = f(*args, **kwargs)
            status = 200 if response.status_code < 400 else response.status_code

            # Record metrics
            REQUESTS_TOTAL.labels(method=method, endpoint=endpoint, status=status).inc()
            REQUEST_DURATION.labels(method=method, endpoint=endpoint).observe(time.time() - start_time)

            return response
        except Exception as e:
            # Record error
            ERRORS_TOTAL.labels(service='billing', error_type=type(e).__name__).inc()
            raise
        finally:
            # Decrement in-progress requests
            REQUESTS_IN_PROGRESS.dec()

    return wrapper

# Track user balance
def track_user_balance(user_id: str, balance: float):
    """Track user balance metric"""
    USER_BALANCE.labels(user_id=user_id).set(balance)

# Track token usage
def track_token_usage(user_id: str, model: str, tokens: int):
    """Track token usage metric"""
    TOKENS_USED.labels(user_id=user_id, model=model).inc(tokens)

# Track service health
def track_service_health(service: str, status: int):
    """Track service health metric"""
    SERVICE_HEALTH.labels(service=service).set(status)

# Custom metrics collector
class CustomCollector:
    """Custom metrics collector for Prometheus"""

    def collect(self):
        # Collect custom metrics
        yield from [
            # Example custom metric
            Gauge('custom_metric', 'Custom metric example').set(42)
        ]








