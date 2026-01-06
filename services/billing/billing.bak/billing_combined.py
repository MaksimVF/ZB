#!/usr/bin/env python3
"""
Объединенный billing сервис
Содержит весь функционал из admin_service.py и server.py
"""

import os
import sys
import logging
import asyncio
import json
import time
import traceback
import jwt
import redis
import grpc
from datetime import datetime, timedelta
from decimal import Decimal, InvalidOperation
from typing import Dict, Any, Optional, List, Tuple
from flask import Flask, request, jsonify, Blueprint
from prometheus_client import Counter, Histogram, Gauge, start_http_server

# Импорт proto файлов
try:
    import billing_pb2
    import billing_pb2_grpc
except ImportError:
    print("⚠️  billing_pb2 modules not found. Run generate_proto.sh first")

# Константы
JWT_SECRET = os.getenv('JWT_SECRET', 'your-secret-key')
REDIS_URL = os.getenv('REDIS_URL', 'redis://localhost:6379')
STRIPE_SECRET_KEY = os.getenv('STRIPE_SECRET_KEY', '')
STRIPE_WEBHOOK_SECRET = os.getenv('STRIPE_WEBHOOK_SECRET', '')
EXTERNAL_PRICING_URL = os.getenv('EXTERNAL_PRICING_URL', '')
EXCHANGE_API_URL = os.getenv('EXCHANGE_API_URL', 'https://api.exchangerate-api.com/v4/latest/USD')

# Настройка логирования
def init_logging():
    """Инициализация системы логирования"""
    logging.basicConfig(
        level=logging.INFO,
        format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
        handlers=[
            logging.FileHandler('/app/logs/billing.log'),
            logging.StreamHandler(sys.stdout)
        ]
    )
    return logging.getLogger(__name__)

# Инициализация Redis
def init_redis():
    """Инициализация Redis клиента"""
    try:
        r = redis.from_url(REDIS_URL)
        r.ping()
        logger.info("✅ Redis connected successfully")
        return r
    except Exception as e:
        logger.error(f"❌ Redis connection failed: {e}")
        return None

# Инициализация цен
def init_pricing():
    """Инициализация системы ценообразования"""
    try:
        pricing_service = PricingService(redis_client)
        pricing_service.load_from_redis()
        logger.info("✅ Pricing service initialized")
        return pricing_service
    except Exception as e:
        logger.error(f"❌ Pricing initialization failed: {e}")
        return None

# Инициализация курсов валют
def init_exchange_rates():
    """Инициализация системы курсов валют"""
    try:
        exchange_service = ExchangeRateService(redis_client)
        exchange_service.load_from_redis()
        logger.info("✅ Exchange rates service initialized")
        return exchange_service
    except Exception as e:
        logger.error(f"❌ Exchange rates initialization failed: {e}")
        return None

# Валидационные функции
def validate_jwt(token: str) -> Optional[Dict[str, Any]]:
    """Проверка JWT токена"""
    try:
        payload = jwt.decode(token, JWT_SECRET, algorithms=['HS256'])
        return payload
    except jwt.ExpiredSignatureError:
        logger.warning("JWT token expired")
        return None
    except jwt.InvalidTokenError:
        logger.warning("Invalid JWT token")
        return None

def validate_user_id(user_id: str) -> bool:
    """Проверка корректности ID пользователя"""
    return user_id and user_id.strip() != "" and len(user_id) <= 100

def validate_model_id(model_id: str) -> bool:
    """Проверка корректности ID модели"""
    return model_id and model_id.strip() != "" and len(model_id) <= 200

def validate_reservation_id(reservation_id: str) -> bool:
    """Проверка корректности ID резерва"""
    return reservation_id and reservation_id.strip() != "" and len(reservation_id) <= 100

def validate_amount(amount: float) -> bool:
    """Проверка корректности суммы"""
    return amount is not None and amount > 0 and amount < 1000000

# ExchangeRateService
class ExchangeRateService:
    def __init__(self, redis_client):
        self.redis = redis_client
        self.currency_key = "billing:exchange_rates"
        self.last_update_key = "billing:exchange_last_update"
        self.rates = {}
        self.last_update = None
        
    async def fetch_exchange_rates(self):
        """Получение курсов валют из внешнего источника"""
        try:
            import requests
            response = requests.get(EXCHANGE_API_URL, timeout=10)
            response.raise_for_status()
            data = response.json()
            
            if 'rates' in data:
                self.rates = data['rates']
                self.last_update = datetime.now()
                
                # Сохранение в Redis
                self.redis.hset(self.currency_key, mapping=self.rates)
                self.redis.set(self.last_update_key, self.last_update.isoformat())
                
                logger.info(f"✅ Updated exchange rates: {len(self.rates)} currencies")
                return True
            else:
                logger.error("❌ Invalid response from exchange API")
                return False
                
        except Exception as e:
            logger.error(f"❌ Failed to fetch exchange rates: {e}")
            return False
    
    def load_from_redis(self):
        """Загрузка курсов из Redis"""
        try:
            rates_data = self.redis.hgetall(self.currency_key)
            if rates_data:
                self.rates = {k.decode(): float(v) for k, v in rates_data.items()}
                last_update_str = self.redis.get(self.last_update_key)
                if last_update_str:
                    self.last_update = datetime.fromisoformat(last_update_str.decode())
                logger.info(f"✅ Loaded {len(self.rates)} exchange rates from Redis")
            else:
                # Загрузка из внешнего источника при первом запуске
                asyncio.run(self.fetch_exchange_rates())
        except Exception as e:
            logger.error(f"❌ Failed to load exchange rates: {e}")
    
    def get_rate(self, currency: str) -> Optional[float]:
        """Получение курса валюты"""
        return self.rates.get(currency.upper())
    
    def add_currency(self, currency: str, rate: float) -> bool:
        """Добавление новой валюты"""
        try:
            self.rates[currency.upper()] = rate
            self.redis.hset(self.currency_key, currency.upper(), rate)
            logger.info(f"✅ Added currency {currency} with rate {rate}")
            return True
        except Exception as e:
            logger.error(f"❌ Failed to add currency {currency}: {e}")
            return False
    
    def remove_currency(self, currency: str) -> bool:
        """Удаление валюты"""
        try:
            currency = currency.upper()
            if currency in self.rates:
                del self.rates[currency]
                self.redis.hdel(self.currency_key, currency)
                logger.info(f"✅ Removed currency {currency}")
                return True
            return False
        except Exception as e:
            logger.error(f"❌ Failed to remove currency {currency}: {e}")
            return False
    
    def update_currency_rate(self, currency: str, rate: float) -> bool:
        """Обновление курса валюты"""
        return self.add_currency(currency, rate)
    
    async def start_auto_update(self):
        """Запуск автоматического обновления курсов"""
        while True:
            try:
                await self.fetch_exchange_rates()
                await asyncio.sleep(3600)  # Обновление каждый час
            except Exception as e:
                logger.error(f"❌ Auto update error: {e}")
                await asyncio.sleep(300)  # При ошибке ждем 5 минут

# PricingService
class PricingService:
    def __init__(self, redis_client):
        self.redis = redis_client
        self.pricing_key = "billing:pricing"
        self.pricing = {}
        
    def load_from_redis(self):
        """Загрузка цен из Redis"""
        try:
            pricing_data = self.redis.get(self.pricing_key)
            if pricing_data:
                self.pricing = json.loads(pricing_data)
                logger.info(f"✅ Loaded pricing data: {len(self.pricing)} models")
            else:
                # Базовые цены по умолчанию
                self.pricing = {
                    "gpt-4": {"input": 0.03, "output": 0.06},
                    "gpt-3.5-turbo": {"input": 0.001, "output": 0.002},
                    "claude-3": {"input": 0.015, "output": 0.075}
                }
                self.save_to_redis()
                logger.info("✅ Initialized default pricing")
        except Exception as e:
            logger.error(f"❌ Failed to load pricing: {e}")
    
    def save_to_redis(self):
        """Сохранение цен в Redis"""
        try:
            self.redis.set(self.pricing_key, json.dumps(self.pricing))
            logger.info("✅ Pricing data saved to Redis")
        except Exception as e:
            logger.error(f"❌ Failed to save pricing: {e}")
    
    def update_from_external_source(self) -> bool:
        """Обновление цен из внешнего источника"""
        try:
            if not EXTERNAL_PRICING_URL:
                logger.info("ℹ️  External pricing URL not configured")
                return False
            
            import requests
            response = requests.get(EXTERNAL_PRICING_URL, timeout=10)
            response.raise_for_status()
            data = response.json()
            
            if 'pricing' in data:
                self.pricing = data['pricing']
                self.save_to_redis()
                logger.info("✅ Updated pricing from external source")
                return True
            
            return False
        except Exception as e:
            logger.error(f"❌ Failed to update pricing from external source: {e}")
            return False
    
    def get_price(self, model: str, token_type: str = "input") -> Optional[float]:
        """Получение цены для модели и типа токенов"""
        model_pricing = self.pricing.get(model.lower())
        if model_pricing and token_type in model_pricing:
            return model_pricing[token_type]
        return None
    
    def get_pricing_info(self) -> Dict[str, Any]:
        """Получение информации о ценах"""
        return {
            "models": list(self.pricing.keys()),
            "pricing": self.pricing,
            "last_updated": datetime.now().isoformat()
        }

# BillingService (gRPC handlers)
class BillingService(billing_pb2_grpc.BillingServiceServicer):
    def __init__(self, redis_client, pricing_service, exchange_service):
        self.redis = redis_client
        self.pricing_service = pricing_service
        self.exchange_service = exchange_service
        self.reservations_key = "billing:reservations"
        self.transactions_key = "billing:transactions"
        
    async def Charge(self, request, context):
        """Списание средств"""
        try:
            user_id = request.user_id
            amount = float(request.amount)
            currency = request.currency
            description = request.description
            
            # Валидация
            if not validate_user_id(user_id):
                context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
                context.set_details("Invalid user_id")
                return billing_pb2.ChargeResponse()
            
            if not validate_amount(amount):
                context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
                context.set_details("Invalid amount")
                return billing_pb2.ChargeResponse()
            
            # Получение баланса
            balance_key = f"billing:balance:{user_id}"
            current_balance = float(self.redis.get(balance_key) or 0)
            
            if current_balance < amount:
                context.set_code(grpc.StatusCode.INSUFFICIENT_RESOURCES)
                context.set_details("Insufficient balance")
                return billing_pb2.ChargeResponse()
            
            # Списание
            new_balance = current_balance - amount
            self.redis.set(balance_key, new_balance)
            
            # Запись транзакции
            transaction = {
                "user_id": user_id,
                "amount": -amount,
                "currency": currency,
                "description": description,
                "timestamp": datetime.now().isoformat(),
                "type": "charge"
            }
            
            self.redis.lpush(self.transactions_key, json.dumps(transaction))
            
            # Логирование
            await monitoring_service.log_transaction("charge", user_id, amount, currency)
            
            logger.info(f"✅ Charged {amount} {currency} from user {user_id}")
            
            return billing_pb2.ChargeResponse(
                success=True,
                new_balance=new_balance,
                transaction_id=f"txn_{int(time.time())}"
            )
            
        except Exception as e:
            logger.error(f"❌ Charge failed: {e}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(e))
            return billing_pb2.ChargeResponse()
    
    async def Reserve(self, request, context):
        """Резервирование средств"""
        try:
            user_id = request.user_id
            amount = float(request.amount)
            currency = request.currency
            model_id = request.model_id
            
            # Валидация
            if not validate_user_id(user_id) or not validate_model_id(model_id):
                context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
                context.set_details("Invalid user_id or model_id")
                return billing_pb2.ReserveResponse()
            
            # Расчет стоимости
            cost = await self.calculate_cost(model_id, amount)
            
            # Проверка баланса
            balance_key = f"billing:balance:{user_id}"
            current_balance = float(self.redis.get(balance_key) or 0)
            
            if current_balance < cost:
                context.set_code(grpc.StatusCode.INSUFFICIENT_RESOURCES)
                context.set_details("Insufficient balance")
                return billing_pb2.ReserveResponse()
            
            # Создание резерва
            reservation_id = f"res_{user_id}_{int(time.time())}"
            reservation_data = {
                "user_id": user_id,
                "model_id": model_id,
                "amount": amount,
                "cost": cost,
                "currency": currency,
                "created_at": datetime.now().isoformat(),
                "expires_at": (datetime.now() + timedelta(hours=24)).isoformat()
            }
            
            # Сохранение резерва
            self.redis.hset(self.reservations_key, reservation_id, json.dumps(reservation_data))
            
            logger.info(f"✅ Reserved {cost} {currency} for user {user_id}, model {model_id}")
            
            return billing_pb2.ReserveResponse(
                success=True,
                reservation_id=reservation_id,
                cost=cost
            )
            
        except Exception as e:
            logger.error(f"❌ Reserve failed: {e}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(e))
            return billing_pb2.ReserveResponse()
    
    async def Commit(self, request, context):
        """Подтверждение резерва"""
        try:
            reservation_id = request.reservation_id
            
            # Получение данных резерва
            reservation_data = self.redis.hget(self.reservations_key, reservation_id)
            if not reservation_data:
                context.set_code(grpc.StatusCode.NOT_FOUND)
                context.set_details("Reservation not found")
                return billing_pb2.CommitResponse()
            
            reservation = json.loads(reservation_data)
            
            # Проверка срока действия
            expires_at = datetime.fromisoformat(reservation["expires_at"])
            if datetime.now() > expires_at:
                context.set_code(grpc.StatusCode.DEADLINE_EXCEEDED)
                context.set_details("Reservation expired")
                return billing_pb2.CommitResponse()
            
            # Списание средств
            user_id = reservation["user_id"]
            cost = reservation["cost"]
            
            balance_key = f"billing:balance:{user_id}"
            current_balance = float(self.redis.get(balance_key) or 0)
            
            if current_balance < cost:
                context.set_code(grpc.StatusCode.INSUFFICIENT_RESOURCES)
                context.set_details("Insufficient balance")
                return billing_pb2.CommitResponse()
            
            new_balance = current_balance - cost
            self.redis.set(balance_key, new_balance)
            
            # Запись транзакции
            transaction = {
                "user_id": user_id,
                "amount": -cost,
                "currency": reservation["currency"],
                "description": f"API usage - {reservation['model_id']}",
                "timestamp": datetime.now().isoformat(),
                "type": "commit",
                "reservation_id": reservation_id
            }
            
            self.redis.lpush(self.transactions_key, json.dumps(transaction))
            
            # Удаление резерва
            self.redis.hdel(self.reservations_key, reservation_id)
            
            # Логирование
            await monitoring_service.log_transaction("commit", user_id, cost, reservation["currency"])
            
            logger.info(f"✅ Committed reservation {reservation_id} for user {user_id}")
            
            return billing_pb2.CommitResponse(
                success=True,
                new_balance=new_balance
            )
            
        except Exception as e:
            logger.error(f"❌ Commit failed: {e}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(e))
            return billing_pb2.CommitResponse()
    
    async def calculate_cost(self, model_id: str, amount: float) -> float:
        """Расчет стоимости использования"""
        try:
            # Базовая стоимость за токен
            price_per_token = self.pricing_service.get_price(model_id, "input")
            if not price_per_token:
                price_per_token = 0.001  # Базовую цену
            
            cost = amount * price_per_token
            
            # Применение курса валют если необходимо
            if self.exchange_service:
                usd_rate = self.exchange_service.get_rate("USD")
                if usd_rate:
                    cost = cost / usd_rate
            
            return round(cost, 6)
            
        except Exception as e:
            logger.error(f"❌ Cost calculation failed: {e}")
            return 0.001  # Минимальная стоимость
    
    async def GetBalance(self, request, context):
        """Получение баланса пользователя"""
        try:
            user_id = request.user_id
            
            if not validate_user_id(user_id):
                context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
                context.set_details("Invalid user_id")
                return billing_pb2.GetBalanceResponse()
            
            balance_key = f"billing:balance:{user_id}"
            balance = float(self.redis.get(balance_key) or 0)
            
            return billing_pb2.GetBalanceResponse(
                user_id=user_id,
                balance=balance,
                currency="USD"
            )
            
        except Exception as e:
            logger.error(f"❌ GetBalance failed: {e}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(e))
            return billing_pb2.GetBalanceResponse()
    
    async def AdjustBalance(self, request, context):
        """Корректировка баланса"""
        try:
            user_id = request.user_id
            amount = float(request.amount)
            reason = request.reason
            
            if not validate_user_id(user_id):
                context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
                context.set_details("Invalid user_id")
                return billing_pb2.AdjustBalanceResponse()
            
            balance_key = f"billing:balance:{user_id}"
            current_balance = float(self.redis.get(balance_key) or 0)
            new_balance = current_balance + amount
            
            if new_balance < 0:
                context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
                context.set_details("Balance cannot be negative")
                return billing_pb2.AdjustBalanceResponse()
            
            self.redis.set(balance_key, new_balance)
            
            # Запись транзакции
            transaction = {
                "user_id": user_id,
                "amount": amount,
                "currency": "USD",
                "description": reason,
                "timestamp": datetime.now().isoformat(),
                "type": "adjustment"
            }
            
            self.redis.lpush(self.transactions_key, json.dumps(transaction))
            
            # Логирование
            await monitoring_service.log_transaction("adjustment", user_id, amount, "USD")
            
            logger.info(f"✅ Adjusted balance for user {user_id}: {amount}")
            
            return billing_pb2.AdjustBalanceResponse(
                success=True,
                new_balance=new_balance
            )
            
        except Exception as e:
            logger.error(f"❌ AdjustBalance failed: {e}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(e))
            return billing_pb2.AdjustBalanceResponse()

# HTTP Handlers
def create_checkout():
    """Создание checkout сессии Stripe"""
    try:
        # Имплементация для создания платежной сессии
        # Здесь будет интеграция с Stripe
        return jsonify({"message": "Checkout created"})
    except Exception as e:
        logger.error(f"❌ Create checkout failed: {e}")
        return jsonify({"error": str(e)}), 500

def stripe_webhook():
    """Webhook для Stripe"""
    try:
        # Имплементация webhook обработчика для Stripe
        return jsonify({"status": "ok"})
    except Exception as e:
        logger.error(f"❌ Stripe webhook failed: {e}")
        return jsonify({"error": str(e)}), 500

def admin_pricing():
    """Админ панель - управление ценами"""
    try:
        if not pricing_service:
            return jsonify({"error": "Pricing service not initialized"}), 500
            
        if request.method == "GET":
            pricing_info = pricing_service.get_pricing_info()
            return jsonify(pricing_info)
        elif request.method == "POST":
            data = request.get_json()
            model = data.get("model")
            input_price = data.get("input_price")
            output_price = data.get("output_price")
            
            if model and input_price and output_price:
                pricing_service.pricing[model] = {
                    "input": float(input_price),
                    "output": float(output_price)
                }
                pricing_service.save_to_redis()
                return jsonify({"message": f"Updated pricing for {model}"})
            else:
                return jsonify({"error": "Missing required fields"}), 400
                
    except Exception as e:
        logger.error(f"❌ Admin pricing failed: {e}")
        return jsonify({"error": str(e)}), 500

def admin_update_pricing():
    """Обновление цен из внешнего источника"""
    try:
        if not pricing_service:
            return jsonify({"error": "Pricing service not initialized"}), 500
        success = pricing_service.update_from_external_source()
        if success:
            return jsonify({"message": "Pricing updated successfully"})
        else:
            return jsonify({"message": "Failed to update pricing"}), 500
    except Exception as e:
        logger.error(f"❌ Update pricing failed: {e}")
        return jsonify({"error": str(e)}), 500

def admin_pricing_info():
    """Получение информации о ценах"""
    try:
        pricing_info = pricing_service.get_pricing_info()
        return jsonify(pricing_info)
    except Exception as e:
        logger.error(f"❌ Pricing info failed: {e}")
        return jsonify({"error": str(e)}), 500

def admin_stats():
    """Статистика биллинга"""
    try:
        # Получение базовой статистики
        stats = {
            "total_users": len([k for k in redis_client.keys("billing:balance:*")]),
            "total_transactions": redis_client.llen("billing:transactions"),
            "total_reservations": redis_client.hlen("billing:reservations"),
            "timestamp": datetime.now().isoformat()
        }
        return jsonify(stats)
    except Exception as e:
        logger.error(f"❌ Admin stats failed: {e}")
        return jsonify({"error": str(e)}), 500

def admin_exchange_rates():
    """Управление курсами валют"""
    try:
        if request.method == "GET":
            rates_info = {
                "rates": exchange_service.rates,
                "last_update": exchange_service.last_update.isoformat() if exchange_service.last_update else None
            }
            return jsonify(rates_info)
        elif request.method == "POST":
            data = request.get_json()
            currency = data.get("currency")
            rate = data.get("rate")
            
            if currency and rate:
                success = exchange_service.add_currency(currency, float(rate))
                if success:
                    return jsonify({"message": f"Updated rate for {currency}"})
                else:
                    return jsonify({"error": "Failed to update rate"}), 500
            else:
                return jsonify({"error": "Missing required fields"}), 400
                
    except Exception as e:
        logger.error(f"❌ Exchange rates admin failed: {e}")
        return jsonify({"error": str(e)}), 500

async def admin_update_exchange_rates():
    """Обновление курсов валют"""
    try:
        if not exchange_service:
            return jsonify({"error": "Exchange service not initialized"}), 500
        success = await exchange_service.fetch_exchange_rates()
        if success:
            return jsonify({"message": "Exchange rates updated successfully"})
        else:
            return jsonify({"message": "Failed to update exchange rates"}), 500
    except Exception as e:
        logger.error(f"❌ Update exchange rates failed: {e}")
        return jsonify({"error": str(e)}), 500

async def admin_exchange_rate_sources():
    """Управление источниками курсов валют"""
    try:
        # Возвращаем информацию о доступных источниках
        sources = {
            "primary": EXCHANGE_API_URL,
            "backup": "https://api.fixer.io/v1/latest/USD",
            "status": "operational"
        }
        return jsonify(sources)
    except Exception as e:
        logger.error(f"❌ Exchange rate sources failed: {e}")
        return jsonify({"error": str(e)}), 500

def admin_monitoring():
    """Мониторинг системы"""
    try:
        metrics = monitoring_service.get_metrics()
        return jsonify(metrics)
    except Exception as e:
        logger.error(f"❌ Admin monitoring failed: {e}")
        return jsonify({"error": str(e)}), 500

async def admin_alerts():
    """Управление алертами"""
    try:
        if not monitoring_service:
            return jsonify({"error": "Monitoring service not initialized"}), 500
        # Проверка алертов
        alerts = await monitoring_service.check_alerts()
        return jsonify({"alerts": alerts})
    except Exception as e:
        logger.error(f"❌ Admin alerts failed: {e}")
        return jsonify({"error": str(e)}), 500

def admin_monitoring_thresholds():
    """Настройка порогов мониторинга"""
    try:
        if request.method == "GET":
            # Возвращаем текущие пороги
            thresholds = {
                "low_balance_threshold": 10.0,
                "high_usage_threshold": 1000.0,
                "error_rate_threshold": 0.05
            }
            return jsonify(thresholds)
        elif request.method == "POST":
            data = request.get_json()
            # Обновляем пороги в Redis
            for key, value in data.items():
                redis_client.set(f"billing:threshold:{key}", value)
            return jsonify({"message": "Thresholds updated"})
                
    except Exception as e:
        logger.error(f"❌ Monitoring thresholds failed: {e}")
        return jsonify({"error": str(e)}), 500

# MonitoringService
class MonitoringService:
    def __init__(self, redis_client):
        self.redis = redis_client
        self.metrics_key = "billing:metrics"
        self.alerts_key = "billing:alerts"
        
        # Prometheus метрики
        self.transaction_counter = Counter('billing_transactions_total', 'Total transactions', ['type', 'status'])
        self.transaction_duration = Histogram('billing_transaction_duration_seconds', 'Transaction duration')
        self.active_reservations = Gauge('billing_active_reservations', 'Active reservations')
        self.balance_gauge = Gauge('billing_user_balance', 'User balance', ['user_id'])
        
    async def log_transaction(self, transaction_type: str, user_id: str, amount: float, currency: str):
        """Логирование транзакции"""
        try:
            transaction_data = {
                "type": transaction_type,
                "user_id": user_id,
                "amount": amount,
                "currency": currency,
                "timestamp": datetime.now().isoformat()
            }
            
            # Сохранение в Redis
            self.redis.lpush("billing:transactions:recent", json.dumps(transaction_data))
            self.redis.ltrim("billing:transactions:recent", 0, 999)  # Храним только последние 1000
            
            # Обновление метрик
            self.transaction_counter.labels(type=transaction_type, status='success').inc()
            
            logger.info(f"📊 Transaction logged: {transaction_type} {amount} {currency} for user {user_id}")
            
        except Exception as e:
            logger.error(f"❌ Failed to log transaction: {e}")
    
    async def check_alerts(self) -> List[Dict[str, Any]]:
        """Проверка алертов"""
        alerts = []
        
        try:
            # Проверка низкого баланса пользователей
            low_balance_threshold = float(redis_client.get("billing:threshold:low_balance_threshold") or 10.0)
            
            balance_keys = redis_client.keys("billing:balance:*")
            for key in balance_keys:
                user_id = key.decode().replace("billing:balance:", "")
                balance = float(redis_client.get(key) or 0)
                
                if balance < low_balance_threshold:
                    alert = {
                        "type": "low_balance",
                        "user_id": user_id,
                        "balance": balance,
                        "threshold": low_balance_threshold,
                        "timestamp": datetime.now().isoformat()
                    }
                    alerts.append(alert)
                    
                    # Сохранение алерта
                    self.redis.lpush(self.alerts_key, json.dumps(alert))
            
            # Проверка количества активных резервов
            active_reservations = redis_client.hlen("billing:reservations")
            high_usage_threshold = float(redis_client.get("billing:threshold:high_usage_threshold") or 1000.0)
            
            if active_reservations > high_usage_threshold:
                alert = {
                    "type": "high_usage",
                    "active_reservations": active_reservations,
                    "threshold": high_usage_threshold,
                    "timestamp": datetime.now().isoformat()
                }
                alerts.append(alert)
                self.redis.lpush(self.alerts_key, json.dumps(alert))
            
            logger.info(f"🔍 Checked alerts: {len(alerts)} found")
            
        except Exception as e:
            logger.error(f"❌ Failed to check alerts: {e}")
        
        return alerts
    
    async def trigger_alert(self, alert_type: str, message: str, severity: str = "warning"):
        """Срабатывание алерта"""
        try:
            alert = {
                "type": alert_type,
                "message": message,
                "severity": severity,
                "timestamp": datetime.now().isoformat()
            }
            
            self.redis.lpush(self.alerts_key, json.dumps(alert))
            
            # Здесь можно добавить отправку уведомлений (email, Slack, etc.)
            logger.warning(f"🚨 Alert triggered: {alert_type} - {message}")
            
        except Exception as e:
            logger.error(f"❌ Failed to trigger alert: {e}")
    
    def get_metrics(self) -> Dict[str, Any]:
        """Получение метрик"""
        try:
            metrics = {
                "active_reservations": redis_client.hlen("billing:reservations"),
                "total_transactions": redis_client.llen("billing:transactions"),
                "recent_transactions": redis_client.lrange("billing:transactions:recent", 0, 9),
                "alerts": redis_client.lrange(self.alerts_key, 0, 19),
                "timestamp": datetime.now().isoformat()
            }
            
            return metrics
            
        except Exception as e:
            logger.error(f"❌ Failed to get metrics: {e}")
            return {}
    
    async def check_user_balance(self, user_id: str) -> Optional[float]:
        """Проверка баланса пользователя"""
        try:
            balance_key = f"billing:balance:{user_id}"
            balance = float(self.redis.get(balance_key) or 0)
            
            # Обновление Prometheus метрики
            self.balance_gauge.labels(user_id=user_id).set(balance)
            
            return balance
            
        except Exception as e:
            logger.error(f"❌ Failed to check user balance: {e}")
            return None
    
    async def check_usage(self) -> Dict[str, Any]:
        """Проверка использования системы"""
        try:
            usage_stats = {
                "active_reservations": redis_client.hlen("billing:reservations"),
                "total_users": len([k for k in redis_client.keys("billing:balance:*")]),
                "redis_memory": redis_client.info()['used_memory_human'],
                "timestamp": datetime.now().isoformat()
            }
            
            # Обновление метрик
            self.active_reservations.set(usage_stats["active_reservations"])
            
            return usage_stats
            
        except Exception as e:
            logger.error(f"❌ Failed to check usage: {e}")
            return {}

# Глобальные переменные
logger = None
redis_client = None
pricing_service = None
exchange_service = None
monitoring_service = None
billing_service = None

# Основная функция запуска
def serve():
    """Основная функция запуска billing сервиса"""
    global logger, redis_client, pricing_service, exchange_service, monitoring_service, billing_service
    
    try:
        # 1. Инициализация логирования
        logger = init_logging()
        logger.info("🚀 Starting Billing Service...")
        
        # 2. Инициализация Redis
        redis_client = init_redis()
        if not redis_client:
            logger.error("❌ Failed to initialize Redis. Exiting.")
            return False
        
        # 3. Инициализация сервиса ценообразования
        pricing_service = init_pricing()
        if not pricing_service:
            logger.error("❌ Failed to initialize pricing service. Exiting.")
            return False
        
        # 4. Инициализация сервиса курсов валют
        exchange_service = init_exchange_rates()
        if not exchange_service:
            logger.error("❌ Failed to initialize exchange rates service. Exiting.")
            return False
        
        # 5. Инициализация мониторинга
        monitoring_service = MonitoringService(redis_client)
        logger.info("✅ Monitoring service initialized")
        
        # 6. Инициализация billing сервиса
        billing_service = BillingService(redis_client, pricing_service, exchange_service)
        logger.info("✅ Billing service initialized")
        
        # 7. Запуск gRPC сервера
        grpc_server = grpc.aio.server()
        billing_pb2_grpc.add_BillingServiceServicer_to_server(billing_service, grpc_server)
        grpc_server.add_insecure_port('[::]:50051')
        logger.info("✅ gRPC server configured")
        
        # 8. Запуск HTTP сервера
        app = Flask(__name__)
        
        # Регистрация HTTP handlers
        app.route('/api/checkout', methods=['POST'])(create_checkout)
        app.route('/webhook/stripe', methods=['POST'])(stripe_webhook)
        
        # Admin routes
        app.route('/admin/pricing', methods=['GET', 'POST'])(admin_pricing)
        app.route('/admin/pricing/update', methods=['POST'])(admin_update_pricing)
        app.route('/admin/pricing/info', methods=['GET'])(admin_pricing_info)
        app.route('/admin/stats', methods=['GET'])(admin_stats)
        app.route('/admin/exchange-rates', methods=['GET', 'POST'])(admin_exchange_rates)
        
        # Async admin routes
        async def admin_update_exchange_rates_async():
            return await admin_update_exchange_rates()
        async def admin_exchange_rate_sources_async():
            return await admin_exchange_rate_sources()
        async def admin_alerts_async():
            return await admin_alerts()
            
        app.route('/admin/exchange-rates/update', methods=['POST'])(admin_update_exchange_rates_async)
        app.route('/admin/exchange-rate-sources', methods=['GET'])(admin_exchange_rate_sources_async)
        app.route('/admin/monitoring', methods=['GET'])(admin_monitoring)
        app.route('/admin/alerts', methods=['GET'])(admin_alerts_async)
        app.route('/admin/monitoring/thresholds', methods=['GET', 'POST'])(admin_monitoring_thresholds)
        
        logger.info("✅ HTTP routes configured")
        
        # 9. Запуск мониторинга Prometheus
        start_http_server(8000)
        logger.info("✅ Prometheus metrics server started on port 8000")
        
        # 10. Запуск фоновых задач
        async def background_tasks():
            """Фоновые задачи"""
            try:
                # Автоматическое обновление курсов валют
                asyncio.create_task(exchange_service.start_auto_update())
                
                # Периодическая проверка алертов
                while True:
                    try:
                        await monitoring_service.check_alerts()
                        await asyncio.sleep(300)  # Проверка каждые 5 минут
                    except Exception as e:
                        logger.error(f"❌ Background task error: {e}")
                        await asyncio.sleep(60)
                        
            except Exception as e:
                logger.error(f"❌ Background tasks failed: {e}")
        
        # 11. Запуск серверов
        async def start_servers():
            """Запуск всех серверов"""
            try:
                # Запуск gRPC сервера
                grpc_server_task = grpc_server.start()
                
                # Запуск HTTP сервера в отдельном потоке
                import threading
                http_thread = threading.Thread(
                    target=lambda: app.run(host='0.0.0.0', port=8080, debug=False),
                    daemon=True
                )
                http_thread.start()
                
                # Запуск фоновых задач
                background_task = asyncio.create_task(background_tasks())
                
                logger.info("🎉 All servers started successfully!")
                logger.info("📊 gRPC server: localhost:50051")
                logger.info("🌐 HTTP server: localhost:8080")
                logger.info("📈 Metrics: localhost:8000")
                
                # Ожидание завершения
                await grpc_server.wait_for_termination()
                
            except KeyboardInterrupt:
                logger.info("🛑 Shutting down servers...")
            except Exception as e:
                logger.error(f"❌ Server startup failed: {e}")
            finally:
                await grpc_server.stop(grace=1)
        
        # Запуск основного цикла
        asyncio.run(start_servers())
        
        return True
        
    except Exception as e:
        logger.error(f"❌ Service startup failed: {e}")
        logger.error(traceback.format_exc())
        return False

if __name__ == "__main__":
    success = serve()
    sys.exit(0 if success else 1)