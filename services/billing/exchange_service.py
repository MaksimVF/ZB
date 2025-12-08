
// services/billing/exchange_service.py
import os
import json
import logging
import time
import threading
import requests
from decimal import Decimal, InvalidOperation
from datetime import datetime
from concurrent import futures

import grpc
import redis

import billing_pb2
import billing_pb2_grpc

# =============================================================================
# Exchange Rate Service
# =============================================================================
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("exchange_service")

# Initialize services
r = redis.from_url(os.getenv("REDIS_URL", "redis://redis:6379"))

# Default exchange rates
EXCHANGE_RATES = {
    "USD": Decimal("1"),
    "EUR": Decimal("0.92"),
    "RUB": Decimal("96.50"),
    "USDT": Decimal("1"),
    "GBP": Decimal("0.79"),
    "CNY": Decimal("7.23"),
    "JPY": Decimal("156.75"),
    "INR": Decimal("83.45")
}

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

# Exchange Rate Service
class ExchangeRateService(billing_pb2_grpc.ExchangeRateServiceServicer):

    def GetExchangeRates(self, request, context):
        """Get current exchange rates"""
        try:
            return billing_pb2.ExchangeResponse(
                rates=EXCHANGE_MANAGER.rates,
                last_updated=EXCHANGE_MANAGER.last_updated,
                supported_currencies=EXCHANGE_MANAGER.supported_currencies
            )
        except Exception as e:
            logger.error(f"Error getting exchange rates: {e}")
            context.abort_with_status(grpc.StatusCode.INTERNAL, "Internal server error")

    def UpdateExchangeRates(self, request, context):
        """Manually update exchange rates"""
        try:
            success = EXCHANGE_MANAGER.fetch_exchange_rates()
            if success:
                return billing_pb2.UpdateResponse(success=True)
            else:
                raise ExternalServiceError("Failed to update exchange rates")
        except ExternalServiceError as e:
            logger.error(f"Error updating exchange rates: {e}")
            context.abort_with_status(grpc.StatusCode.INTERNAL, str(e))
        except Exception as e:
            logger.error(f"Unexpected error: {e}")
            context.abort_with_status(grpc.StatusCode.INTERNAL, "Internal server error")

    def AddCurrency(self, request, context):
        """Add a new currency"""
        try:
            currency = request.currency
            rate = Decimal(str(request.rate))

            success = EXCHANGE_MANAGER.add_currency(currency, rate)
            if success:
                return billing_pb2.CurrencyResponse(success=True)
            else:
                raise ValidationError("Failed to add currency")
        except ValidationError as e:
            logger.error(f"Error adding currency: {e}")
            context.abort_with_status(grpc.StatusCode.INVALID_ARGUMENT, str(e))
        except Exception as e:
            logger.error(f"Unexpected error: {e}")
            context.abort_with_status(grpc.StatusCode.INTERNAL, "Internal server error")

    def RemoveCurrency(self, request, context):
        """Remove a currency"""
        try:
            currency = request.currency

            success = EXCHANGE_MANAGER.remove_currency(currency)
            if success:
                return billing_pb2.CurrencyResponse(success=True)
            else:
                raise ValidationError("Failed to remove currency")
        except ValidationError as e:
            logger.error(f"Error removing currency: {e}")
            context.abort_with_status(grpc.StatusCode.INVALID_ARGUMENT, str(e))
        except Exception as e:
            logger.error(f"Unexpected error: {e}")
            context.abort_with_status(grpc.StatusCode.INTERNAL, "Internal server error")

    def UpdateCurrencyRate(self, request, context):
        """Update exchange rate for a specific currency"""
        try:
            currency = request.currency
            rate = Decimal(str(request.rate))

            success = EXCHANGE_MANAGER.update_currency_rate(currency, rate)
            if success:
                return billing_pb2.CurrencyResponse(success=True)
            else:
                raise ValidationError("Failed to update currency rate")
        except ValidationError as e:
            logger.error(f"Error updating currency rate: {e}")
            context.abort_with_status(grpc.StatusCode.INVALID_ARGUMENT, str(e))
        except Exception as e:
            logger.error(f"Unexpected error: {e}")
            context.abort_with_status(grpc.StatusCode.INTERNAL, "Internal server error")

# =============================================================================
# Запуск
# =============================================================================
def serve():
    # gRPC
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    billing_pb2_grpc.add_ExchangeRateServiceServicer_to_server(ExchangeRateService(), server)
    server.add_insecure_port("[::]:50054")

    logger.info("Exchange Rate Service: gRPC :50054")
    server.start()
    server.wait_for_termination()

if __name__ == "__main__":
    serve()






