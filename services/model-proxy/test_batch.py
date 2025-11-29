
import requests
import json

def test_batch_processing():
    """Test that the model-proxy can handle batch-like requests"""

    # Test data similar to what the batch processor would send
    test_data = {
        "model": "gpt-4o",
        "messages": [
            {"role": "user", "content": "Hello, how are you?"},
            {"role": "assistant", "content": "I'm fine, thank you!"},
            {"role": "user", "content": "What is the weather today?"}
        ],
        "temperature": 0.7,
        "max_tokens": 100
    }

    # Send request to model-proxy FastAPI endpoint
    response = requests.post(
        "http://localhost:8100/v1/generate",
        json=test_data
    )

    print("Status code:", response.status_code)
    print("Response:", response.json())

    # Verify we get a proper response
    assert response.status_code == 200
    assert "text" in response.json()
    assert "usage" in response.json()

    print("✅ Batch processing test passed!")

if __name__ == "__main__":
    test_batch_processing()
