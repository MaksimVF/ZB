






import os
import logging
import requests
import time
from typing import Optional, Dict, Any

# Service registration configuration
CONSUL_HTTP_ADDR = os.getenv("CONSUL_HTTP_ADDR", "localhost:8500")

# Configure logging
logger = logging.getLogger("service_registration")

class ServiceRegistration:
    """Service registration with Consul"""

    def __init__(self, consul_addr: str = CONSUL_HTTP_ADDR):
        self.consul_addr = consul_addr
        self.base_url = f"http://{consul_addr}/v1"

    def register_service(self, service_name: str, address: str, port: int, tags: list = None, check_url: str = None):
        """Register service with Consul"""
        try:
            url = f"{self.base_url}/agent/service/register"
            payload = {
                "Name": service_name,
                "Address": address,
                "Port": port,
                "Tags": tags or []
            }

            # Add health check if provided
            if check_url:
                payload["Check"] = {
                    "HTTP": f"http://{address}:{port}{check_url}",
                    "Interval": "10s",
                    "Timeout": "5s"
                }

            response = requests.put(url, json=payload)
            response.raise_for_status()
            logger.info(f"Registered service {service_name} with Consul")
            return True
        except requests.RequestException as e:
            logger.error(f"Failed to register service {service_name} with Consul: {e}")
            return False

    def deregister_service(self, service_id: str):
        """Deregister service from Consul"""
        try:
            url = f"{self.base_url}/agent/service/deregister/{service_id}"
            response = requests.put(url)
            response.raise_for_status()
            logger.info(f"Deregistered service {service_id} from Consul")
            return True
        except requests.RequestException as e:
            logger.error(f"Failed to deregister service {service_id} from Consul: {e}")
            return False

    def register_with_retry(self, service_name: str, address: str, port: int, tags: list = None, check_url: str = None, retries: int = 3, delay: int = 5):
        """Register service with retry logic"""
        for attempt in range(retries):
            try:
                if self.register_service(service_name, address, port, tags, check_url):
                    return True
                time.sleep(delay)
            except Exception as e:
                logger.warning(f"Attempt {attempt + 1} failed: {e}")
                time.sleep(delay)
        return False

    def get_service_id(self, service_name: str, address: str, port: int) -> Optional[str]:
        """Get service ID from Consul"""
        try:
            url = f"{self.base_url}/agent/services"
            response = requests.get(url)
            response.raise_for_status()
            services = response.json()
            for service_id, service in services.items():
                if (service.get("Service") == service_name and
                    service.get("Address") == address and
                    service.get("Port") == port):
                    return service_id
            return None
        except requests.RequestException as e:
            logger.error(f"Failed to get service ID for {service_name}: {e}")
            return None

    def update_service(self, service_id: str, service_name: str, address: str, port: int, tags: list = None, check_url: str = None):
        """Update service registration in Consul"""
        try:
            # First deregister the old service
            if not self.deregister_service(service_id):
                return False

            # Then register the new service
            return self.register_service(service_name, address, port, tags, check_url)
        except Exception as e:
            logger.error(f"Failed to update service {service_id}: {e}")
            return False

# Singleton instance
service_registration = ServiceRegistration()

# Example usage
if __name__ == "__main__":
    # Register a service
    success = service_registration.register_service(
        "billing-core",
        "localhost",
        50052,
        ["billing", "core"],
        "/health"
    )
    print(f"Service registration success: {success}")

    # Get service ID
    service_id = service_registration.get_service_id("billing-core", "localhost", 50052)
    print(f"Service ID: {service_id}")

    # Update service
    if service_id:
        update_success = service_registration.update_service(
            service_id,
            "billing-core",
            "localhost",
            50052,
            ["billing", "core", "updated"],
            "/health"
        )
        print(f"Service update success: {update_success}")

    # Deregister service
    if service_id:
        deregister_success = service_registration.deregister_service(service_id)
        print(f"Service deregistration success: {deregister_success}")















