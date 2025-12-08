




import logging
import sys
from pythonjsonlogger import jsonlogger

# Logging configuration
def configure_logging(service_name: str, log_level: str = 'INFO'):
    """Configure logging for the service"""
    # Create logger
    logger = logging.getLogger(service_name)
    logger.setLevel(getattr(logging, log_level.upper(), logging.INFO))

    # Create console handler
    console_handler = logging.StreamHandler(sys.stdout)
    console_handler.setLevel(logging.INFO)

    # Create file handler
    file_handler = logging.FileHandler(f'/var/log/billing/{service_name}.log')
    file_handler.setLevel(logging.DEBUG)

    # Create JSON formatter for file handler
    json_formatter = jsonlogger.JsonFormatter(
        '%(asctime)s %(name)s %(levelname)s %(message)s %(pathname)s %(lineno)d'
    )
    file_handler.setFormatter(json_formatter)

    # Create simple formatter for console
    console_formatter = logging.Formatter(
        '%(asctime)s - %(name)s - %(levelname)s - %(message)s'
    )
    console_handler.setFormatter(console_formatter)

    # Add handlers to logger
    if not logger.handlers:
        logger.addHandler(console_handler)
        logger.addHandler(file_handler)

    # Add Loki handler if needed
    # loki_handler = LokiHandler(url='http://loki:3100/loki/api/v1/push')
    # logger.addHandler(loki_handler)

    return logger

# Loki handler
class LokiHandler(logging.Handler):
    """Logging handler for Loki"""

    def __init__(self, url: str, labels: dict = None):
        super().__init__()
        self.url = url
        self.labels = labels or {}

    def emit(self, record):
        """Send log record to Loki"""
        try:
            log_entry = self.format(record)
            # Send to Loki via HTTP POST
            # Implement HTTP POST to Loki
            pass
        except Exception as e:
            print(f"Failed to send log to Loki: {e}")

# Configure logging for each service
billing_logger = configure_logging('billing_core')
pricing_logger = configure_logging('pricing_service')
exchange_logger = configure_logging('exchange_service')
monitoring_logger = configure_logging('monitoring_service')
admin_logger = configure_logging('admin_service')









