





import os
import logging
import requests
from typing import Optional, Dict, Any
from functools import lru_cache

# Service discovery configuration
CONSUL_HTTP_ADDR = os.getenv("CONSUL_HTTP_ADDR", "localhost:8500")

# Configure logging
logger = logging.getLogger("service_discovery")

class ServiceDiscovery:
    """Service discovery using Consul"""

    def __init__(self, consul_addr: str = CONSUL_HTTP_ADDR):
        self.consul_addr = consul_addr
        self.base_url = f"http://{consul_addr}/v1"

    def get_service(self, service_name: str) -> Optional[Dict[str, Any]]:
        """Get service information from Consul"""
        try:
            url = f"{self.base_url}/catalog/service/{service_name}"
            response = requests.get(url)
            response.raise_for_status()
            services = response.json()

            if not services:
                logger.warning(f"No instances found for service: {service_name}")
                return None

            # Return the first service instance
            return services[0]
        except requests.RequestException as e:
            logger.error(f"Failed to get service {service_name} from Consul: {e}")
            return None

    def get_service_address(self, service_name: str) -> Optional[str]:
        """Get service address from Consul"""
        service = self.get_service(service_name)
        if service:
            address = service.get("ServiceAddress")
            port = service.get("ServicePort")
            if address and port:
                return f"{address}:{port}"
        return None

    def register_service(self, service_name: str, address: str, port: int, tags: list = None):
        """Register service with Consul"""
        try:
            url = f"{self.base_url}/agent/service/register"
            payload = {
                "Name": service_name,
                "Address": address,
                "Port": port,
                "Tags": tags or []
            }
            response = requests.put(url, json=payload)
            response.raise_for_status()
            logger.info(f"Registered service {service_name} with Consul")
        except requests.RequestException as e:
            logger.error(f"Failed to register service {service_name} with Consul: {e}")

    def deregister_service(self, service_id: str):
        """Deregister service from Consul"""
        try:
            url = f"{self.base_url}/agent/service/deregister/{service_id}"
            response = requests.put(url)
            response.raise_for_status()
            logger.info(f"Deregistered service {service_id} from Consul")
        except requests.RequestException as e:
            logger.error(f"Failed to deregister service {service_id} from Consul: {e}")

    def get_all_services(self) -> list:
        """Get all services from Consul"""
        try:
            url = f"{self.base_url}/catalog/services"
            response = requests.get(url)
            response.raise_for_status()
            return list(response.json().keys())
        except requests.RequestException as e:
            logger.error(f"Failed to get services from Consul: {e}")
            return []

    def health_check(self, service_name: str) -> bool:
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

# Singleton instance
@lru_cache(maxsize=1)
def get_service_discovery() -> ServiceDiscovery:
    """Get singleton instance of ServiceDiscovery"""
    return ServiceDiscovery()

# Service discovery decorator
def discover_service(service_name: str):
    """Decorator for service discovery"""
    def decorator(f):
        def wrapper(*args, **kwargs):
            sd = get_service_discovery()
            address = sd.get_service_address(service_name)
            if not address:
                raise Exception(f"Service {service_name} not found in Consul")
            return f(*args, **kwargs, service_address=address)
        return wrapper
    return decorator

# Example usage
if __name__ == "__main__":
    # Initialize service discovery
    sd = get_service_discovery()

    # Register a service
    sd.register_service("billing-core", "localhost", 50052, ["billing", "core"])

    # Get a service
    billing_service = sd.get_service("billing-core")
    print(f"Billing service: {billing_service}")

    # Get all services
    services = sd.get_all_services()
    print(f"All services: {services}")

    # Check health
    health = sd.health_check("billing-core")
    print(f"Billing service health: {health}")












