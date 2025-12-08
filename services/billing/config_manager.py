





import os
import logging
import requests
from typing import Optional, Dict, Any
from functools import lru_cache

# Configuration management
CONSUL_HTTP_ADDR = os.getenv("CONSUL_HTTP_ADDR", "localhost:8500")

# Configure logging
logger = logging.getLogger("config_manager")

class ConfigManager:
    """Configuration manager using Consul"""

    def __init__(self, consul_addr: str = CONSUL_HTTP_ADDR):
        self.consul_addr = consul_addr
        self.base_url = f"http://{consul_addr}/v1"

    def get_config(self, key: str, default: Any = None) -> Any:
        """Get configuration from Consul"""
        try:
            url = f"{self.base_url}/kv/{key}"
            response = requests.get(url)
            response.raise_for_status()
            if response.json():
                return response.json()[0]["Value"]
            return default
        except requests.RequestException as e:
            logger.error(f"Failed to get config {key} from Consul: {e}")
            return default

    def set_config(self, key: str, value: Any):
        """Set configuration in Consul"""
        try:
            url = f"{self.base_url}/kv/{key}"
            response = requests.put(url, json={"Value": value})
            response.raise_for_status()
            logger.info(f"Set config {key} in Consul")
        except requests.RequestException as e:
            logger.error(f"Failed to set config {key} in Consul: {e}")

    def delete_config(self, key: str):
        """Delete configuration from Consul"""
        try:
            url = f"{self.base_url}/kv/{key}"
            response = requests.delete(url)
            response.raise_for_status()
            logger.info(f"Deleted config {key} from Consul")
        except requests.RequestException as e:
            logger.error(f"Failed to delete config {key} from Consul: {e}")

    def get_all_configs(self, prefix: str = "") -> Dict[str, Any]:
        """Get all configurations from Consul"""
        try:
            url = f"{self.base_url}/kv/{prefix}?recurse"
            response = requests.get(url)
            response.raise_for_status()
            configs = {}
            for item in response.json():
                key = item["Key"]
                value = item["Value"]
                configs[key] = value
            return configs
        except requests.RequestException as e:
            logger.error(f"Failed to get configs from Consul: {e}")
            return {}

    def watch_config(self, key: str, callback):
        """Watch configuration changes in Consul"""
        try:
            url = f"{self.base_url}/kv/{key}?wait=5m"
            while True:
                response = requests.get(url)
                response.raise_for_status()
                if response.json():
                    value = response.json()[0]["Value"]
                    callback(value)
        except requests.RequestException as e:
            logger.error(f"Failed to watch config {key} in Consul: {e}")

# Singleton instance
@lru_cache(maxsize=1)
def get_config_manager() -> ConfigManager:
    """Get singleton instance of ConfigManager"""
    return ConfigManager()

# Configuration decorator
def config_value(key: str, default: Any = None):
    """Decorator for configuration values"""
    def decorator(f):
        def wrapper(*args, **kwargs):
            cm = get_config_manager()
            value = cm.get_config(key, default)
            return f(*args, **kwargs, config_value=value)
        return wrapper
    return decorator

# Example usage
if __name__ == "__main__":
    # Initialize config manager
    cm = get_config_manager()

    # Set a config
    cm.set_config("billing/max_balance", 1000)

    # Get a config
    max_balance = cm.get_config("billing/max_balance", 500)
    print(f"Max balance: {max_balance}")

    # Get all configs
    configs = cm.get_all_configs("billing/")
    print(f"Billing configs: {configs}")

    # Watch a config
    def on_config_change(value):
        print(f"Config changed: {value}")

    # cm.watch_config("billing/max_balance", on_config_change)













