
// services/billing/admin_service.py
import os
import json
import logging
import time
from decimal import Decimal, InvalidOperation
from datetime import datetime
from concurrent import futures

import grpc
import redis
import jwt
from flask import Flask, jsonify, request, abort
from flask_limiter import Limiter
from flask_limiter.util import get_remote_address

import billing_pb2
import billing_pb2_grpc

# =============================================================================
# Admin Service
# =============================================================================
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("admin_service")

# Security configuration
JWT_SECRET = os.getenv("JWT_SECRET", "default-super-secret-key-2025")
ADMIN_KEY = os.getenv("ADMIN_KEY", "default-admin-key-2025")
STRIPE_WEBHOOK_SECRET = os.getenv("STRIPE_WEBHOOK_SECRET", "default-stripe-secret-2025")

# Initialize services
r = redis.from_url(os.getenv("REDIS_URL", "redis://redis:6379"))

# Flask app
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

# Admin Service
class AdminService(billing_pb2_grpc.AdminServiceServicer):

    def GetStats(self, request, context):
        """Get system statistics"""
        try:
            total_revenue = sum(float(x["cost_usd"]) for x in r.xrange("billing:log"))
            users = len(r.keys("balance:*"))
            today = datetime.now().strftime("%Y-%m-%d")
            today_usage = r.hgetall(f"usage:daily:{today}")

            return billing_pb2.StatsResponse(
                total_revenue_usd=round(total_revenue, 2),
                active_users=users,
                today_usage=json.dumps({k: int(v) for k, v in today_usage.items()})
            )
        except Exception as e:
            logger.error(f"Error generating stats: {e}")
            context.abort_with_status(grpc.StatusCode.INTERNAL, "Internal server error")

# =============================================================================
# HTTP API (Stripe + Админка)
# =============================================================================

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

# =============================================================================
# Запуск
# =============================================================================
def serve():
    # gRPC
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    billing_pb2_grpc.add_AdminServiceServicer_to_server(AdminService(), server)
    server.add_insecure_port("[::]:50056")

    # Flask (Stripe + админка)
    import threading
    threading.Thread(target=app.run, kwargs={"host": "0.0.0.0", "port": 50057}, daemon=True).start()

    logger.info("Admin Service: gRPC :50056 | HTTP :50057")
    server.start()
    server.wait_for_termination()

if __name__ == "__main__":
    serve()





