


#!/usr/bin/env python3
"""
Integration test for Bifrost-based model proxy replacement
Tests both HTTP and gRPC interfaces
"""

import requests
import grpc
import time
import json
from concurrent.futures import ThreadPoolExecutor

# Import generated gRPC classes
import sys
import os

# Add the proto directory to Python path for imports
sys.path.append(os.path.join(os.path.dirname(__file__), '../../proto'))

import model_pb2
import model_pb2_grpc

def test_http_api():
    """Test Bifrost HTTP API directly"""
    print("Testing Bifrost HTTP API...")

    url = "http://localhost:8100/v1/chat/completions"

    payload = {
        "model": "openai/gpt-3.5-turbo",
        "messages": [
            {"role": "user", "content": "Hello, this is a test from Bifrost integration!"}
        ],
        "temperature": 0.7,
        "max_tokens": 50
    }

    try:
        response = requests.post(
            url,
            json=payload,
            headers={"Content-Type": "application/json"},
            timeout=30
        )

        if response.status_code == 200:
            result = response.json()
            print(f"✅ HTTP API test passed. Response: {result['choices'][0]['message']['content'][:50]}...")
            return True
        else:
            print(f"❌ HTTP API test failed. Status: {response.status_code}, Error: {response.text}")
            return False

    except Exception as e:
        print(f"❌ HTTP API test failed with exception: {e}")
        return False

def test_grpc_api():
    """Test gRPC adapter that translates to Bifrost"""
    print("Testing gRPC API adapter...")

    try:
        # Create gRPC channel
        channel = grpc.insecure_channel('localhost:50061')
        stub = model_pb2_grpc.ModelServiceStub(channel)

        # Create request
        request = model_pb2.GenRequest(
            request_id="test-123",
            model="openai/gpt-3.5-turbo",
            messages=["Hello, this is a gRPC test!"],
            temperature=0.7,
            max_tokens=50,
            stream=False
        )

        # Call Generate
        response = stub.Generate(request)

        print(f"✅ gRPC API test passed. Response: {response.text[:50]}...")
        return True

    except Exception as e:
        print(f"❌ gRPC API test failed with exception: {e}")
        return False

def test_streaming_grpc():
    """Test gRPC streaming interface"""
    print("Testing gRPC streaming API...")

    try:
        # Create gRPC channel
        channel = grpc.insecure_channel('localhost:50061')
        stub = model_pb2_grpc.ModelServiceStub(channel)

        # Create streaming request
        request = model_pb2.GenRequest(
            request_id="test-stream-123",
            model="openai/gpt-3.5-turbo",
            messages=["Hello, this is a streaming test!"],
            temperature=0.7,
            max_tokens=50,
            stream=True
        )

        # Call GenerateStream
        response_stream = stub.GenerateStream(request)

        # Collect responses
        full_text = ""
        for response in response_stream:
            full_text += response.text

        print(f"✅ gRPC streaming test passed. Full text: {full_text[:50]}...")
        return True

    except Exception as e:
        print(f"❌ gRPC streaming test failed with exception: {e}")
        return False

def test_batch_grpc():
    """Test gRPC batch processing"""
    print("Testing gRPC batch API...")

    try:
        # Create gRPC channel
        channel = grpc.insecure_channel('localhost:50061')
        stub = model_pb2_grpc.ModelServiceStub(channel)

        # Create batch request with multiple individual requests
        requests = [
            model_pb2.GenRequest(
                request_id=f"batch-{i}",
                model="openai/gpt-3.5-turbo",
                messages=[f"Batch request number {i}"],
                temperature=0.7,
                max_tokens=30,
                stream=False
            ) for i in range(3)
        ]

        batch_request = model_pb2.BatchGenRequest(requests=requests)

        # Call BatchGenerate
        response = stub.BatchGenerate(batch_request)

        print(f"✅ gRPC batch test passed. Got {len(response.responses)} responses")
        return True

    except Exception as e:
        print(f"❌ gRPC batch test failed with exception: {e}")
        return False

def test_concurrent_requests():
    """Test concurrent request handling"""
    print("Testing concurrent requests...")

    def make_request(i):
        try:
            url = "http://localhost:8100/v1/chat/completions"
            payload = {
                "model": "openai/gpt-3.5-turbo",
                "messages": [{"role": "user", "content": f"Concurrent test request {i}"}],
                "temperature": 0.7,
                "max_tokens": 20
            }

            response = requests.post(url, json=payload, timeout=10)
            return response.status_code == 200
        except:
            return False

    # Test with 10 concurrent requests
    with ThreadPoolExecutor(max_workers=10) as executor:
        results = list(executor.map(make_request, range(10)))

    success_count = sum(results)
    print(f"✅ Concurrent test: {success_count}/10 requests succeeded")

    return success_count >= 8  # Allow some failures for robustness

def main():
    """Run all integration tests"""
    print("🚀 Starting Bifrost integration tests...")
    print("=" * 50)

    # Wait a bit for services to be ready
    print("Waiting for services to initialize...")
    time.sleep(10)

    tests = [
        ("HTTP API", test_http_api),
        ("gRPC API", test_grpc_api),
        ("gRPC Streaming", test_streaming_grpc),
        ("gRPC Batch", test_batch_grpc),
        ("Concurrent Requests", test_concurrent_requests),
    ]

    passed = 0
    total = len(tests)

    for test_name, test_func in tests:
        print(f"\n🧪 Running {test_name} test...")
        if test_func():
            passed += 1
        else:
            print(f"❌ {test_name} test failed")

    print("\n" + "=" * 50)
    print(f"📊 Test Results: {passed}/{total} tests passed")

    if passed == total:
        print("🎉 All tests passed! Bifrost integration is working correctly.")
        return 0
    else:
        print("⚠️  Some tests failed. Please check the implementation.")
        return 1

if __name__ == "__main__":
    exit(main())


