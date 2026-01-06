
# services/billing/redis_manager.py
import os
import redis
import redis.connection
from functools import wraps
from datetime import datetime, timedelta
import logging
import time
from typing import Optional, Dict, Any, List, Union

logger = logging.getLogger("redis_manager")

class RedisManager:
    """Advanced Redis connection manager with connection pooling and performance optimizations"""

    def __init__(self, redis_url: str = None, pool_size: int = 10, max_connections: int = 100):
        """
        Initialize Redis connection pool

        Args:
            redis_url: Redis connection URL
            pool_size: Initial connection pool size
            max_connections: Maximum connections in pool
        """
        self.redis_url = redis_url or os.getenv("REDIS_URL", "redis://redis:6379")
        self.pool_size = pool_size
        self.max_connections = max_connections

        # Create connection pool
        self.connection_pool = redis.ConnectionPool.from_url(
            self.redis_url,
            max_connections=self.max_connections,
            decode_responses=True
        )

        # Create thread-safe client
        self.client = redis.Redis(
            connection_pool=self.connection_pool,
            socket_timeout=5,
            socket_connect_timeout=2,
            retry_on_timeout=True,
            health_check_interval=30
        )

        # Health monitoring
        self.last_health_check = time.time()
        self.health_check_interval = 60  # seconds

    def get_connection(self):
        """Get a connection from the pool"""
        try:
            return redis.Redis(connection_pool=self.connection_pool)
        except Exception as e:
            logger.error(f"Failed to get Redis connection: {e}")
            raise

    def pipeline(self):
        """Get a Redis pipeline for batch operations"""
        return self.client.pipeline()

    def transaction(self):
        """Get a Redis transaction"""
        return self.client.pipeline(transaction=True)

    def health_check(self) -> bool:
        """Check Redis health status"""
        try:
            if time.time() - self.last_health_check > self.health_check_interval:
                ping = self.client.ping()
                self.last_health_check = time.time()
                return ping == True
            return True
        except Exception as e:
            logger.error(f"Redis health check failed: {e}")
            return False

    def cache_get(self, key: str, expiration: int = None) -> Optional[str]:
        """
        Get cached value with optional expiration check

        Args:
            key: Cache key
            expiration: Optional TTL in seconds

        Returns:
            Cached value or None if not found/expired
        """
        try:
            value = self.client.get(key)
            if value is None:
                return None

            if expiration:
                # Check if cache is still valid
                ttl = self.client.ttl(key)
                if ttl < 0:  # Key exists but has no TTL
                    return value

                # If remaining TTL is less than requested, refresh it
                if ttl < expiration * 0.8:  # Refresh if less than 80% remaining
                    self.client.expire(key, expiration)

            return value
        except Exception as e:
            logger.error(f"Cache get failed for {key}: {e}")
            return None

    def cache_set(self, key: str, value: str, expiration: int = None):
        """
        Set cached value with optional expiration

        Args:
            key: Cache key
            value: Value to cache
            expiration: TTL in seconds
        """
        try:
            if expiration:
                self.client.setex(key, expiration, value)
            else:
                self.client.set(key, value)
        except Exception as e:
            logger.error(f"Cache set failed for {key}: {e}")
            raise

    def cache_delete(self, key: str):
        """Delete cached value"""
        try:
            return self.client.delete(key)
        except Exception as e:
            logger.error(f"Cache delete failed for {key}: {e}")
            return False

    def batch_operations(self, operations: List[Dict[str, Any]]) -> List[Any]:
        """
        Execute batch operations using pipeline

        Args:
            operations: List of operation dictionaries with format:
                {
                    'method': 'method_name',
                    'args': [arg1, arg2, ...],
                    'kwargs': {kwarg1: val1, ...}
                }

        Returns:
            List of results from pipeline execution
        """
        if not operations:
            return []

        try:
            with self.pipeline() as pipe:
                for op in operations:
                    method = getattr(pipe, op['method'], None)
                    if not method:
                        continue

                    if 'kwargs' in op:
                        method(*op.get('args', []), **op.get('kwargs', {}))
                    else:
                        method(*op.get('args', []))

                results = pipe.execute()
                return results
        except Exception as e:
            logger.error(f"Batch operations failed: {e}")
            raise

    def close(self):
        """Close the connection pool"""
        try:
            self.connection_pool.disconnect()
        except Exception as e:
            logger.error(f"Failed to close connection pool: {e}")

# Global Redis manager instance
redis_manager = RedisManager()

# Decorator for caching function results
def redis_cache(key_prefix: str, expiration: int = 300):
    """
    Decorator for caching function results in Redis

    Args:
        key_prefix: Prefix for cache keys
        expiration: Cache TTL in seconds
    """
    def decorator(func):
        @wraps(func)
        def wrapper(*args, **kwargs):
            # Generate cache key
            cache_key_parts = [key_prefix]
            cache_key_parts.extend([str(arg) for arg in args])
            cache_key_parts.extend([f"{k}_{v}" for k, v in kwargs.items()])
            cache_key = ":".join(cache_key_parts)

            # Try to get from cache
            cached_result = redis_manager.cache_get(cache_key)
            if cached_result is not None:
                return cached_result

            # Execute function and cache result
            result = func(*args, **kwargs)
            redis_manager.cache_set(cache_key, result, expiration)
            return result

        return wrapper
    return decorator

# Decorator for Redis transactions
def redis_transaction():
    """Decorator for wrapping functions in Redis transactions"""
    def decorator(func):
        @wraps(func)
        def wrapper(*args, **kwargs):
            try:
                with redis_manager.transaction() as transaction:
                    result = func(*args, **kwargs, transaction=transaction)
                    transaction.execute()
                    return result
            except Exception as e:
                logger.error(f"Redis transaction failed: {e}")
                raise
        return wrapper
    return decorator
