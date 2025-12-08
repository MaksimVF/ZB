

#!/usr/bin/env python3
"""
Performance test script for billing service optimization
"""

import os
import sys
import time
import grpc
import logging
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime

import billing_pb2
import billing_pb2_grpc

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("performance_test")

# Configuration
TEST_USER_ID = "test_user_123"
TEST_MODEL = "gpt-4o"
TEST_ENDPOINT = "chat"
TEST_TOKENS = 1000
TEST_COST = 0.005
TEST_ITERATIONS = 100
TEST_CONCURRENCY = 10
JWT_TOKEN = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoidGVzdF91c2VyXzEyMyIsImV4cCI6MTY4MzQwNzE5Mn0.1Qa2i7jKfXnYlLKQxZJZJZJZJZJZJZJZJZJZJZJZJZJ"

def create_billing_stub(host='localhost', port=50052):
    """Create gRPC stub for billing service"""
    channel = grpc.insecure_channel(f'{host}:{port}')
    return billing_pb2_grpc.BillingServiceStub(channel)

def test_charge(stub, user_id, model, tokens, cost):
    """Test charge operation"""
    try:
        response = stub.Charge(
            billing_pb2.BillRequest(
                user_id=user_id,
                model=model,
                tokens_used=tokens,
                cost=cost
            ),
            metadata=[('authorization', JWT_TOKEN)]
        )
        return response, None
    except grpc.RpcError as e:
        return None, e

def test_reserve(stub, user_id, model, endpoint, input_tokens, output_tokens):
    """Test reserve operation"""
    try:
        response = stub.Reserve(
            billing_pb2.ReserveRequest(
                user_id=user_id,
                model=model,
                endpoint=endpoint,
                input_tokens_estimate=input_tokens,
                output_tokens_estimate=output_tokens
            ),
            metadata=[('authorization', JWT_TOKEN)]
        )
        return response, None
    except grpc.RpcError as e:
        return None, e

def test_commit(stub, reservation_id, input_tokens, output_tokens):
    """Test commit operation"""
    try:
        response = stub.Commit(
            billing_pb2.CommitRequest(
                reservation_id=reservation_id,
                input_tokens_actual=input_tokens,
                output_tokens_actual=output_tokens
            ),
            metadata=[('authorization', JWT_TOKEN)]
        )
        return response, None
    except grpc.RpcError as e:
        return None, e

def test_get_balance(stub, user_id):
    """Test get balance operation"""
    try:
        response = stub.GetBalance(
            billing_pb2.BalanceRequest(user_id=user_id),
            metadata=[('authorization', JWT_TOKEN)]
        )
        return response, None
    except grpc.RpcError as e:
        return None, e

def run_performance_test(host='localhost', port=50052, label="Original"):
    """Run performance test against billing service"""
    logger.info(f"Starting performance test for {label} service on {host}:{port}")

    stub = create_billing_stub(host, port)

    # Test parameters
    start_time = time.time()
    success_count = 0
    error_count = 0
    total_time = 0

    # Run sequential tests
    for i in range(TEST_ITERATIONS):
        iteration_start = time.time()

        # Test charge operation
        charge_response, charge_error = test_charge(stub, TEST_USER_ID, TEST_MODEL, TEST_TOKENS, TEST_COST)
        if charge_error:
            logger.error(f"Charge error: {charge_error}")
            error_count += 1
        else:
            success_count += 1

        # Test reserve operation
        reserve_response, reserve_error = test_reserve(stub, TEST_USER_ID, TEST_MODEL, TEST_ENDPOINT, TEST_TOKENS, TEST_TOKENS)
        if reserve_error:
            logger.error(f"Reserve error: {reserve_error}")
            error_count += 1
        else:
            success_count += 1
            reservation_id = reserve_response.reservation_id

            # Test commit operation
            commit_response, commit_error = test_commit(stub, reservation_id, TEST_TOKENS, TEST_TOKENS)
            if commit_error:
                logger.error(f"Commit error: {commit_error}")
                error_count += 1
            else:
                success_count += 1

        # Test balance operation
        balance_response, balance_error = test_get_balance(stub, TEST_USER_ID)
        if balance_error:
            logger.error(f"Balance error: {balance_error}")
            error_count += 1
        else:
            success_count += 1

        iteration_time = time.time() - iteration_start
        total_time += iteration_time

    total_elapsed = time.time() - start_time
    avg_time_per_operation = total_time / (TEST_ITERATIONS * 4)  # 4 operations per iteration

    logger.info(f"{label} Performance Test Results:")
    logger.info(f"  Total time: {total_elapsed:.2f}s")
    logger.info(f"  Operations: {TEST_ITERATIONS * 4}")
    logger.info(f"  Success rate: {success_count}/{TEST_ITERATIONS * 4} ({success_count/(TEST_ITERATIONS * 4)*100:.1f}%)")
    logger.info(f"  Error rate: {error_count}/{TEST_ITERATIONS * 4} ({error_count/(TEST_ITERATIONS * 4)*100:.1f}%)")
    logger.info(f"  Avg time per operation: {avg_time_per_operation*1000:.2f}ms")
    logger.info(f"  Operations per second: {TEST_ITERATIONS * 4 / total_elapsed:.2f}")

    return {
        'label': label,
        'total_time': total_elapsed,
        'operations': TEST_ITERATIONS * 4,
        'success_count': success_count,
        'error_count': error_count,
        'avg_time_per_op': avg_time_per_operation,
        'ops_per_sec': TEST_ITERATIONS * 4 / total_elapsed
    }

def run_concurrent_test(host='localhost', port=50052, label="Original"):
    """Run concurrent performance test"""
    logger.info(f"Starting concurrent performance test for {label} service on {host}:{port}")

    stub = create_billing_stub(host, port)

    def worker(worker_id):
        """Worker function for concurrent testing"""
        results = []
        for i in range(TEST_ITERATIONS // TEST_CONCURRENCY):
            # Test charge operation
            charge_response, charge_error = test_charge(stub, f"{TEST_USER_ID}_{worker_id}", TEST_MODEL, TEST_TOKENS, TEST_COST)
            if charge_error:
                results.append(('charge', charge_error))
            else:
                results.append(('charge', None))

            # Test balance operation
            balance_response, balance_error = test_get_balance(stub, f"{TEST_USER_ID}_{worker_id}")
            if balance_error:
                results.append(('balance', balance_error))
            else:
                results.append(('balance', None))
        return results

    start_time = time.time()
    with ThreadPoolExecutor(max_workers=TEST_CONCURRENCY) as executor:
        futures = [executor.submit(worker, i) for i in range(TEST_CONCURRENCY)]

        success_count = 0
        error_count = 0

        for future in as_completed(futures):
            try:
                results = future.result()
                for op_type, error in results:
                    if error:
                        error_count += 1
                    else:
                        success_count += 1
            except Exception as e:
                logger.error(f"Worker failed: {e}")
                error_count += 1

    total_elapsed = time.time() - start_time
    total_operations = TEST_ITERATIONS * 2  # 2 operations per iteration
    ops_per_sec = total_operations / total_elapsed

    logger.info(f"{label} Concurrent Performance Test Results:")
    logger.info(f"  Total time: {total_elapsed:.2f}s")
    logger.info(f"  Operations: {total_operations}")
    logger.info(f"  Success rate: {success_count}/{total_operations} ({success_count/total_operations*100:.1f}%)")
    logger.info(f"  Error rate: {error_count}/{total_operations} ({error_count/total_operations*100:.1f}%)")
    logger.info(f"  Operations per second: {ops_per_sec:.2f}")

    return {
        'label': label,
        'total_time': total_elapsed,
        'operations': total_operations,
        'success_count': success_count,
        'error_count': error_count,
        'ops_per_sec': ops_per_sec
    }

def compare_performance(original_results, optimized_results):
    """Compare performance results"""
    logger.info("\n=== PERFORMANCE COMPARISON ===")

    improvement = (original_results['avg_time_per_op'] - optimized_results['avg_time_per_op']) / original_results['avg_time_per_op'] * 100
    ops_improvement = (optimized_results['ops_per_sec'] - original_results['ops_per_sec']) / original_results['ops_per_sec'] * 100

    logger.info(f"Original Service:")
    logger.info(f"  Avg time per operation: {original_results['avg_time_per_op']*1000:.2f}ms")
    logger.info(f"  Operations per second: {original_results['ops_per_sec']:.2f}")
    logger.info(f"  Success rate: {original_results['success_count']}/{original_results['operations']} ({original_results['success_count']/original_results['operations']*100:.1f}%)")

    logger.info(f"Optimized Service:")
    logger.info(f"  Avg time per operation: {optimized_results['avg_time_per_op']*1000:.2f}ms")
    logger.info(f"  Operations per second: {optimized_results['ops_per_sec']:.2f}")
    logger.info(f"  Success rate: {optimized_results['success_count']}/{optimized_results['operations']} ({optimized_results['success_count']/optimized_results['operations']*100:.1f}%)")

    logger.info(f"\nImprovements:")
    logger.info(f"  Time per operation: {improvement:.1f}% faster")
    logger.info(f"  Throughput: {ops_improvement:.1f}% higher")

if __name__ == "__main__":
    logger.info("Billing Service Performance Test")
    logger.info("================================")

    # Test original service
    try:
        original_results = run_performance_test(port=50052, label="Original")
    except Exception as e:
        logger.error(f"Original service test failed: {e}")
        original_results = None

    # Test optimized service
    try:
        optimized_results = run_performance_test(port=50058, label="Optimized")
    except Exception as e:
        logger.error(f"Optimized service test failed: {e}")
        optimized_results = None

    # Compare results if both tests succeeded
    if original_results and optimized_results:
        compare_performance(original_results, optimized_results)

    # Run concurrent tests
    logger.info("\n=== CONCURRENT PERFORMANCE TESTS ===")

    try:
        original_concurrent = run_concurrent_test(port=50052, label="Original")
    except Exception as e:
        logger.error(f"Original concurrent test failed: {e}")
        original_concurrent = None

    try:
        optimized_concurrent = run_concurrent_test(port=50058, label="Optimized")
    except Exception as e:
        logger.error(f"Optimized concurrent test failed: {e}")
        optimized_concurrent = None

    # Compare concurrent results
    if original_concurrent and optimized_concurrent:
        logger.info("\n=== CONCURRENT PERFORMANCE COMPARISON ===")
        concurrency_improvement = (optimized_concurrent['ops_per_sec'] - original_concurrent['ops_per_sec']) / original_concurrent['ops_per_sec'] * 100

        logger.info(f"Original Service: {original_concurrent['ops_per_sec']:.2f} ops/sec")
        logger.info(f"Optimized Service: {optimized_concurrent['ops_per_sec']:.2f} ops/sec")
        logger.info(f"Concurrency Improvement: {concurrency_improvement:.1f}%")

