
// services/billing/server.py
import os
import json
import logging
import time
import uuid
from decimal import Decimal, ROUND_HALF_UP
from datetime import datetime, timedelta
from concurrent import futures

import grpc
import redis
import stripe
from flask import Flask, request, jsonify, render_template_string

import billing_pb2
import billing_pb2_grpc

# =============================================================================
# Конфигурация
# =============================================================================
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("billing")

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
PRICING = {
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

# =============================================================================
# gRPC сервис
# =============================================================================
class BillingService(billing_pb2_grpc.BillingServiceServicer):

    def Charge(self, request, context):
        """Direct usage recording without reservation"""
        user_id = request.user_id or "anonymous"
        model = request.model
        tokens_used = request.tokens_used
        cost = Decimal(str(request.cost))

        # Check if cost was provided, otherwise calculate it
        if cost <= 0:
            # Try to calculate cost based on tokens (if we have enough info)
            # For now, return error if cost is not provided
            return billing_pb2.BillResponse(
                success=False,
                error="invalid_cost",
                new_balance=0
            )

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
        user_id = request.user_id or "anonymous"
        request_id = request.request_id or str(uuid.uuid4())
        model = request.model
        endpoint = request.endpoint
        input_tokens = request.input_tokens_estimate
        output_tokens = request.output_tokens_estimate

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
        reservation_id = request.reservation_id
        input_tokens_actual = request.input_tokens_actual
        output_tokens_actual = request.output_tokens_actual

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
        prices = PRICING.get(model, {})
        if endpoint == "chat":
            input_cost = Decimal(prices.get("chat_input", 10)) / 1_000_000
            output_cost = Decimal(prices.get("chat_output", 30)) / 1_000_000
            return (Decimal(input_t) * input_cost + Decimal(output_t) * output_cost).quantize(Decimal('0.00001'), ROUND_HALF_UP)
        elif endpoint == "embed":
            cost_per_m = Decimal(prices.get("embed", 0.13))
            return (Decimal(input_t) * cost_per_m / 1_000_000).quantize(Decimal('0.00001'), ROUND_HALF_UP)
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

@app.route("/create-checkout", methods=["POST"])
def create_checkout():
    data = request.json
    user_id = data["user_id"]
    amount_usd = data["amount_usd"]

    session = stripe.checkout.Session.create(
        payment_method_types=['card'],
        line_items=[{
            'price_data': {
                'currency': 'usd',
                'product_data': {'name': 'LLM Credits'},
                'unit_amount': int(Decimal(str(amount_usd)) * 100),
            },
            'quantity': 1,
        }],
        mode='payment',
        success_url=os.getenv("DOMAIN") + "/dashboard?success=1",
        cancel_url=os.getenv("DOMAIN") + "/dashboard",
        metadata={"user_id": user_id}
    )
    return jsonify({"url": session.url})

@app.route("/webhook", methods=["POST"])
def stripe_webhook():
    payload = request.data
    sig = request.headers.get("Stripe-Signature")

    try:
        event = stripe.Webhook.construct_event(payload, sig, os.getenv("STRIPE_WEBHOOK_SECRET"))
    except:
        return "invalid", 400

    if event.type == "checkout.session.completed":
        session = event.data.object
        user_id = session.metadata.user_id
        amount_usd = Decimal(session.amount_total) / 100

        key = f"balance:{user_id}"
        current = Decimal(r.get(key) or "0")
        r.set(key, str(current + amount_usd))

        r.xadd("billing:deposits", {
            "user_id": user_id,
            "amount_usd": float(amount_usd),
            "source": "stripe",
            "timestamp": int(time.time())
        })
        logger.info(f"Top-up +{amount_usd} USD → {user_id}")

    return "ok", 200

@app.route("/admin/pricing", methods=["GET", "POST"])
def admin_pricing():
    if request.headers.get("X-Admin-Key") != os.getenv("ADMIN_KEY"):
        return "forbidden", 403

    if request.method == "POST":
        global PRICING
        PRICING = request.json
        r.set("pricing:current", json.dumps(PRICING))
        return "saved", 200

    return jsonify(PRICING)

@app.route("/admin/stats")
def admin_stats():
    if request.headers.get("X-Admin-Key") != os.getenv("ADMIN_KEY"):
        return "forbidden", 403

    total_revenue = sum(float(x["cost_usd"]) for x in r.xrange("billing:log"))
    users = len(r.keys("balance:*"))
    today = datetime.now().strftime("%Y-%m-%d")
    today_usage = r.hgetall(f"usage:daily:{today}")

    return jsonify({
        "total_revenue_usd": round(total_revenue, 2),
        "active_users": users,
        "today_usage": {k: int(v) for k, v in today_usage.items()}
    })

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

