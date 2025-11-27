# billing service (python) server.py
import os
import json
import logging
from concurrent import futures
import grpc
try:
    import billing_pb2_grpc, billing_pb2
except Exception:
    billing_pb2 = None
    billing_pb2_grpc = None

import redis

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("billing")

def serve():
    r = redis.from_url(os.getenv("REDIS_URL","redis://localhost:6379"))
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=4))
    # add servicer after proto generation
    addr = os.getenv("BILL_ADDR", "[::]:50052")
    server.add_insecure_port(addr)
    logger.info("Billing listening %s", addr)
    server.start()
    server.wait_for_termination()

if __name__ == "__main__":
    serve()
