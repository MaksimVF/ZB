
// services/billing/server.py
import os
import json
import logging
import time
import uuid
import re
import threading
from decimal import Decimal, ROUND_HALF_UP, InvalidOperation
from datetime import datetime, timedelta
from concurrent import futures
from functools import wraps

import grpc
import redis
import stripe
import jwt
import requests
from flask import Flask, request, jsonify, render_template_string
from flask_limiter import Limiter
from flask_limiter.util import get_remote_address

import billing_pb2
import billing_pb2_grpc

# =============================================================================
# Конфигурация
# =============================================================================
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("billing")

# Security configuration
JWT_SECRET = os.getenv("JWT_SECRET", "default-super-secret-key-2025")
ADMIN_KEY = os.getenv("ADMIN_KEY", "default-admin-key-2025")
STRIPE_WEBHOOK_SECRET = os.getenv("STRIPE_WEBHOOK_SECRET")

# Initialize services
r = redis.from_url(os.getenv("REDIS_URL", "redis://redis:6379"))
stripe.api_key = os.getenv("STRIPE_SECRET_KEY")

# Курсы валют (обновляются раз в час)
EXCHANGE_RATES = {
    "USD": Decimal("1"),
    "EUR": Decimal("0.92"),
    "RUB": Decimal("96.50"),
    "USDT": Decimal("1"),
}

# Цены за 1M токенов (USD)
DEFAULT_PRICING = {
    # Chat модели
    "gpt-4o":           {"chat_input": 5.00,  "chat_output": 15.00, "embed": 0.10},
    "gpt-4-turbo":      {"chat_input": 10.00, "chat_output": 30.00, "embed": 0.13},
    "claude-3-opus":    {"chat_input": 15.00, "chat_output": 75.00},
    "llama3-70b":       {"chat_input": 0.20, "chat_output": 0.60},

    # Embedding модели
    "text-embedding-3-large":  {"embed": 0.130},
    "voyage-2":                {"embed": 0.100},
    "cohere-embed-v3":         {"embed": 0.200},
}

# Время жизни резервации (10 минут)
RESERVATION_TTL = 600

# Input validation patterns
USER_ID_PATTERN = re.compile(r'^[a-zA-Z0-9_\-]{3,64}$')
MODEL_ID_PATTERN = re.compile(r'^[a-zA-Z0-9_\-\.]{2,64}$')
RESERVATION_ID_PATTERN = re.compile(r'^res:[a-zA-Z0-9_\-]{3,64}:[a-zA-Z0-9_\-]{3,64}:\d+$')

# Error handling
class BillingError(Exception):
    """Base class for billing errors"""
    def __init__(self, message, code=None, details=None):
        super().__init__(message)
        self.code = code
        self.details = details
        self.message = message

class AuthenticationError(BillingError):
    """Authentication related errors"""
    pass

class ValidationError(BillingError):
    """Input validation errors"""
    pass

class BalanceError(BillingError):
    """Balance related errors"""
    pass

class PricingError(BillingError):
    """Pricing related errors"""
    pass

class ReservationError(BillingError):
    """Reservation related errors"""
    pass

class ExternalServiceError(BillingError):
    """External service errors"""
    pass

# Error handling decorator
def handle_billing_errors(f):
    """Decorator for handling billing errors"""
    @wraps(f)
    def wrapper(*args, **kwargs):
        try:
            return f(*args, **kwargs)
        except AuthenticationError as e:
            logger.warning(f"Authentication error: {e}")
            if len(args) > 1 and hasattr(args[1], 'abort_with_status'):
                # gRPC context
                args[1].abort_with_status(grpc.StatusCode.UNAUTHENTICATED, str(e))
            else:
                # HTTP context
                return jsonify({"error": str(e), "code": e.code}), 401
        except ValidationError as e:
            logger.warning(f"Validation error: {e}")
            if len(args) > 1 and hasattr(args[1], 'abort_with_status'):
                # gRPC context
                args[1].abort_with_status(grpc.StatusCode.INVALID_ARGUMENT, str(e))
            else:
                # HTTP context
                return jsonify({"error": str(e), "code": e.code}), 400
        except BalanceError as e:
            logger.warning(f"Balance error: {e}")
            if len(args) > 1 and hasattr(args[1], 'abort_with_status'):
                # gRPC context
                return billing_pb2.BillResponse(success=False, error=str(e), new_balance=0)
            else:
                # HTTP context
                return jsonify({"error": str(e), "code": e.code}), 400
        except PricingError as e:
            logger.error(f"Pricing error: {e}")
            if len(args) > 1 and hasattr(args[1], 'abort_with_status'):
                # gRPC context
                return billing_pb2.BillResponse(success=False, error=str(e), new_balance=0)
            else:
                # HTTP context
                return jsonify({"error": str(e), "code": e.code}), 400
        except ReservationError as e:
            logger.warning(f"Reservation error: {e}")
            if len(args) > 1 and hasattr(args[1], 'abort_with_status'):
                # gRPC context
                return billing_pb2.ReserveResponse(success=False, error=str(e), reserved_amount=0, remaining_balance=0)
            else:
                # HTTP context
                return jsonify({"error": str(e), "code": e.code}), 400
        except ExternalServiceError as e:
            logger.error(f"External service error: {e}")
            if len(args) > 1 and hasattr(args[1], 'abort_with_status'):
                # gRPC context
                args[1].abort_with_status(grpc.StatusCode.INTERNAL, str(e))
            else:
                # HTTP context
                return jsonify({"error": str(e), "code": e.code}), 500
        except Exception as e:
            logger.error(f"Unexpected error: {e}")
            if len(args) > 1 and hasattr(args[1], 'abort_with_status'):
                # gRPC context
                args[1].abort_with_status(grpc.StatusCode.INTERNAL, "Internal server error")
            else:
                # HTTP context
                return jsonify({"error": "Internal server error"}), 500
    return wrapper

# Security helpers
def validate_jwt(token: str) -> bool:
    """Validate JWT token"""
    try:
        if not token:
            raise AuthenticationError("Missing JWT token")
        decoded = jwt.decode(token, JWT_SECRET, algorithms=["HS256"])
        return True
    except (jwt.ExpiredSignatureError, jwt.InvalidTokenError) as e:
        logger.warning(f"Invalid JWT: {e}")
        raise AuthenticationError("Invalid JWT token")
    except Exception as e:
        logger.error(f"JWT validation error: {e}")
        raise AuthenticationError("JWT validation failed")

def validate_user_id(user_id: str) -> bool:
    """Validate user ID format"""
    if not USER_ID_PATTERN.match(user_id):
        raise ValidationError(f"Invalid user_id format: {user_id}")
    return True

def validate_model_id(model_id: str) -> bool:
    """Validate model ID format"""
    if not MODEL_ID_PATTERN.match(model_id):
        raise ValidationError(f"Invalid model_id format: {model_id}")
    return True

def validate_reservation_id(reservation_id: str) -> bool:
    """Validate reservation ID format"""
    if not RESERVATION_ID_PATTERN.match(reservation_id):
        raise ValidationError(f"Invalid reservation_id format: {reservation_id}")
    return True

def validate_amount(amount: Decimal) -> bool:
    """Validate monetary amount"""
    if amount <= 0 or amount >= 1000000:
        raise ValidationError(f"Invalid amount: {amount}")
    return True

# Exchange rate manager
class ExchangeRateManager:
    """Automated exchange rate updater with admin currency management"""

    def __init__(self):
        self.rates = EXCHANGE_RATES.copy()
        self.last_updated = 0
        self.update_interval = 3600  # 1 hour
        self.api_url = "https://api.exchangerate-api.com/v4/latest/USD"
        self.lock = threading.Lock()
        self.supported_currencies = ["USD", "EUR", "RUB", "USDT", "GBP", "CNY", "JPY", "INR"]

    def fetch_exchange_rates(self):
        """Fetch exchange rates from external API"""
        try:
            with self.lock:
                logger.info("Fetching exchange rates from external API")
                response = requests.get(self.api_url, timeout=10)
                response.raise_for_status()

                data = response.json()
                if "rates" not in data:
                    raise ExternalServiceError("Invalid exchange rate API response")

                # Update rates for supported currencies
                new_rates = {"USD": Decimal("1")}  # USD is always 1

                for currency in self.supported_currencies:
                    if currency == "USD":
                        continue
                    if currency == "USDT":
                        new_rates[currency] = Decimal("1")  # USDT is pegged to USD
                    else:
                        rate = data["rates"].get(currency)
                        if rate:
                            new_rates[currency] = Decimal(str(rate))
                        else:
                            logger.warning(f"Currency {currency} not found in API response, using default")
                            # Keep existing rate or use default
                            new_rates[currency] = self.rates.get(currency, Decimal("1"))

                self.rates = new_rates
                self.last_updated = int(time.time())

                # Save to Redis
                r.set("exchange_rates:current", json.dumps(new_rates))
                r.set("exchange_rates:last_updated", self.last_updated)
                r.set("exchange_rates:supported", json.dumps(self.supported_currencies))

                logger.info(f"Exchange rates updated: {new_rates}")
                return True
        except requests.exceptions.RequestException as e:
            logger.error(f"Failed to fetch exchange rates: {e}")
            raise ExternalServiceError("Failed to fetch exchange rates")
        except (json.JSONDecodeError, KeyError) as e:
            logger.error(f"Invalid exchange rate API response: {e}")
            raise ExternalServiceError("Invalid exchange rate API response")
        except Exception as e:
            logger.error(f"Unexpected error fetching exchange rates: {e}")
            raise ExternalServiceError("Failed to update exchange rates")

    def load_from_redis(self):
        """Load exchange rates from Redis"""
        try:
            with self.lock:
                saved_rates = r.get("exchange_rates:current")
                last_updated = r.get("exchange_rates:last_updated")
                supported_currencies = r.get("exchange_rates:supported")

                if saved_rates and last_updated:
                    self.rates = json.loads(saved_rates)
                    self.last_updated = int(last_updated)
                    if supported_currencies:
                        self.supported_currencies = json.loads(supported_currencies)
                    logger.info(f"Exchange rates loaded from Redis, last updated: {datetime.fromtimestamp(self.last_updated)}")
                    return True
                return False
        except Exception as e:
            logger.error(f"Failed to load exchange rates from Redis: {e}")
            return False

    def get_rate(self, currency: str) -> Decimal:
        """Get exchange rate for a specific currency"""
        if currency not in self.rates:
            raise ValidationError(f"Unsupported currency: {currency}")
        return self.rates[currency]

    def add_currency(self, currency: str, rate: Decimal):
        """Add a new currency to the system"""
        if currency in self.rates:
            raise ValidationError(f"Currency {currency} already exists")

        if not currency.isalpha() or len(currency) != 3:
            raise ValidationError(f"Invalid currency code: {currency}")

        with self.lock:
            self.rates[currency] = rate
            self.supported_currencies.append(currency)

            # Save to Redis
            r.set("exchange_rates:current", json.dumps(self.rates))
            r.set("exchange_rates:supported", json.dumps(self.supported_currencies))

            logger.info(f"Added new currency: {currency} = {rate}")
            return True

    def remove_currency(self, currency: str):
        """Remove a currency from the system"""
        if currency == "USD" or currency == "USDT":
            raise ValidationError("Cannot remove base currencies (USD, USDT)")

        if currency not in self.rates:
            raise ValidationError(f"Currency {currency} not found")

        with self.lock:
            del self.rates[currency]
            self.supported_currencies.remove(currency)

            # Save to Redis
            r.set("exchange_rates:current", json.dumps(self.rates))
            r.set("exchange_rates:supported", json.dumps(self.supported_currencies))

            logger.info(f"Removed currency: {currency}")
            return True

    def update_currency_rate(self, currency: str, rate: Decimal):
        """Update exchange rate for a specific currency"""
        if currency not in self.rates:
            raise ValidationError(f"Currency {currency} not found")

        with self.lock:
            self.rates[currency] = rate
            self.last_updated = int(time.time())

            # Save to Redis
            r.set("exchange_rates:current", json.dumps(self.rates))
            r.set("exchange_rates:last_updated", self.last_updated)

            logger.info(f"Updated currency rate: {currency} = {rate}")
            return True

    def start_auto_update(self):
        """Start automatic exchange rate updates"""
        def update_loop():
            while True:
                try:
                    time.sleep(self.update_interval)
                    self.fetch_exchange_rates()
                except Exception as e:
                    logger.error(f"Exchange rate update failed: {e}")
                    time.sleep(60)  # Retry after 1 minute on failure

        threading.Thread(target=update_loop, daemon=True, name="ExchangeRateUpdater").start()

# Initialize exchange rate manager
EXCHANGE_MANAGER = ExchangeRateManager()

# Load exchange rates from Redis at startup
try:
    if not EXCHANGE_MANAGER.load_from_redis():
        # Fetch fresh rates if not available in Redis
        EXCHANGE_MANAGER.fetch_exchange_rates()
except Exception as e:
    logger.error(f"Failed to initialize exchange rates: {e}")

# Start automatic exchange rate updates
EXCHANGE_MANAGER.start_auto_update()

# Pricing system
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

# =============================================================================
# gRPC сервис
# =============================================================================
class BillingService(billing_pb2_grpc.BillingServiceServicer):

    @handle_billing_errors
    def Charge(self, request, context):
        """Direct usage recording without reservation"""
        # Authentication check
        metadata = context.invocation_metadata()
        auth_token = None
        for key, value in metadata:
            if key == "authorization":
                auth_token = value
                break

        if not auth_token:
            raise AuthenticationError("Missing authentication token")
        if not validate_jwt(auth_token):
            raise AuthenticationError("Invalid authentication token")

        # Input validation
        user_id = request.user_id or "anonymous"
        model = request.model
        tokens_used = request.tokens_used
        cost = Decimal(str(request.cost))

        # Validate inputs
        validate_user_id(user_id)
        validate_model_id(model)

        if tokens_used <= 0:
            raise ValidationError("Invalid tokens_used value")

        if cost <= 0:
            raise ValidationError("Invalid cost value")

        # Check if cost was provided, otherwise calculate it
        # For now, we assume cost is provided and validated

        # Проверяем баланс
        balance_key = f"balance:{user_id}"
        balance = Decimal(r.get(balance_key) or "0")

        # Check user balance and alert if low
        MONITORING.check_user_balance(user_id, balance)

        if balance < cost:
            raise BalanceError("Insufficient balance")

        # Списываем
        new_balance = balance - cost
        r.set(balance_key, str(new_balance))

        # Логируем транзакцию
        tx = {
            "user_id": user_id,
            "model": model,
            "tokens_used": tokens_used,
            "cost_usd": float(cost),
            "balance_usd": float(new_balance),
            "timestamp": int(time.time())
        }
        r.xadd("billing:log", tx)
        r.hincrby(f"usage:{user_id}:model:{model}", "direct", tokens_used)
        r.hincrby(f"usage:daily:{datetime.now():%Y-%m-%d}", model, tokens_used)

        # Log transaction for monitoring
        MONITORING.log_transaction("charge", cost, success=True)

        logger.info(f"Charged {cost:.5f} USD → {user_id} | {model} | {tokens_used} tokens")
        return billing_pb2.BillResponse(
            success=True,
            new_balance=float(new_balance)
        )

    @handle_billing_errors
    def Reserve(self, request, context):
        """Reserve funds for estimated usage"""
        # Authentication check
        metadata = context.invocation_metadata()
        auth_token = None
        for key, value in metadata:
            if key == "authorization":
                auth_token = value
                break

        if not auth_token:
            raise AuthenticationError("Missing authentication token")
        if not validate_jwt(auth_token):
            raise AuthenticationError("Invalid authentication token")

        # Input validation
        user_id = request.user_id or "anonymous"
        request_id = request.request_id or str(uuid.uuid4())
        model = request.model
        endpoint = request.endpoint
        input_tokens = request.input_tokens_estimate
        output_tokens = request.output_tokens_estimate

        # Validate inputs
        validate_user_id(user_id)
        validate_model_id(model)

        if input_tokens <= 0 or output_tokens < 0:
            raise ValidationError("Invalid token values")

        # Calculate estimated cost
        estimated_cost = self.calculate_cost(model, endpoint, input_tokens, output_tokens)
        if estimated_cost <= 0:
            raise PricingError("Invalid pricing calculation")

        # Check balance
        balance_key = f"balance:{user_id}"
        balance = Decimal(r.get(balance_key) or "0")

        # Check user balance and alert if low
        MONITORING.check_user_balance(user_id, balance)

        if balance < estimated_cost:
            raise BalanceError("Insufficient balance")

        # Create reservation
        reservation_id = f"res:{user_id}:{request_id}:{int(time.time())}"
        reservation_data = {
            "user_id": user_id,
            "model": model,
            "endpoint": endpoint,
            "input_tokens": input_tokens,
            "output_tokens": output_tokens,
            "estimated_cost": float(estimated_cost),
            "status": "reserved",
            "created_at": int(time.time())
        }

        # Store reservation (with TTL)
        reservation_key = f"reservation:{reservation_id}"
        try:
            r.hmset(reservation_key, reservation_data)
            r.expire(reservation_key, RESERVATION_TTL)
        except Exception as e:
            logger.error(f"Failed to save reservation: {e}")
            raise ReservationError("Failed to create reservation")

        # Deduct estimated amount from balance
        new_balance = balance - estimated_cost
        r.set(balance_key, str(new_balance))

        # Log transaction for monitoring
        MONITORING.log_transaction("reserve", estimated_cost, success=True)

        logger.info(f"Reserved {estimated_cost:.5f} USD → {user_id} | {reservation_id}")
        return billing_pb2.ReserveResponse(
            success=True,
            reservation_id=reservation_id,
            reserved_amount=float(estimated_cost),
            remaining_balance=float(new_balance)
        )

    @handle_billing_errors
    def Commit(self, request, context):
        """Commit actual usage against a reservation"""
        # Authentication check
        metadata = context.invocation_metadata()
        auth_token = None
        for key, value in metadata:
            if key == "authorization":
                auth_token = value
                break

        if not auth_token:
            raise AuthenticationError("Missing authentication token")
        if not validate_jwt(auth_token):
            raise AuthenticationError("Invalid authentication token")

        # Input validation
        reservation_id = request.reservation_id
        input_tokens_actual = request.input_tokens_actual
        output_tokens_actual = request.output_tokens_actual

        # Validate inputs
        validate_reservation_id(reservation_id)

        if input_tokens_actual <= 0 or output_tokens_actual < 0:
            raise ValidationError("Invalid token values")

        # Get reservation data
        reservation_key = f"reservation:{reservation_id}"
        reservation_data = r.hgetall(reservation_key)

        if not reservation_data:
            raise ReservationError("Reservation not found")

        # Check if already committed
        if reservation_data.get("status") == "committed":
            raise ReservationError("Already committed")

        user_id = reservation_data.get("user_id")
        model = reservation_data.get("model")
        endpoint = reservation_data.get("endpoint")
        estimated_cost = Decimal(reservation_data.get("estimated_cost", "0"))

        # Calculate actual cost
        actual_cost = self.calculate_cost(model, endpoint, input_tokens_actual, output_tokens_actual)

        # Get current balance
        balance_key = f"balance:{user_id}"
        balance = Decimal(r.get(balance_key) or "0")

        # Adjust balance: refund the difference between estimated and actual
        balance_adjustment = estimated_cost - actual_cost
        new_balance = balance + balance_adjustment
        r.set(balance_key, str(new_balance))

        # Update reservation status
        try:
            r.hset(reservation_key, "status", "committed")
            r.hset(reservation_key, "actual_cost", float(actual_cost))
            r.hset(reservation_key, "input_tokens_actual", input_tokens_actual)
            r.hset(reservation_key, "output_tokens_actual", output_tokens_actual)
            r.expire(reservation_key, 86400)  # Keep for 24h after commit
        except Exception as e:
            logger.error(f"Failed to update reservation: {e}")
            raise ReservationError("Failed to update reservation")

        # Log the transaction
        tx = {
            "user_id": user_id,
            "model": model,
            "endpoint": endpoint,
            "input_tokens": input_tokens_actual,
            "output_tokens": output_tokens_actual,
            "cost_usd": float(actual_cost),
            "balance_usd": float(new_balance),
            "reservation_id": reservation_id,
            "timestamp": int(time.time())
        }
        r.xadd("billing:log", tx)
        r.hincrby(f"usage:{user_id}:model:{model}", endpoint, input_tokens_actual + output_tokens_actual)
        r.hincrby(f"usage:daily:{datetime.now():%Y-%m-%d}", model, input_tokens_actual + output_tokens_actual)

        # Log transaction for monitoring
        MONITORING.log_transaction("commit", actual_cost, success=True)

        logger.info(f"Committed {actual_cost:.5f} USD → {user_id} | {reservation_id}")
        return billing_pb2.CommitResponse(
            success=True,
            final_cost=float(actual_cost),
            remaining_balance=float(new_balance)
        )

    @handle_billing_errors
    def calculate_cost(self, model: str, endpoint: str, input_t: int, output_t: int) -> Decimal:
        """Calculate cost using unified pricing system"""
        try:
            prices = PRICING_MANAGER.get_price(model, endpoint)

            if endpoint == "chat":
                input_cost = prices.get("input", Decimal("0.00001"))
                output_cost = prices.get("output", Decimal("0.00003"))
                total_cost = (Decimal(input_t) * input_cost + Decimal(output_t) * output_cost)
                return total_cost.quantize(Decimal('0.00001'), ROUND_HALF_UP)
            elif endpoint == "embed":
                embed_cost = prices.get("embed", Decimal("0.00001"))
                total_cost = (Decimal(input_t) * embed_cost)
                return total_cost.quantize(Decimal('0.00001'), ROUND_HALF_UP)
            else:
                logger.warning(f"Unknown endpoint type: {endpoint}")
                raise PricingError(f"Unknown endpoint type: {endpoint}")
        except (InvalidOperation, ValueError) as e:
            logger.error(f"Pricing calculation error: {e}")
            raise PricingError(f"Invalid pricing data: {e}")
        except Exception as e:
            logger.error(f"Unexpected pricing error: {e}")
            raise PricingError("Failed to calculate price")

    @handle_billing_errors
    def GetBalance(self, request, context):
        user_id = request.user_id or "anonymous"
        validate_user_id(user_id)

        balance = Decimal(r.get(f"balance:{user_id}") or "0")

        # Check user balance and alert if low
        MONITORING.check_user_balance(user_id, balance)

        try:
            return billing_pb2.GetBalanceResponse(
                balance_usd=float(balance),
                balance_rub=float(balance * EXCHANGE_MANAGER.get_rate("RUB")),
                balance_eur=float(balance * EXCHANGE_MANAGER.get_rate("EUR"))
            )
        except ValidationError as e:
            logger.error(f"Invalid currency in GetBalance: {e}")
            return billing_pb2.GetBalanceResponse(
                balance_usd=float(balance),
                balance_rub=0,
                balance_eur=0
            )

    @handle_billing_errors
    def AdjustBalance(self, request, context):
        user_id = request.user_id
        amount_usd = Decimal(str(request.amount_usd))
        reason = request.reason or "manual_adjustment"

        validate_user_id(user_id)
        validate_amount(amount_usd)

        key = f"balance:{user_id}"
        current = Decimal(r.get(key) or "0")
        new = current + amount_usd
        r.set(key, str(new))

        r.xadd("billing:adjustments", {
            "user_id": user_id,
            "amount_usd": float(amount_usd),
            "reason": reason,
            "timestamp": int(time.time())
        })

        # Log transaction for monitoring
        MONITORING.log_transaction("adjust", amount_usd, success=True)

        return billing_pb2.AdjustBalanceResponse(success=True, new_balance_usd=float(new))

# =============================================================================
# HTTP API (Stripe + Админка)
# =============================================================================
app = Flask(__name__)

# Rate limiting setup
limiter = Limiter(
    app,
    key_func=get_remote_address,
    default_limits=["200 per day", "50 per hour"]
)

# Admin endpoints rate limiting
admin_limiter = Limiter(
    key_func=get_remote_address,
    default_limits=["50 per hour", "10 per minute"]
)

def handle_http_errors(f):
    """Decorator for handling HTTP errors"""
    @wraps(f)
    def wrapper(*args, **kwargs):
        try:
            return f(*args, **kwargs)
        except AuthenticationError as e:
            logger.warning(f"Authentication error: {e}")
            return jsonify({"error": str(e), "code": e.code}), 401
        except ValidationError as e:
            logger.warning(f"Validation error: {e}")
            return jsonify({"error": str(e), "code": e.code}), 400
        except BalanceError as e:
            logger.warning(f"Balance error: {e}")
            return jsonify({"error": str(e), "code": e.code}), 400
        except PricingError as e:
            logger.error(f"Pricing error: {e}")
            return jsonify({"error": str(e), "code": e.code}), 400
        except ReservationError as e:
            logger.warning(f"Reservation error: {e}")
            return jsonify({"error": str(e), "code": e.code}), 400
        except ExternalServiceError as e:
            logger.error(f"External service error: {e}")
            return jsonify({"error": str(e), "code": e.code}), 500
        except Exception as e:
            logger.error(f"Unexpected error: {e}")
            return jsonify({"error": "Internal server error"}), 500
    return wrapper

@app.route("/create-checkout", methods=["POST"])
@limiter.limit("10 per minute")
@handle_http_errors
def create_checkout():
    # Input validation
    if not request.is_json:
        raise ValidationError("Invalid request format")

    data = request.json
    if not data or "user_id" not in data or "amount_usd" not in data:
        raise ValidationError("Missing required parameters")

    user_id = data["user_id"]
    amount_usd = data["amount_usd"]

    # Validate inputs
    validate_user_id(user_id)

    try:
        amount = Decimal(str(amount_usd))
        validate_amount(amount)
    except (InvalidOperation, ValueError):
        raise ValidationError("Invalid amount format")

    # Create Stripe session
    try:
        session = stripe.checkout.Session.create(
            payment_method_types=['card'],
            line_items=[{
                'price_data': {
                    'currency': 'usd',
                    'product_data': {'name': 'LLM Credits'},
                    'unit_amount': int(amount * 100),  # Convert to cents
                },
                'quantity': 1,
            }],
            mode='payment',
            success_url=os.getenv("DOMAIN") + "/dashboard?success=1",
            cancel_url=os.getenv("DOMAIN") + "/dashboard",
            metadata={"user_id": user_id}
        )
        return jsonify({"url": session.url})
    except stripe.error.StripeError as e:
        logger.error(f"Stripe error: {e}")
        raise ExternalServiceError("Payment processing error")

@app.route("/webhook", methods=["POST"])
@handle_http_errors
def stripe_webhook():
    payload = request.data
    sig = request.headers.get("Stripe-Signature")

    # Validate webhook signature
    try:
        event = stripe.Webhook.construct_event(payload, sig, STRIPE_WEBHOOK_SECRET)
    except ValueError as e:
        # Invalid payload
        logger.warning(f"Invalid webhook payload: {e}")
        raise ValidationError("Invalid webhook payload")
    except stripe.error.SignatureVerificationError as e:
        # Invalid signature
        logger.warning(f"Invalid webhook signature: {e}")
        raise AuthenticationError("Invalid webhook signature")

    # Handle the event
    if event.type == "checkout.session.completed":
        session = event.data.object
        user_id = session.metadata.get("user_id")

        # Validate user_id
        if not user_id:
            raise ValidationError("Missing user_id in webhook")
        validate_user_id(user_id)

        try:
            amount_usd = Decimal(session.amount_total) / 100

            # Update balance
            key = f"balance:{user_id}"
            current = Decimal(r.get(key) or "0")
            r.set(key, str(current + amount_usd))

            # Log deposit
            r.xadd("billing:deposits", {
                "user_id": user_id,
                "amount_usd": float(amount_usd),
                "source": "stripe",
                "timestamp": int(time.time())
            })
            logger.info(f"Top-up +{amount_usd} USD → {user_id}")
        except (InvalidOperation, ValueError) as e:
            logger.error(f"Error processing webhook amount: {e}")
            raise ValidationError("Invalid amount in webhook")

    return "ok", 200

@app.route("/admin/pricing", methods=["GET", "POST"])
@admin_limiter.limit("5 per minute")
@handle_http_errors
def admin_pricing():
    # Enhanced admin authentication
    auth_key = request.headers.get("X-Admin-Key")
    if not auth_key or auth_key != ADMIN_KEY:
        logger.warning(f"Unauthorized access attempt to admin/pricing from {request.remote_addr}")
        raise AuthenticationError("Invalid admin key")

    if request.method == "POST":
        # Validate input
        if not request.is_json:
            raise ValidationError("Invalid request format")

        new_pricing = request.json
        if not isinstance(new_pricing, dict):
            raise ValidationError("Invalid pricing format")

        # Validate pricing structure
        for model_id, prices in new_pricing.items():
            validate_model_id(model_id)
            if not isinstance(prices, dict):
                raise ValidationError(f"Invalid pricing for {model_id}")

        # Update pricing
        global PRICING
        PRICING = new_pricing
        try:
            r.set("pricing:current", json.dumps(PRICING))
        except Exception as e:
            logger.error(f"Failed to save pricing to Redis: {e}")
            raise PricingError("Failed to save pricing")

        logger.info(f"Pricing updated by {request.remote_addr}")
        return jsonify({"status": "saved"}), 200

    return jsonify(PRICING)

@app.route("/admin/pricing/update", methods=["POST"])
@admin_limiter.limit("2 per minute")
@handle_http_errors
def admin_update_pricing():
    """Update pricing from external source"""
    # Enhanced admin authentication
    auth_key = request.headers.get("X-Admin-Key")
    if not auth_key or auth_key != ADMIN_KEY:
        logger.warning(f"Unauthorized access attempt to admin/pricing/update from {request.remote_addr}")
        raise AuthenticationError("Invalid admin key")

    # Validate input
    if not request.is_json:
        raise ValidationError("Invalid request format")

    data = request.json
    if "source_url" not in data:
        raise ValidationError("Missing source_url parameter")

    source_url = data["source_url"]

    # Validate URL format
    if not source_url.startswith(("http://", "https://")):
        raise ValidationError("Invalid URL format")

    # Update pricing from external source
    try:
        success = PRICING_MANAGER.update_from_external_source(source_url)
        if success:
            return jsonify({
                "status": "success",
                "source": PRICING_MANAGER.source,
                "last_updated": PRICING_MANAGER.last_updated,
                "pricing": PRICING_MANAGER.pricing
            }), 200
        else:
            raise PricingError("Failed to update pricing from external source")
    except PricingError as e:
        raise
    except Exception as e:
        logger.error(f"Error updating pricing: {e}")
        raise ExternalServiceError("Error updating pricing")

@app.route("/admin/pricing/info", methods=["GET"])
@admin_limiter.limit("10 per minute")
@handle_http_errors
def admin_pricing_info():
    """Get pricing information"""
    # Enhanced admin authentication
    auth_key = request.headers.get("X-Admin-Key")
    if not auth_key or auth_key != ADMIN_KEY:
        logger.warning(f"Unauthorized access attempt to admin/pricing/info from {request.remote_addr}")
        raise AuthenticationError("Invalid admin key")

    try:
        pricing_info = PRICING_MANAGER.get_pricing_info()
        return jsonify(pricing_info), 200
    except PricingError as e:
        raise
    except Exception as e:
        logger.error(f"Error getting pricing info: {e}")
        raise ExternalServiceError("Error getting pricing info")

@app.route("/admin/stats")
@admin_limiter.limit("5 per minute")
@handle_http_errors
def admin_stats():
    # Enhanced admin authentication
    auth_key = request.headers.get("X-Admin-Key")
    if not auth_key or auth_key != ADMIN_KEY:
        logger.warning(f"Unauthorized access attempt to admin/stats from {request.remote_addr}")
        raise AuthenticationError("Invalid admin key")

    try:
        total_revenue = sum(float(x["cost_usd"]) for x in r.xrange("billing:log"))
        users = len(r.keys("balance:*"))
        today = datetime.now().strftime("%Y-%m-%d")
        today_usage = r.hgetall(f"usage:daily:{today}")

        return jsonify({
            "total_revenue_usd": round(total_revenue, 2),
            "active_users": users,
            "today_usage": {k: int(v) for k, v in today_usage.items()}
        })
    except Exception as e:
        logger.error(f"Error generating stats: {e}")
        raise ExternalServiceError("Error generating stats")

@app.route("/admin/exchange-rates", methods=["GET", "POST", "PUT", "DELETE"])
@admin_limiter.limit("10 per minute")
@handle_http_errors
def admin_exchange_rates():
    """Manage exchange rates"""
    # Enhanced admin authentication
    auth_key = request.headers.get("X-Admin-Key")
    if not auth_key or auth_key != ADMIN_KEY:
        logger.warning(f"Unauthorized access attempt to admin/exchange-rates from {request.remote_addr}")
        raise AuthenticationError("Invalid admin key")

    if request.method == "GET":
        # Get current exchange rates
        try:
            return jsonify({
                "rates": EXCHANGE_MANAGER.rates,
                "last_updated": EXCHANGE_MANAGER.last_updated,
                "supported_currencies": EXCHANGE_MANAGER.supported_currencies,
                "source": "automated"
            }), 200
        except Exception as e:
            logger.error(f"Error getting exchange rates: {e}")
            raise ExternalServiceError("Error getting exchange rates")

    elif request.method == "POST":
        # Add a new currency
        if not request.is_json:
            raise ValidationError("Invalid request format")

        data = request.json
        if "currency" not in data or "rate" not in data:
            raise ValidationError("Missing required parameters")

        currency = data["currency"]
        try:
            rate = Decimal(str(data["rate"]))
        except (InvalidOperation, ValueError):
            raise ValidationError("Invalid rate format")

        try:
            EXCHANGE_MANAGER.add_currency(currency, rate)
            return jsonify({
                "status": "success",
                "currency": currency,
                "rate": float(rate)
            }), 201
        except ValidationError as e:
            raise
        except Exception as e:
            logger.error(f"Error adding currency: {e}")
            raise ExternalServiceError("Error adding currency")

    elif request.method == "PUT":
        # Update an existing currency rate
        if not request.is_json:
            raise ValidationError("Invalid request format")

        data = request.json
        if "currency" not in data or "rate" not in data:
            raise ValidationError("Missing required parameters")

        currency = data["currency"]
        try:
            rate = Decimal(str(data["rate"]))
        except (InvalidOperation, ValueError):
            raise ValidationError("Invalid rate format")

        try:
            EXCHANGE_MANAGER.update_currency_rate(currency, rate)
            return jsonify({
                "status": "success",
                "currency": currency,
                "rate": float(rate)
            }), 200
        except ValidationError as e:
            raise
        except Exception as e:
            logger.error(f"Error updating currency rate: {e}")
            raise ExternalServiceError("Error updating currency rate")

    elif request.method == "DELETE":
        # Remove a currency
        if not request.is_json:
            raise ValidationError("Invalid request format")

        data = request.json
        if "currency" not in data:
            raise ValidationError("Missing currency parameter")

        currency = data["currency"]

        try:
            EXCHANGE_MANAGER.remove_currency(currency)
            return jsonify({
                "status": "success",
                "currency": currency
            }), 200
        except ValidationError as e:
            raise
        except Exception as e:
            logger.error(f"Error removing currency: {e}")
            raise ExternalServiceError("Error removing currency")

    else:
        raise ValidationError("Unsupported HTTP method")

@app.route("/admin/exchange-rates/update", methods=["POST"])
@admin_limiter.limit("2 per minute")
@handle_http_errors
def admin_update_exchange_rates():
    """Manually update exchange rates"""
    # Enhanced admin authentication
    auth_key = request.headers.get("X-Admin-Key")
    if not auth_key or auth_key != ADMIN_KEY:
        logger.warning(f"Unauthorized access attempt to admin/exchange-rates/update from {request.remote_addr}")
        raise AuthenticationError("Invalid admin key")

    try:
        success = EXCHANGE_MANAGER.fetch_exchange_rates()
        if success:
            return jsonify({
                "status": "success",
                "rates": EXCHANGE_MANAGER.rates,
                "last_updated": EXCHANGE_MANAGER.last_updated
            }), 200
        else:
            raise ExternalServiceError("Failed to update exchange rates")
    except ExternalServiceError as e:
        raise
    except Exception as e:
        logger.error(f"Error updating exchange rates: {e}")
        raise ExternalServiceError("Error updating exchange rates")

@app.route("/admin/exchange-rates/sources", methods=["GET"])
@admin_limiter.limit("10 per minute")
@handle_http_errors
def admin_exchange_rate_sources():
    """Get available exchange rate sources"""
    # Enhanced admin authentication
    auth_key = request.headers.get("X-Admin-Key")
    if not auth_key or auth_key != ADMIN_KEY:
        logger.warning(f"Unauthorized access attempt to admin/exchange-rates/sources from {request.remote_addr}")
        raise AuthenticationError("Invalid admin key")

    # List of available exchange rate APIs
    sources = [
        {
            "name": "exchangerate-api",
            "url": "https://api.exchangerate-api.com/v4/latest/USD",
            "description": "Free exchange rate API with hourly updates",
            "currencies": ["USD", "EUR", "RUB", "GBP", "CNY", "JPY", "INR"]
        },
        {
            "name": "openexchangerates",
            "url": "https://openexchangerates.org/api/latest.json",
            "description": "Paid exchange rate API with real-time updates",
            "currencies": ["USD", "EUR", "RUB", "GBP", "CNY", "JPY", "INR"]
        },
        {
            "name": "currencyapi",
            "url": "https://api.currencyapi.com/v3/latest",
            "description": "Paid exchange rate API with high accuracy",
            "currencies": ["USD", "EUR", "RUB", "GBP", "CNY", "JPY", "INR"]
        }
    ]

    return jsonify({
        "sources": sources,
        "current_source": EXCHANGE_MANAGER.api_url
    }), 200

@app.route("/admin/monitoring", methods=["GET"])
@admin_limiter.limit("10 per minute")
@handle_http_errors
def admin_monitoring():
    """Get monitoring metrics and alerts"""
    # Enhanced admin authentication
    auth_key = request.headers.get("X-Admin-Key")
    if not auth_key or auth_key != ADMIN_KEY:
        logger.warning(f"Unauthorized access attempt to admin/monitoring from {request.remote_addr}")
        raise AuthenticationError("Invalid admin key")

    try:
        # Get metrics from monitoring system
        metrics = MONITORING.get_metrics()

        # Get recent alerts
        alerts = []
        try:
            alert_stream = r.xrange("billing:alerts")
            for alert in alert_stream:
                alerts.append(alert)
                if len(alerts) >= 10:  # Limit to 10 most recent alerts
                    break
        except Exception as e:
            logger.error(f"Failed to get alerts: {e}")
            alerts = []

        # Get system health
        system_health = {
            "status": "healthy",
            "redis_connected": r.ping() == b"PONG",
            "last_exchange_update": EXCHANGE_MANAGER.last_updated,
            "last_pricing_update": PRICING_MANAGER.last_updated,
            "reservation_ttl": RESERVATION_TTL,
            "reservation_ttl_healthy": RESERVATION_TTL >= MONITORING.alert_thresholds["reservation_ttl"]
        }

        return jsonify({
            "metrics": metrics,
            "alerts": alerts,
            "system_health": system_health
        }), 200
    except Exception as e:
        logger.error(f"Error getting monitoring data: {e}")
        raise ExternalServiceError("Error getting monitoring data")

@app.route("/admin/monitoring/alerts", methods=["GET"])
@admin_limiter.limit("10 per minute")
@handle_http_errors
def admin_alerts():
    """Get recent alerts"""
    # Enhanced admin authentication
    auth_key = request.headers.get("X-Admin-Key")
    if not auth_key or auth_key != ADMIN_KEY:
        logger.warning(f"Unauthorized access attempt to admin/monitoring/alerts from {request.remote_addr}")
        raise AuthenticationError("Invalid admin key")

    try:
        alerts = []
        alert_stream = r.xrange("billing:alerts")
        for alert in alert_stream:
            alerts.append(alert)
            if len(alerts) >= 50:  # Limit to 50 most recent alerts
                break

        return jsonify({
            "alerts": alerts,
            "count": len(alerts)
        }), 200
    except Exception as e:
        logger.error(f"Error getting alerts: {e}")
        raise ExternalServiceError("Error getting alerts")

@app.route("/admin/monitoring/thresholds", methods=["GET", "POST"])
@admin_limiter.limit("5 per minute")
@handle_http_errors
def admin_monitoring_thresholds():
    """Manage monitoring thresholds"""
    # Enhanced admin authentication
    auth_key = request.headers.get("X-Admin-Key")
    if not auth_key or auth_key != ADMIN_KEY:
        logger.warning(f"Unauthorized access attempt to admin/monitoring/thresholds from {request.remote_addr}")
        raise AuthenticationError("Invalid admin key")

    if request.method == "GET":
        # Get current thresholds
        return jsonify(MONITORING.alert_thresholds), 200

    elif request.method == "POST":
        # Update thresholds
        if not request.is_json:
            raise ValidationError("Invalid request format")

        data = request.json
        if not isinstance(data, dict):
            raise ValidationError("Invalid thresholds format")

        try:
            with MONITORING.lock:
                for key, value in data.items():
                    if key in MONITORING.alert_thresholds:
                        if key == "low_balance":
                            MONITORING.alert_thresholds[key] = Decimal(str(value))
                        else:
                            MONITORING.alert_thresholds[key] = value
                    else:
                        logger.warning(f"Invalid threshold key: {key}")

                logger.info(f"Updated monitoring thresholds: {MONITORING.alert_thresholds}")
                return jsonify(MONITORING.alert_thresholds), 200
        except Exception as e:
            logger.error(f"Error updating thresholds: {e}")
            raise ExternalServiceError("Error updating thresholds")

# Monitoring and alerting
class MonitoringSystem:
    """Comprehensive monitoring and alerting system"""

    def __init__(self):
        self.alert_thresholds = {
            "low_balance": Decimal("10.00"),  # Alert when user balance < $10
            "high_usage": 1000000,  # Alert when token usage > 1M in 24h
            "error_rate": 0.05,  # Alert when error rate > 5%
            "reservation_ttl": 300,  # Alert when reservation TTL < 5 min
        }
        self.metrics = {
            "total_requests": 0,
            "successful_requests": 0,
            "failed_requests": 0,
            "total_charges": Decimal("0"),
            "total_reservations": 0,
            "total_commits": 0,
            "last_alert": 0,
            "alert_cooldown": 3600,  # 1 hour cooldown between alerts
        }
        self.lock = threading.Lock()

    def log_transaction(self, tx_type: str, amount: Decimal = None, success: bool = True):
        """Log a transaction for monitoring"""
        with self.lock:
            self.metrics["total_requests"] += 1
            if success:
                self.metrics["successful_requests"] += 1
            else:
                self.metrics["failed_requests"] += 1

            if tx_type == "charge" and amount:
                self.metrics["total_charges"] += amount
            elif tx_type == "reserve":
                self.metrics["total_reservations"] += 1
            elif tx_type == "commit":
                self.metrics["total_commits"] += 1

            # Check for alerts
            self.check_alerts()

    def check_alerts(self):
        """Check metrics and trigger alerts if needed"""
        with self.lock:
            now = time.time()
            if now - self.metrics["last_alert"] < self.alert_cooldown:
                return

            # Calculate error rate
            total = self.metrics["total_requests"]
            if total > 0:
                error_rate = self.metrics["failed_requests"] / total
                if error_rate > self.alert_thresholds["error_rate"]:
                    self.trigger_alert(f"High error rate: {error_rate:.2%}")
                    return

            # Check reservation TTL
            if RESERVATION_TTL < self.alert_thresholds["reservation_ttl"]:
                self.trigger_alert(f"Low reservation TTL: {RESERVATION_TTL} seconds")
                return

    def trigger_alert(self, message: str):
        """Trigger an alert (log and optionally send notification)"""
        with self.lock:
            self.metrics["last_alert"] = time.time()
            logger.warning(f"ALERT: {message}")

            # In production, this could send to monitoring system
            # or notification service (e.g., email, Slack, PagerDuty)
            try:
                alert_data = {
                    "message": message,
                    "timestamp": int(time.time()),
                    "metrics": self.metrics
                }
                r.xadd("billing:alerts", alert_data)
                logger.info("Alert logged to Redis")
            except Exception as e:
                logger.error(f"Failed to log alert: {e}")

    def get_metrics(self):
        """Get current metrics"""
        with self.lock:
            return {
                "metrics": self.metrics,
                "thresholds": self.alert_thresholds,
                "last_alert": datetime.fromtimestamp(self.metrics["last_alert"]).isoformat() if self.metrics["last_alert"] > 0 else None
            }

    def check_user_balance(self, user_id: str, balance: Decimal):
        """Check user balance and alert if low"""
        if balance < self.alert_thresholds["low_balance"]:
            self.trigger_alert(f"Low balance for user {user_id}: {balance:.2f} USD")

    def check_usage(self, user_id: str, tokens: int, period: str = "24h"):
        """Check token usage and alert if high"""
        if tokens > self.alert_thresholds["high_usage"]:
            self.trigger_alert(f"High usage for user {user_id}: {tokens} tokens in {period}")

# Initialize monitoring system
MONITORING = MonitoringSystem()

# =============================================================================
# Запуск
# =============================================================================
def serve():
    # Загружаем тарифы из Redis (если есть)
    try:
        saved = r.get("pricing:current")
        if saved:
            global PRICING
            PRICING = json.loads(saved)
    except Exception as e:
        logger.error(f"Failed to load pricing at startup: {e}")

    # gRPC
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    billing_pb2_grpc.add_BillingServiceServicer_to_server(BillingService(), server)
    server.add_insecure_port("[::]:50052")

    # Flask (Stripe + админка)
    import threading
    threading.Thread(target=app.run, kwargs={"host": "0.0.0.0", "port": 50053}, daemon=True).start()

    logger.info("Billing service: gRPC :50052 | HTTP :50053")
    server.start()
    server.wait_for_termination()

if __name__ == "__main__":
    serve()

