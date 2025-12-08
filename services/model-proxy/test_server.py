
#!/usr/bin/env python3
"""
Test script for the model-proxy gRPC server
"""
import grpc
import sys
import os

# Add the current directory to Python path to import generated modules
sys.path.append(os.path.dirname(os.path.abspath(__file__)))

try:
    import model_pb2
    import model_pb2_grpc
except ImportError as e:
    print(f"Failed to import gRPC modules: {e}")
    sys.exit(1)

def run_test():
    # Create a gRPC channel to connect to the server (using insecure port for testing)
    with grpc.insecure_channel('localhost:50062') as channel:
        # Create a stub (client)
        stub = model_pb2_grpc.ModelServiceStub(channel)

        # Create a test request
        request = model_pb2.GenRequest(
            request_id="test123",
            model="test-model",
            messages=["Hello, how are you?", "What is the weather today?"]
        )

        try:
            # Test the Generate method
            print("Testing Generate method...")
            response = stub.Generate(request)
            print(f"Generate response: {response}")
            print(f"Request ID: {response.request_id}")
            print(f"Text: {response.text}")
            print(f"Tokens used: {response.tokens_used}")

            # Test the GenerateStream method
            print("\nTesting GenerateStream method...")
            stream_response = stub.GenerateStream(request)
            for i, chunk in enumerate(stream_response):
                print(f"Stream chunk {i+1}:")
                print(f"  Request ID: {chunk.request_id}")
                print(f"  Text: {chunk.text}")
                print(f"  Tokens used: {chunk.tokens_used}")

        except grpc.RpcError as e:
            print(f"gRPC error: {e.code()} - {e.details()}")
        except Exception as e:
            print(f"Error: {str(e)}")

if __name__ == "__main__":
    run_test()
