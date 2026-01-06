

# services/billing/billing_core_optimized.py
import os
import json
import logging
import time
import uuid
import re
from decimal import Decimal, ROUND_HALF_UP, InvalidOperation
from datetime import datetime, timedelta
from concurrent import futures
from functools import wraps, lru_cache

import grpc
import jwt

import billing_pb2
import billing_pb2_grpc
from redis_manager import redis_manager, redis_cache, redis_transaction

# =============================================================================
# Optimized Billing Core Service
# =============================================================================
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("billing_core_optimized")

# Security configuration
JWT_SECRET = os.getenv("JWT_SECRET", "default-super-secret-key-2025")
ADMIN_KEY = os.getenv("ADMIN_KEY", "default-admin-key-2025")

# Constants
RESERVATION_TTL = 600  # 10 minutes
BALANCE_CACHE_TTL = 300  # 5 minutes
PRICING_CACHE_TTL = 3600  # 1 hour
EXCHANGE_RATE_CACHE_TTL = 3600  # 1 hour

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

# Caching decorators
@redis_cache("pricing", PRICING_CACHE_TTL)
def get_pricing_data(model: str, endpoint: str) -> dict:
    """Get pricing data with caching"""
    # This would normally fetch from a pricing service or database
    # For now, return default pricing structure
    default_pricing = {
        "chat_input": "5.00",
        "chat_output": "15.00",
        "embed": "0.10"
    }
    return default_pricing

@redis_cache("exchange_rates", EXCHANGE_RATE_CACHE_TTL)
def get_exchange_rates() -> dict:
    """Get exchange rates with caching"""
    # This would normally fetch from an external service
    # For now, return default rates
    return {
        "USD": "1",
        "EUR": "0.92",
        "RUB": "96.50",
        "USDT": "1"
    }

# Optimized Billing Service
class OptimizedBillingService(billing_pb2_grpc.BillingServiceServicer):

    @handle_billing_errors
    def Charge(self, request, context):
        """Direct usage recording without reservation with optimized Redis operations"""
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

        # Use pipeline for batch operations
        try:
            with redis_manager.pipeline() as pipe:
                # Check balance
                balance_key = f"balance:{user_id}"
                pipe.get(balance_key)

                # Prepare transaction data
                new_balance = None
                tx = {
                    "user_id": user_id,
                    "model": model,
                    "tokens_used": tokens_used,
                    "cost_usd": float(cost),
                    "timestamp": int(time.time())
                }

                # Log usage
                pipe.xadd("billing:log", tx)
                pipe.hincrby(f"usage:{user_id}:model:{model}", "direct", tokens_used)
                pipe.hincrby(f"usage:daily:{datetime.now():%Y-%m-%d}", model, tokens_used)

                # Execute pipeline to get current balance
                results = pipe.execute()
                current_balance = Decimal(results[0] or "0")

                if current_balance < cost:
                    raise BalanceError("Insufficient balance")

                # Update balance
                new_balance = current_balance - cost
                redis_manager.client.set(balance_key, str(new_balance))

                tx["balance_usd"] = float(new_balance)

                logger.info(f"Charged {cost:.5f} USD → {user_id} | {model} | {tokens_used} tokens")
                return billing_pb2.BillResponse(
                    success=True,
                    new_balance=float(new_balance)
                )

        except Exception as e:
            logger.error(f"Charge operation failed: {e}")
            raise

    @handle_billing_errors
    def Reserve(self, request, context):
        """Reserve funds for estimated usage with optimized Redis operations"""
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

        # Use transaction for atomic operations
        try:
            with redis_manager.transaction() as transaction:
                # Check balance
                balance_key = f"balance:{user_id}"
                transaction.get(balance_key)

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
                transaction.hmset(reservation_key, reservation_data)
                transaction.expire(reservation_key, RESERVATION_TTL)

                # Deduct estimated amount from balance
                results = transaction.execute()
                balance = Decimal(results[0] or "0")

                if balance < estimated_cost:
                    raise BalanceError("Insufficient balance")

                new_balance = balance - estimated_cost
                redis_manager.client.set(balance_key, str(new_balance))

                logger.info(f"Reserved {estimated_cost:.5f} USD → {user_id} | {reservation_id}")
                return billing_pb2.ReserveResponse(
                    success=True,
                    reservation_id=reservation_id,
                    reserved_amount=float(estimated_cost),
                    remaining_balance=float(new_balance)
                )

        except Exception as e:
            logger.error(f"Reservation operation failed: {e}")
            raise

    @handle_billing_errors
    def Commit(self, request, context):
        """Commit actual usage against a reservation with optimized Redis operations"""
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

        # Use pipeline for batch operations
        try:
            with redis_manager.pipeline() as pipe:
                # Get reservation data
                reservation_key = f"reservation:{reservation_id}"
                pipe.hgetall(reservation_key)

                # Execute pipeline to get reservation data
                results = pipe.execute()
                reservation_data = results[0]

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
                current_balance = Decimal(redis_manager.client.get(balance_key) or "0")

                # Adjust balance: refund the difference between estimated and actual
                balance_adjustment = estimated_cost - actual_cost
                new_balance = current_balance + balance_adjustment
                redis_manager.client.set(balance_key, str(new_balance))

                # Update reservation status
                update_data = {
                    "status": "committed",
                    "actual_cost": float(actual_cost),
                    "input_tokens_actual": input_tokens_actual,
                    "output_tokens_actual": output_tokens_actual
                }
                redis_manager.client.hmset(reservation_key, update_data)
                redis_manager.client.expire(reservation_key, 86400)  # Keep for 24h after commit

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
                redis_manager.client.xadd("billing:log", tx)
                redis_manager.client.hincrby(f"usage:{user_id}:model:{model}", endpoint, input_tokens_actual + output_tokens_actual)
                redis_manager.client.hincrby(f"usage:daily:{datetime.now():%Y-%m-%d}", model, input_tokens_actual + output_tokens_actual)

                logger.info(f"Committed {actual_cost:.5f} USD → {user_id} | {reservation_id}")
                return billing_pb2.CommitResponse(
                    success=True,
                    final_cost=float(actual_cost),
                    remaining_balance=float(new_balance)
                )

        except Exception as e:
            logger.error(f"Commit operation failed: {e}")
            raise

    @handle_billing_errors
    @redis_cache("cost_calculation", 300)
    def calculate_cost(self, model: str, endpoint: str, input_t: int, output_t: int) -> Decimal:
        """Calculate cost using unified pricing system with caching"""
        try:
            # Get pricing from cache
            pricing_data = get_pricing_data(model, endpoint)

            if endpoint == "chat":
                input_cost = Decimal(str(pricing_data.get("chat_input", "5.00"))) / 1_000_000
                output_cost = Decimal(str(pricing_data.get("chat_output", "15.00"))) / 1_000_000
                total_cost = (Decimal(input_t) * input_cost + Decimal(output_t) * output_cost)
                return total_cost.quantize(Decimal('0.00001'), ROUND_HALF_UP)
            elif endpoint == "embed":
                embed_cost = Decimal(str(pricing_data.get("embed", "0.10"))) / 1_000_000
                total_cost = (Decimal(input_t) * embed_cost)
                return total_cost.quantize(Decimal('0.00001'), ROUND_HALF_UP)
            else:
                # Default pricing
                input_cost = Decimal("5.00") / 1_000_000
                output_cost = Decimal("15.00") / 1_000_000
                total_cost = (Decimal(input_t) * input_cost + Decimal(output_t) * output_cost)
                return total_cost.quantize(Decimal('0.00001'), ROUND_HALF_UP)
        except Exception as e:
            logger.error(f"Cost calculation error: {e}")
            raise PricingError("Failed to calculate cost")

    @handle_billing_errors
    def GetBalance(self, request, context):
        """Get user balance with caching"""
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
        user_id = request.user_id
        validate_user_id(user_id)

        # Get balance with caching
        balance_key = f"balance:{user_id}"
        balance = redis_manager.cache_get(balance_key)
        if balance is None:
            balance = "0"

        # Get exchange rates
        rates = get_exchange_rates()

        return billing_pb2.BalanceResponse(
            success=True,
            balance_usd=float(balance),
            balance_rub=float(Decimal(balance) * Decimal(rates.get("RUB", "96.50"))),
            balance_eur=float(Decimal(balance) * Decimal(rates.get("EUR", "0.92")))
        )

    @handle_billing_errors
    def AdjustBalance(self, request, context):
        """Adjust user balance with transaction support"""
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
        user_id = request.user_id
        amount = Decimal(str(request.amount))
        validate_user_id(user_id)
        validate_amount(amount)

        # Use transaction for atomic balance update
        try:
            with redis_manager.transaction() as transaction:
                balance_key = f"balance:{user_id}"
                transaction.get(balance_key)
                results = transaction.execute()
                current_balance = Decimal(results[0] or "0")

                new_balance = current_balance + amount
                redis_manager.client.set(balance_key, str(new_balance))

                logger.info(f"Adjusted balance for {user_id}: {current_balance} → {new_balance}")
                return billing_pb2.AdjustBalanceResponse(
                    success=True,
                    new_balance_usd=float(new_balance)
                )

        except Exception as e:
            logger.error(f"Balance adjustment failed: {e}")
            raise

# Initialize the optimized billing service
optimized_billing_service = OptimizedBillingService()

def serve():
    """Start the optimized billing gRPC server"""
    server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=10),
        options=[
            ('grpc.max_send_message_length', 50 * 1024 * 1024),
            ('grpc.max_receive_message_length', 50 * 1024 * 1024),
        ]
    )
    billing_pb2_grpc.add_BillingServiceServicer_to_server(optimized_billing_service, server)
    server.add_insecure_port('[::]:50052')
    logger.info("Optimized Billing Service started on port 50052")
    server.start()
    server.wait_for_termination()

if __name__ == '__main__':
    serve()

