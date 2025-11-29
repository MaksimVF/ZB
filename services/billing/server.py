
// services/billing/server.py
import os
import json
import logging
import time
import uuid
import re
from decimal import Decimal, ROUND_HALF_UP, InvalidOperation
from datetime import datetime, timedelta
from concurrent import futures

import grpc
import redis
import stripe
import jwt
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

# Security helpers
def validate_jwt(token: str) -> bool:
    """Validate JWT token"""
    try:
        if not token:
            return False
        decoded = jwt.decode(token, JWT_SECRET, algorithms=["HS256"])
        return True
    except (jwt.ExpiredSignatureError, jwt.InvalidTokenError) as e:
        logger.warning(f"Invalid JWT: {e}")
        return False

def validate_user_id(user_id: str) -> bool:
    """Validate user ID format"""
    return bool(USER_ID_PATTERN.match(user_id))

def validate_model_id(model_id: str) -> bool:
    """Validate model ID format"""
    return bool(MODEL_ID_PATTERN.match(model_id))

def validate_reservation_id(reservation_id: str) -> bool:
    """Validate reservation ID format"""
    return bool(RESERVATION_ID_PATTERN.match(reservation_id))

def validate_amount(amount: Decimal) -> bool:
    """Validate monetary amount"""
    return amount > 0 and amount < 1000000  # Reasonable limits

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
            except json.JSONDecodeError:
                logger.error("Failed to load pricing from Redis - invalid JSON")

    def save_to_redis(self):
        """Save current pricing to Redis"""
        try:
            r.set("pricing:current", json.dumps(self.pricing))
            logger.info("Pricing saved to Redis")
        except Exception as e:
            logger.error(f"Failed to save pricing to Redis: {e}")

    def update_from_external_source(self, source_url: str):
        """Update pricing from external API"""
        try:
            # This would be an actual API call in production
            # For now, we'll simulate with a placeholder
            logger.info(f"Updating pricing from external source: {source_url}")
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
            self.pricing = external_pricing
            self.source = f"external:{source_url}"
            self.last_updated = time.time()
            self.save_to_redis()
            return True
        except Exception as e:
            logger.error(f"Failed to update pricing from external source: {e}")
            return False

    def get_price(self, model: str, endpoint: str) -> Decimal:
        """Get price for a specific model and endpoint"""
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
        return {"default": Decimal("0.00001")}

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
PRICING_MANAGER.load_from_redis()

# =============================================================================
# gRPC сервис
# =============================================================================
class BillingService(billing_pb2_grpc.BillingServiceServicer):

    def Charge(self, request, context):
        """Direct usage recording without reservation"""
        # Authentication check
        metadata = context.invocation_metadata()
        auth_token = None
        for key, value in metadata:
            if key == "authorization":
                auth_token = value
                break

        if not auth_token or not validate_jwt(auth_token):
            context.abort_with_status(grpc.StatusCode.UNAUTHENTICATED, "Invalid or missing authentication token")

        # Input validation
        user_id = request.user_id or "anonymous"
        model = request.model
        tokens_used = request.tokens_used
        cost = Decimal(str(request.cost))

        # Validate inputs
        if not validate_user_id(user_id):
            context.abort_with_status(grpc.StatusCode.INVALID_ARGUMENT, "Invalid user_id format")

        if not validate_model_id(model):
            context.abort_with_status(grpc.StatusCode.INVALID_ARGUMENT, "Invalid model_id format")

        if tokens_used <= 0:
            context.abort_with_status(grpc.StatusCode.INVALID_ARGUMENT, "Invalid tokens_used value")

        if cost <= 0:
            context.abort_with_status(grpc.StatusCode.INVALID_ARGUMENT, "Invalid cost value")

        # Check if cost was provided, otherwise calculate it
        # For now, we assume cost is provided and validated

        # Проверяем баланс
        balance_key = f"balance:{user_id}"
        balance = Decimal(r.get(balance_key) or "0")

        if balance < cost:
            return billing_pb2.BillResponse(
                success=False,
                error="insufficient_balance",
                new_balance=float(balance)
            )

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

    def Reserve(self, request, context):
        """Reserve funds for estimated usage"""
        # Authentication check
        metadata = context.invocation_metadata()
        auth_token = None
        for key, value in metadata:
            if key == "authorization":
                auth_token = value
                break

        if not auth_token or not validate_jwt(auth_token):
            context.abort_with_status(grpc.StatusCode.UNAUTHENTICATED, "Invalid or missing authentication token")

        # Input validation
        user_id = request.user_id or "anonymous"
        request_id = request.request_id or str(uuid.uuid4())
        model = request.model
        endpoint = request.endpoint
        input_tokens = request.input_tokens_estimate
        output_tokens = request.output_tokens_estimate

        # Validate inputs
        if not validate_user_id(user_id):
            context.abort_with_status(grpc.StatusCode.INVALID_ARGUMENT, "Invalid user_id format")

        if not validate_model_id(model):
            context.abort_with_status(grpc.StatusCode.INVALID_ARGUMENT, "Invalid model_id format")

        if input_tokens <= 0 or output_tokens < 0:
            context.abort_with_status(grpc.StatusCode.INVALID_ARGUMENT, "Invalid token values")

        # Calculate estimated cost
        estimated_cost = self.calculate_cost(model, endpoint, input_tokens, output_tokens)
        if estimated_cost <= 0:
            return billing_pb2.ReserveResponse(
                success=False,
                error="invalid_estimate",
                reserved_amount=0,
                remaining_balance=0
            )

        # Check balance
        balance_key = f"balance:{user_id}"
        balance = Decimal(r.get(balance_key) or "0")

        if balance < estimated_cost:
            return billing_pb2.ReserveResponse(
                success=False,
                error="insufficient_balance",
                reserved_amount=0,
                remaining_balance=float(balance)
            )

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
        r.hmset(reservation_key, reservation_data)
        r.expire(reservation_key, RESERVATION_TTL)

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

    def Commit(self, request, context):
        """Commit actual usage against a reservation"""
        # Authentication check
        metadata = context.invocation_metadata()
        auth_token = None
        for key, value in metadata:
            if key == "authorization":
                auth_token = value
                break

        if not auth_token or not validate_jwt(auth_token):
            context.abort_with_status(grpc.StatusCode.UNAUTHENTICATED, "Invalid or missing authentication token")

        # Input validation
        reservation_id = request.reservation_id
        input_tokens_actual = request.input_tokens_actual
        output_tokens_actual = request.output_tokens_actual

        # Validate inputs
        if not validate_reservation_id(reservation_id):
            context.abort_with_status(grpc.StatusCode.INVALID_ARGUMENT, "Invalid reservation_id format")

        if input_tokens_actual <= 0 or output_tokens_actual < 0:
            context.abort_with_status(grpc.StatusCode.INVALID_ARGUMENT, "Invalid token values")

        # Get reservation data
        reservation_key = f"reservation:{reservation_id}"
        reservation_data = r.hgetall(reservation_key)

        if not reservation_data:
            return billing_pb2.CommitResponse(
                success=False,
                error="reservation_not_found",
                final_cost=0,
                remaining_balance=0
            )

        # Check if already committed
        if reservation_data.get("status") == "committed":
            return billing_pb2.CommitResponse(
                success=False,
                error="already_committed",
                final_cost=0,
                remaining_balance=0
            )

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
        r.hset(reservation_key, "status", "committed")
        r.hset(reservation_key, "actual_cost", float(actual_cost))
        r.hset(reservation_key, "input_tokens_actual", input_tokens_actual)
        r.hset(reservation_key, "output_tokens_actual", output_tokens_actual)
        r.expire(reservation_key, 86400)  # Keep for 24h after commit

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
                return Decimal("0")
        except Exception as e:
            logger.error(f"Error calculating cost: {e}")
            return Decimal("0")

    def GetBalance(self, request, context):
        user_id = request.user_id or "anonymous"
        balance = Decimal(r.get(f"balance:{user_id}") or "0")
        return billing_pb2.GetBalanceResponse(
            balance_usd=float(balance),
            balance_rub=float(balance * EXCHANGE_RATES["RUB"]),
            balance_eur=float(balance * EXCHANGE_RATES["EUR"])
        )

    def AdjustBalance(self, request, context):
        user_id = request.user_id
        amount_usd = Decimal(str(request.amount_usd))
        reason = request.reason or "manual_adjustment"

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

@app.route("/create-checkout", methods=["POST"])
@limiter.limit("10 per minute")
def create_checkout():
    # Input validation
    if not request.is_json:
        return jsonify({"error": "Invalid request format"}), 400

    data = request.json
    if not data or "user_id" not in data or "amount_usd" not in data:
        return jsonify({"error": "Missing required parameters"}), 400

    user_id = data["user_id"]
    amount_usd = data["amount_usd"]

    # Validate inputs
    if not validate_user_id(user_id):
        return jsonify({"error": "Invalid user_id format"}), 400

    try:
        amount = Decimal(str(amount_usd))
        if not validate_amount(amount):
            return jsonify({"error": "Invalid amount"}), 400
    except (InvalidOperation, ValueError):
        return jsonify({"error": "Invalid amount format"}), 400

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
        return jsonify({"error": "Payment processing error"}), 500

@app.route("/webhook", methods=["POST"])
def stripe_webhook():
    payload = request.data
    sig = request.headers.get("Stripe-Signature")

    # Validate webhook signature
    try:
        event = stripe.Webhook.construct_event(payload, sig, STRIPE_WEBHOOK_SECRET)
    except ValueError as e:
        # Invalid payload
        logger.warning(f"Invalid webhook payload: {e}")
        return "invalid payload", 400
    except stripe.error.SignatureVerificationError as e:
        # Invalid signature
        logger.warning(f"Invalid webhook signature: {e}")
        return "invalid signature", 400

    # Handle the event
    if event.type == "checkout.session.completed":
        session = event.data.object
        user_id = session.metadata.get("user_id")

        # Validate user_id
        if not user_id or not validate_user_id(user_id):
            logger.warning(f"Invalid user_id in webhook: {user_id}")
            return "invalid user", 400

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
            return "invalid amount", 400

    return "ok", 200

@app.route("/admin/pricing", methods=["GET", "POST"])
@admin_limiter.limit("5 per minute")
def admin_pricing():
    # Enhanced admin authentication
    auth_key = request.headers.get("X-Admin-Key")
    if not auth_key or auth_key != ADMIN_KEY:
        logger.warning(f"Unauthorized access attempt to admin/pricing from {request.remote_addr}")
        return jsonify({"error": "forbidden"}), 403

    if request.method == "POST":
        # Validate input
        if not request.is_json:
            return jsonify({"error": "Invalid request format"}), 400

        new_pricing = request.json
        if not isinstance(new_pricing, dict):
            return jsonify({"error": "Invalid pricing format"}), 400

        # Validate pricing structure
        for model_id, prices in new_pricing.items():
            if not validate_model_id(model_id):
                return jsonify({"error": f"Invalid model_id: {model_id}"}), 400
            if not isinstance(prices, dict):
                return jsonify({"error": f"Invalid pricing for {model_id}"}), 400

        # Update pricing
        global PRICING
        PRICING = new_pricing
        r.set("pricing:current", json.dumps(PRICING))
        logger.info(f"Pricing updated by {request.remote_addr}")
        return jsonify({"status": "saved"}), 200

    return jsonify(PRICING)

@app.route("/admin/stats")
@admin_limiter.limit("5 per minute")
def admin_stats():
    # Enhanced admin authentication
    auth_key = request.headers.get("X-Admin-Key")
    if not auth_key or auth_key != ADMIN_KEY:
        logger.warning(f"Unauthorized access attempt to admin/stats from {request.remote_addr}")
        return jsonify({"error": "forbidden"}), 403

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
        return jsonify({"error": "Error generating stats"}), 500

@app.route("/admin/pricing/update", methods=["POST"])
@admin_limiter.limit("2 per minute")
def admin_update_pricing():
    """Update pricing from external source"""
    # Enhanced admin authentication
    auth_key = request.headers.get("X-Admin-Key")
    if not auth_key or auth_key != ADMIN_KEY:
        logger.warning(f"Unauthorized access attempt to admin/pricing/update from {request.remote_addr}")
        return jsonify({"error": "forbidden"}), 403

    # Validate input
    if not request.is_json:
        return jsonify({"error": "Invalid request format"}), 400

    data = request.json
    if "source_url" not in data:
        return jsonify({"error": "Missing source_url parameter"}), 400

    source_url = data["source_url"]

    # Validate URL format
    if not source_url.startswith(("http://", "https://")):
        return jsonify({"error": "Invalid URL format"}), 400

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
            return jsonify({"error": "Failed to update pricing"}), 500
    except Exception as e:
        logger.error(f"Error updating pricing: {e}")
        return jsonify({"error": "Error updating pricing"}), 500

@app.route("/admin/pricing/info", methods=["GET"])
@admin_limiter.limit("10 per minute")
def admin_pricing_info():
    """Get pricing information"""
    # Enhanced admin authentication
    auth_key = request.headers.get("X-Admin-Key")
    if not auth_key or auth_key != ADMIN_KEY:
        logger.warning(f"Unauthorized access attempt to admin/pricing/info from {request.remote_addr}")
        return jsonify({"error": "forbidden"}), 403

    try:
        pricing_info = PRICING_MANAGER.get_pricing_info()
        return jsonify(pricing_info), 200
    except Exception as e:
        logger.error(f"Error getting pricing info: {e}")
        return jsonify({"error": "Error getting pricing info"}), 500

# =============================================================================
# Запуск
# =============================================================================
def serve():
    # Загружаем тарифы из Redis (если есть)
    saved = r.get("pricing:current")
    if saved:
        global PRICING
        PRICING = json.loads(saved)

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

