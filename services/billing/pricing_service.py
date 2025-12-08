
// services/billing/pricing_service.py
import os
import json
import logging
import time
import re
from decimal import Decimal, ROUND_HALF_UP, InvalidOperation
from datetime import datetime, timedelta
from concurrent import futures
from functools import wraps

import grpc
import redis
import jwt

import billing_pb2
import billing_pb2_grpc

# =============================================================================
# Pricing Service
# =============================================================================
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("pricing_service")

# Security configuration
JWT_SECRET = os.getenv("JWT_SECRET", "default-super-secret-key-2025")
ADMIN_KEY = os.getenv("ADMIN_KEY", "default-admin-key-2025")

# Initialize services
r = redis.from_url(os.getenv("REDIS_URL", "redis://redis:6379"))

# Default pricing
DEFAULT_PRICING = {
    "gpt-4o":           {"chat_input": 5.25,  "chat_output": 15.75, "embed": 0.11},
    "gpt-4-turbo":      {"chat_input": 10.50, "chat_output": 31.50, "embed": 0.14},
    "claude-3-opus":     {"chat_input": 16.00, "chat_output": 78.00},
    "llama3-70b":        {"chat_input": 0.22, "chat_output": 0.65},
    "text-embedding-3-large":  {"embed": 0.135},
    "voyage-2":                {"embed": 0.105},
    "cohere-embed-v3":         {"embed": 0.210},
}

# Error handling
class PricingError(Exception):
    """Pricing related errors"""
    def __init__(self, message, code=None, details=None):
        super().__init__(message)
        self.code = code
        self.details = details
        self.message = message

# Pricing Manager
class PricingManager:
    """Unified pricing system with support for multiple sources"""

    def __init__(self):
        self.pricing = DEFAULT_PRICING.copy()
        self.last_updated = time.time()
        self.source = "default"

    def load_from_redis(self):
        """Load pricing from Redis"""
        saved = r.get("pricing:current")
        if saved:
            try:
                pricing_data = json.loads(saved)
                self.pricing = pricing_data
                self.source = "redis"
                self.last_updated = time.time()
                logger.info("Pricing loaded from Redis")
            except json.JSONDecodeError as e:
                logger.error(f"Failed to load pricing from Redis - invalid JSON: {e}")
                raise PricingError("Invalid pricing data in Redis")
            except Exception as e:
                logger.error(f"Failed to load pricing from Redis: {e}")
                raise PricingError("Failed to load pricing from Redis")

    def save_to_redis(self):
        """Save current pricing to Redis"""
        try:
            r.set("pricing:current", json.dumps(self.pricing))
            logger.info("Pricing saved to Redis")
        except Exception as e:
            logger.error(f"Failed to save pricing to Redis: {e}")
            raise PricingError("Failed to save pricing to Redis")

    def update_from_external_source(self, source_url: str):
        """Update pricing from external API"""
        try:
            # This would be an actual API call in production
            # For now, we'll simulate with a placeholder
            logger.info(f"Updating pricing from external source: {source_url}")

            # Validate URL
            if not source_url.startswith(("http://", "https://")):
                raise ValidationError("Invalid source URL")

            # Simulate external pricing data
            external_pricing = {
                "gpt-4o":           {"chat_input": 5.25,  "chat_output": 15.75, "embed": 0.11},
                "gpt-4-turbo":      {"chat_input": 10.50, "chat_output": 31.50, "embed": 0.14},
                "claude-3-opus":     {"chat_input": 16.00, "chat_output": 78.00},
                "llama3-70b":        {"chat_input": 0.22, "chat_output": 0.65},
                "text-embedding-3-large":  {"embed": 0.135},
                "voyage-2":                {"embed": 0.105},
                "cohere-embed-v3":         {"embed": 0.210},
            }

            # Validate pricing data
            for model_id, prices in external_pricing.items():
                try:
                    validate_model_id(model_id)
                    if not isinstance(prices, dict):
                        raise ValidationError(f"Invalid pricing format for {model_id}")
                except ValidationError as e:
                    logger.warning(f"Invalid pricing data: {e}")
                    continue

            self.pricing = external_pricing
            self.source = f"external:{source_url}"
            self.last_updated = time.time()
            self.save_to_redis()
            return True
        except ValidationError:
            raise  # Re-raise validation errors
        except Exception as e:
            logger.error(f"Failed to update pricing from external source: {e}")
            raise PricingError("Failed to update pricing from external source")

    def get_price(self, model: str, endpoint: str) -> Decimal:
        """Get price for a specific model and endpoint"""
        try:
            model_pricing = self.pricing.get(model, {})
            if endpoint == "chat":
                return {
                    "input": Decimal(str(model_pricing.get("chat_input", 10.00))) / 1_000_000,
                    "output": Decimal(str(model_pricing.get("chat_output", 30.00))) / 1_000_000
                }
            elif endpoint == "embed":
                return {
                    "embed": Decimal(str(model_pricing.get("embed", 0.13))) / 1_000_000
                }
            else:
                logger.warning(f"Unknown endpoint type: {endpoint}")
                raise PricingError(f"Unknown endpoint type: {endpoint}")
        except (InvalidOperation, ValueError) as e:
            logger.error(f"Pricing calculation error: {e}")
            raise PricingError(f"Invalid pricing data: {e}")
        except Exception as e:
            logger.error(f"Unexpected pricing error: {e}")
            raise PricingError("Failed to calculate price")

    def get_pricing_info(self):
        """Get pricing metadata"""
        return {
            "source": self.source,
            "last_updated": self.last_updated,
            "pricing": self.pricing
        }

# Initialize pricing manager
PRICING_MANAGER = PricingManager()

# Load pricing from Redis at startup
try:
    PRICING_MANAGER.load_from_redis()
except PricingError as e:
    logger.error(f"Failed to load pricing at startup: {e}")

# Pricing Service
class PricingService(billing_pb2_grpc.PricingServiceServicer):

    def GetPricing(self, request, context):
        """Get pricing for a specific model and endpoint"""
        model = request.model
        endpoint = request.endpoint

        try:
            prices = PRICING_MANAGER.get_price(model, endpoint)

            if endpoint == "chat":
                return billing_pb2.PricingResponse(
                    model=model,
                    endpoint=endpoint,
                    chat_input=prices["input"],
                    chat_output=prices["output"],
                    embed=0
                )
            elif endpoint == "embed":
                return billing_pb2.PricingResponse(
                    model=model,
                    endpoint=endpoint,
                    chat_input=0,
                    chat_output=0,
                    embed=prices["embed"]
                )
            else:
                raise PricingError(f"Unknown endpoint type: {endpoint}")
        except PricingError as e:
            logger.error(f"Pricing error: {e}")
            context.abort_with_status(grpc.StatusCode.INVALID_ARGUMENT, str(e))
        except Exception as e:
            logger.error(f"Unexpected error: {e}")
            context.abort_with_status(grpc.StatusCode.INTERNAL, "Internal server error")

    def UpdatePricing(self, request, context):
        """Update pricing from external source"""
        source_url = request.source_url

        try:
            success = PRICING_MANAGER.update_from_external_source(source_url)
            if success:
                return billing_pb2.UpdateResponse(success=True)
            else:
                raise PricingError("Failed to update pricing")
        except PricingError as e:
            logger.error(f"Pricing update error: {e}")
            context.abort_with_status(grpc.StatusCode.INVALID_ARGUMENT, str(e))
        except Exception as e:
            logger.error(f"Unexpected error: {e}")
            context.abort_with_status(grpc.StatusCode.INTERNAL, "Internal server error")

    def GetPricingInfo(self, request, context):
        """Get pricing metadata"""
        try:
            pricing_info = PRICING_MANAGER.get_pricing_info()
            return billing_pb2.PricingInfoResponse(
                source=pricing_info["source"],
                last_updated=pricing_info["last_updated"],
                pricing=json.dumps(pricing_info["pricing"])
            )
        except Exception as e:
            logger.error(f"Error getting pricing info: {e}")
            context.abort_with_status(grpc.StatusCode.INTERNAL, "Internal server error")

# =============================================================================
# Запуск
# =============================================================================
def serve():
    # gRPC
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    billing_pb2_grpc.add_PricingServiceServicer_to_server(PricingService(), server)
    server.add_insecure_port("[::]:50053")

    logger.info("Pricing Service: gRPC :50053")
    server.start()
    server.wait_for_termination()

if __name__ == "__main__":
    serve()





