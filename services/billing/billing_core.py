
// services/billing/billing_core.py
import os
import json
import logging
import time
import uuid
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
# Billing Core Service
# =============================================================================
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("billing_core")

# Security configuration
JWT_SECRET = os.getenv("JWT_SECRET", "default-super-secret-key-2025")
ADMIN_KEY = os.getenv("ADMIN_KEY", "default-admin-key-2025")

# Initialize services
r = redis.from_url(os.getenv("REDIS_URL", "redis://redis:6379"))

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

# Billing Core Service
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
            # Get pricing from Pricing Service
            pricing_response = pricing_stub.GetPricing(billing_pb2.PricingRequest(model=model, endpoint=endpoint))
            prices = pricing_response.prices

            if endpoint == "chat":
                input_cost = Decimal(str(prices.chat_input)) / 1_000_000
                output_cost = Decimal(str(prices.chat_output)) / 1_000_000
                total_cost = (Decimal(input_t) * input_cost + Decimal(output_t) * output_cost)
                return total_cost.quantize(Decimal('0.00001'), ROUND_HALF_UP)
            elif endpoint == "embed":
                embed_cost = Decimal(str(prices.embed)) / 1_000_000
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

        # Get exchange rates from Exchange Rate Service
        exchange_response = exchange_stub.GetExchangeRates(billing_pb2.ExchangeRequest())
        rates = exchange_response.rates

        try:
            return billing_pb2.GetBalanceResponse(
                balance_usd=float(balance),
                balance_rub=float(balance * Decimal(str(rates["RUB"]))),
                balance_eur=float(balance * Decimal(str(rates["EUR"])))
            )
        except (KeyError, InvalidOperation, ValueError) as e:
            logger.error(f"Exchange rate error: {e}")
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

        return billing_pb2.AdjustBalanceResponse(success=True, new_balance_usd=float(new))

# =============================================================================
# Запуск
# =============================================================================
def serve():
    # gRPC
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    billing_pb2_grpc.add_BillingServiceServicer_to_server(BillingService(), server)
    server.add_insecure_port("[::]:50052")

    logger.info("Billing Core Service: gRPC :50052")
    server.start()
    server.wait_for_termination()

if __name__ == "__main__":
    serve()




