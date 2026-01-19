



#!/usr/bin/env python3
"""
Performance benchmarking script to compare LiteLLM vs Bifrost implementations
"""

import requests
import grpc
import time
import json
import statistics
import threading
import queue
from concurrent.futures import ThreadPoolExecutor, as_completed
from typing import List, Dict, Any

# Import generated gRPC classes
import sys
import os

# Add the proto directory to Python path for imports
sys.path.append(os.path.join(os.path.dirname(__file__), '../../proto'))

import model_pb2
import model_pb2_grpc

class BenchmarkResult:
    def __init__(self):
        self.success_count = 0
        self.failure_count = 0
        self.latencies = []
        self.start_time = None
        self.end_time = None
        self.total_requests = 0

    def start(self):
        self.start_time = time.time()

    def end(self):
        self.end_time = time.time()

    def add_result(self, success: bool, latency: float):
        self.total_requests += 1
        if success:
            self.success_count += 1
            self.latencies.append(latency)
        else:
            self.failure_count += 1

    def get_stats(self) -> Dict[str, Any]:
        if not self.latencies:
            return {
                "success_rate": 0.0,
                "avg_latency": 0.0,
                "min_latency": 0.0,
                "max_latency": 0.0,
                "stddev_latency": 0.0,
                "rps": 0.0,
                "total_time": 0.0,
                "success_count": self.success_count,
                "failure_count": self.failure_count,
                "total_requests": self.total_requests
            }

        total_time = self.end_time - self.start_time if self.end_time and self.start_time else 0
        rps = self.success_count / total_time if total_time > 0 else 0

        return {
            "success_rate": self.success_count / self.total_requests * 100,
            "avg_latency": statistics.mean(self.latencies),
            "min_latency": min(self.latencies),
            "max_latency": max(self.latencies),
            "stddev_latency": statistics.stdev(self.latencies) if len(self.latencies) > 1 else 0,
            "rps": rps,
            "total_time": total_time,
            "success_count": self.success_count,
            "failure_count": self.failure_count,
            "total_requests": self.total_requests
        }

def benchmark_http_api(url: str, test_name: str, concurrency: int, requests_per_worker: int) -> BenchmarkResult:
    """Benchmark HTTP API performance"""
    print(f"🚀 Starting {test_name} benchmark...")
    print(f"   Concurrency: {concurrency}, Requests per worker: {requests_per_worker}")

    result = BenchmarkResult()
    result.start()

    def worker(worker_id: int):
        worker_results = []
        for i in range(requests_per_worker):
            try:
                start_time = time.time()

                payload = {
                    "model": "openai/gpt-3.5-turbo",
                    "messages": [{"role": "user", "content": f"Benchmark request {worker_id}-{i}"}],
                    "temperature": 0.7,
                    "max_tokens": 30
                }

                response = requests.post(
                    url,
                    json=payload,
                    headers={"Content-Type": "application/json"},
                    timeout=10
                )

                latency = time.time() - start_time
                success = response.status_code == 200
                worker_results.append((success, latency))

            except Exception as e:
                worker_results.append((False, time.time() - start_time))

        return worker_results

    # Run concurrent workers
    with ThreadPoolExecutor(max_workers=concurrency) as executor:
        futures = [executor.submit(worker, i) for i in range(concurrency)]

        for future in as_completed(futures):
            for success, latency in future.result():
                result.add_result(success, latency)

    result.end()
    return result

def benchmark_grpc_api(address: str, test_name: str, concurrency: int, requests_per_worker: int) -> BenchmarkResult:
    """Benchmark gRPC API performance"""
    print(f"🚀 Starting {test_name} benchmark...")
    print(f"   Concurrency: {concurrency}, Requests per worker: {requests_per_worker}")

    result = BenchmarkResult()
    result.start()

    def worker(worker_id: int):
        worker_results = []

        # Create gRPC channel (reused for all requests in this worker)
        channel = grpc.insecure_channel(address)
        stub = model_pb2_grpc.ModelServiceStub(channel)

        for i in range(requests_per_worker):
            try:
                start_time = time.time()

                request = model_pb2.GenRequest(
                    request_id=f"benchmark-{worker_id}-{i}",
                    model="openai/gpt-3.5-turbo",
                    messages=[f"Benchmark gRPC request {worker_id}-{i}"],
                    temperature=0.7,
                    max_tokens=30,
                    stream=False
                )

                response = stub.Generate(request)
                latency = time.time() - start_time
                success = response is not None and response.text != ""
                worker_results.append((success, latency))

            except Exception as e:
                worker_results.append((False, time.time() - start_time))

        # Close channel
        channel.close()
        return worker_results

    # Run concurrent workers
    with ThreadPoolExecutor(max_workers=concurrency) as executor:
        futures = [executor.submit(worker, i) for i in range(concurrency)]

        for future in as_completed(futures):
            for success, latency in future.result():
                result.add_result(success, latency)

    result.end()
    return result

def run_comparison_benchmark():
    """Run comprehensive comparison between old and new implementations"""
    print("🔬 Running Bifrost vs LiteLLM Performance Comparison")
    print("=" * 60)

    # Configuration
    test_configs = [
        {"name": "Low Load", "concurrency": 5, "requests_per_worker": 10},
        {"name": "Medium Load", "concurrency": 20, "requests_per_worker": 25},
        {"name": "High Load", "concurrency": 50, "requests_per_worker": 50},
    ]

    # Test both HTTP and gRPC interfaces
    interfaces = [
        {"name": "HTTP API", "test_func": benchmark_http_api, "urls": {
            "bifrost": "http://localhost:8100/v1/chat/completions",
            "litellm": "http://localhost:8100/v1/generate"
        }},
        {"name": "gRPC API", "test_func": benchmark_grpc_api, "urls": {
            "bifrost": "localhost:50061",
            "litellm": "localhost:50061"
        }}
    ]

    all_results = {}

    for interface in interfaces:
        print(f"\n📊 Testing {interface['name']} Interface")
        print("-" * 40)

        interface_results = {}

        for config in test_configs:
            print(f"\n🧪 {config['name']} Test")

            # Test Bifrost
            print("   Testing Bifrost...")
            bifrost_result = interface["test_func"](
                interface["urls"]["bifrost"],
                f"Bifrost {interface['name']} {config['name']}",
                config["concurrency"],
                config["requests_per_worker"]
            )

            # Test LiteLLM (if available)
            print("   Testing LiteLLM...")
            try:
                litellm_result = interface["test_func"](
                    interface["urls"]["litellm"],
                    f"LiteLLM {interface['name']} {config['name']}",
                    config["concurrency"],
                    config["requests_per_worker"]
                )
            except Exception as e:
                print(f"   ⚠️  LiteLLM test failed: {e}")
                litellm_result = None

            # Store results
            interface_results[config["name"]] = {
                "bifrost": bifrost_result.get_stats(),
                "litellm": litellm_result.get_stats() if litellm_result else None
            }

            # Print comparison
            print(f"\n   📈 {config['name']} Results:")
            print(f"   Bifrost: {bifrost_result.success_count}/{bifrost_result.total_requests} successes")
            print(f"   Avg Latency: {bifrost_result.get_stats()['avg_latency']*1000:.2f}ms")
            print(f"   RPS: {bifrost_result.get_stats()['rps']:.2f}")

            if litellm_result:
                print(f"   LiteLLM: {litellm_result.success_count}/{litellm_result.total_requests} successes")
                print(f"   Avg Latency: {litellm_result.get_stats()['avg_latency']*1000:.2f}ms")
                print(f"   RPS: {litellm_result.get_stats()['rps']:.2f}")

                # Calculate improvement
                if litellm_result.get_stats()['avg_latency'] > 0:
                    latency_improvement = (litellm_result.get_stats()['avg_latency'] - bifrost_result.get_stats()['avg_latency']) / litellm_result.get_stats()['avg_latency'] * 100
                    print(f"   🚀 Latency Improvement: {latency_improvement:.1f}%")

                if litellm_result.get_stats()['rps'] > 0:
                    rps_improvement = (bifrost_result.get_stats()['rps'] - litellm_result.get_stats()['rps']) / litellm_result.get_stats()['rps'] * 100
                    print(f"   🚀 RPS Improvement: {rps_improvement:.1f}%")

        all_results[interface["name"]] = interface_results

    return all_results

def print_summary(results: Dict[str, Any]):
    """Print a summary of benchmark results"""
    print("\n" + "=" * 60)
    print("📋 PERFORMANCE BENCHMARK SUMMARY")
    print("=" * 60)

    for interface_name, interface_results in results.items():
        print(f"\n📊 {interface_name} Interface Summary")
        print("-" * 40)

        for test_name, test_results in interface_results.items():
            bifrost_stats = test_results["bifrost"]
            litellm_stats = test_results["litellm"]

            print(f"\n{test_name}:")
            print(f"  Bifrost: {bifrost_stats['success_rate']:.1f}% success, {bifrost_stats['avg_latency']*1000:.2f}ms avg latency, {bifrost_stats['rps']:.2f} RPS")

            if litellm_stats:
                print(f"  LiteLLM: {litellm_stats['success_rate']:.1f}% success, {litellm_stats['avg_latency']*1000:.2f}ms avg latency, {litellm_stats['rps']:.2f} RPS")

                # Calculate improvements
                if litellm_stats['avg_latency'] > 0:
                    latency_improvement = (litellm_stats['avg_latency'] - bifrost_stats['avg_latency']) / litellm_stats['avg_latency'] * 100
                    print(f"  🚀 Latency: {latency_improvement:.1f}% better")

                if litellm_stats['rps'] > 0:
                    rps_improvement = (bifrost_stats['rps'] - litellm_stats['rps']) / litellm_stats['rps'] * 100
                    print(f"  🚀 Throughput: {rps_improvement:.1f}% better")

def save_results_to_file(results: Dict[str, Any], filename: str = "benchmark_results.json"):
    """Save benchmark results to a JSON file"""
    with open(filename, 'w') as f:
        json.dump(results, f, indent=2)
    print(f"\n💾 Benchmark results saved to {filename}")

def main():
    """Run performance benchmarking"""
    print("🔬 Bifrost vs LiteLLM Performance Benchmark")
    print("This script compares the performance of the new Bifrost-based model proxy")
    print("with the original LiteLLM implementation.")
    print()
    print("Note: For accurate results, ensure both services are running and")
    print("      no other load is on the system during testing.")
    print()

    # Wait for services to be ready
    print("Waiting for services to initialize...")
    time.sleep(15)

    # Run benchmarks
    results = run_comparison_benchmark()

    # Print summary
    print_summary(results)

    # Save results
    save_results_to_file(results)

    print("\n🎉 Benchmarking complete!")
    print("Check the results to see the performance improvements from Bifrost!")

if __name__ == "__main__":
    main()



