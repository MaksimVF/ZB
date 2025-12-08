
// services/billing/monitoring_service.py
import os
import json
import logging
import time
import threading
from decimal import Decimal, InvalidOperation
from datetime import datetime
from concurrent import futures

import grpc
import redis

import billing_pb2
import billing_pb2_grpc

# =============================================================================
# Monitoring Service
# =============================================================================
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("monitoring_service")

# Initialize services
r = redis.from_url(os.getenv("REDIS_URL", "redis://redis:6379"))

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

# Monitoring Service
class MonitoringService(billing_pb2_grpc.MonitoringServiceServicer):

    def GetMetrics(self, request, context):
        """Get current metrics"""
        try:
            metrics = MONITORING.get_metrics()
            return billing_pb2.MetricsResponse(
                metrics=json.dumps(metrics["metrics"]),
                thresholds=json.dumps(metrics["thresholds"]),
                last_alert=metrics["last_alert"] or ""
            )
        except Exception as e:
            logger.error(f"Error getting metrics: {e}")
            context.abort_with_status(grpc.StatusCode.INTERNAL, "Internal server error")

    def GetAlerts(self, request, context):
        """Get recent alerts"""
        try:
            alerts = []
            alert_stream = r.xrange("billing:alerts")
            for alert in alert_stream:
                alerts.append(alert)
                if len(alerts) >= 50:  # Limit to 50 most recent alerts
                    break

            return billing_pb2.AlertsResponse(
                alerts=json.dumps(alerts),
                count=len(alerts)
            )
        except Exception as e:
            logger.error(f"Error getting alerts: {e}")
            context.abort_with_status(grpc.StatusCode.INTERNAL, "Internal server error")

    def UpdateThresholds(self, request, context):
        """Update alert thresholds"""
        try:
            thresholds = json.loads(request.thresholds)
            if not isinstance(thresholds, dict):
                raise ValidationError("Invalid thresholds format")

            with MONITORING.lock:
                for key, value in thresholds.items():
                    if key in MONITORING.alert_thresholds:
                        if key == "low_balance":
                            MONITORING.alert_thresholds[key] = Decimal(str(value))
                        else:
                            MONITORING.alert_thresholds[key] = value
                    else:
                        logger.warning(f"Invalid threshold key: {key}")

                logger.info(f"Updated monitoring thresholds: {MONITORING.alert_thresholds}")
                return billing_pb2.UpdateResponse(success=True)
        except Exception as e:
            logger.error(f"Error updating thresholds: {e}")
            context.abort_with_status(grpc.StatusCode.INTERNAL, "Internal server error")

# =============================================================================
# Запуск
# =============================================================================
def serve():
    # gRPC
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    billing_pb2_grpc.add_MonitoringServiceServicer_to_server(MonitoringService(), server)
    server.add_insecure_port("[::]:50055")

    logger.info("Monitoring Service: gRPC :50055")
    server.start()
    server.wait_for_termination()

if __name__ == "__main__":
    serve()





