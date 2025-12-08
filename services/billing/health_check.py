






import os
import logging
import requests
from typing import Optional, Dict, Any

# Health check configuration
CONSUL_HTTP_ADDR = os.getenv("CONSUL_HTTP_ADDR", "localhost:8500")

# Configure logging
logger = logging.getLogger("health_check")

class HealthCheck:
    """Health check using Consul"""

    def __init__(self, consul_addr: str = CONSUL_HTTP_ADDR):
        self.consul_addr = consul_addr
        self.base_url = f"http://{consul_addr}/v1"

    def check_service_health(self, service_name: str) -> bool:
        """Check service health"""
        try:
            url = f"{self.base_url}/health/checks/{service_name}"
            response = requests.get(url)
            response.raise_for_status()
            checks = response.json()
            return all(check["Status"] == "passing" for check in checks)
        except requests.RequestException as e:
            logger.error(f"Failed to check health for {service_name}: {e}")
            return False

    def get_service_health(self, service_name: str) -> Dict[str, Any]:
        """Get service health details"""
        try:
            url = f"{self.base_url}/health/checks/{service_name}"
            response = requests.get(url)
            response.raise_for_status()
            return response.json()
        except requests.RequestException as e:
            logger.error(f"Failed to get health for {service_name}: {e}")
            return {}

    def check_all_services_health(self) -> Dict[str, bool]:
        """Check health of all services"""
        try:
            url = f"{self.base_url}/health/state/critical"
            response = requests.get(url)
            response.raise_for_status()
            return response.json()
        except requests.RequestException as e:
            logger.error(f"Failed to check health of all services: {e}")
            return {}

    def check_service_status(self, service_name: str) -> str:
        """Check service status"""
        try:
            url = f"{self.base_url}/health/state/any?filter=ServiceName=={service_name}"
            response = requests.get(url)
            response.raise_for_status()
            checks = response.json()
            if not checks:
                return "unknown"
            return checks[0]["Status"]
        except requests.RequestException as e:
            logger.error(f"Failed to check status for {service_name}: {e}")
            return "unknown"

    def check_service_availability(self, service_name: str) -> bool:
        """Check service availability"""
        try:
            url = f"{self.base_url}/health/service/{service_name}"
            response = requests.get(url)
            response.raise_for_status()
            checks = response.json()
            return any(check["Status"] == "passing" for check in checks)
        except requests.RequestException as e:
            logger.error(f"Failed to check availability for {service_name}: {e}")
            return False

# Singleton instance
health_check = HealthCheck()

# Example usage
if __name__ == "__main__":
    # Check service health
    health = health_check.check_service_health("billing-core")
    print(f"Billing service health: {health}")

    # Get service health details
    health_details = health_check.get_service_health("billing-core")
    print(f"Billing service health details: {health_details}")

    # Check all services health
    all_health = health_check.check_all_services_health()
    print(f"All services health: {all_health}")

    # Check service status
    status = health_check.check_service_status("billing-core")
    print(f"Billing service status: {status}")

    # Check service availability
    available = health_check.check_service_availability("billing-core")
    print(f"Billing service available: {available}")
















