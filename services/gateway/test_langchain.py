








import os
import requests
import json
from langchain_openai import ChatOpenAI
from langchain_core.messages import HumanMessage

# Configuration
GATEWAY_URL = "http://localhost:8080"
API_KEY = "langchain-test-key-12345"

def test_gateway_health():
    """Test the gateway health endpoint"""
    response = requests.get(f"{GATEWAY_URL}/health")
    assert response.status_code == 200
    assert response.json() == {"status": "healthy"}
    print("✅ Health check passed")

def test_langchain_integration():
    """Test LangChain integration with the gateway"""
    # Initialize LangChain with our gateway
    llm = ChatOpenAI(
        base_url=f"{GATEWAY_URL}/v1/langchain",
        api_key=API_KEY,
        model="gpt-4",
        temperature=0.7,
        streaming=False,
    )

    # Test a simple request
    try:
        response = llm.invoke([HumanMessage(content="Hello, how are you?")])
        print(f"✅ LangChain integration test passed: {response.content}")
        return True
    except Exception as e:
        print(f"❌ LangChain integration test failed: {e}")
        return False

def test_direct_api():
    """Test direct API access"""
    headers = {
        "Authorization": f"Bearer {API_KEY}",
        "Content-Type": "application/json"
    }

    data = {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Hello!"}],
        "stream": False
    }

    response = requests.post(
        f"{GATEWAY_URL}/v1/langchain/chat/completions",
        headers=headers,
        json=data
    )

    if response.status_code == 200:
        print("✅ Direct API test passed")
        return True
    else:
        print(f"❌ Direct API test failed: {response.status_code} - {response.text}")
        return False

def test_streaming():
    """Test streaming functionality"""
    headers = {
        "Authorization": f"Bearer {API_KEY}",
        "Content-Type": "application/json"
    }

    data = {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Hello!"}],
        "stream": True
    }

    response = requests.post(
        f"{GATEWAY_URL}/v1/langchain/chat/completions",
        headers=headers,
        json=data,
        stream=True
    )

    if response.status_code == 200:
        # Process streaming response
        for line in response.iter_lines():
            if line:
                line = line.decode('utf-8')
                if line.startswith('data: '):
                    data = line[6:]
                    if data == '[DONE]':
                        break
                    print(f"Streaming data: {data}")
        print("✅ Streaming test passed")
        return True
    else:
        print(f"❌ Streaming test failed: {response.status_code} - {response.text}")
        return False

if __name__ == "__main__":
    print("🚀 Testing LangChain Gateway Integration")
    print("=" * 50)

    # Run tests
    test_gateway_health()
    test_direct_api()
    test_streaming()
    test_langchain_integration()

    print("=" * 50)
    print("🎉 All tests completed!")





