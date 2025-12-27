from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, validator
import uvicorn
import os
import json
import logging
from typing import List

# Minimal model-proxy: echo or simple litellm usage if available

app = FastAPI(title="Model Proxy")

logger = logging.getLogger("model-proxy-fastapi")

# Get provider keys from secrets service
def get_provider_keys_from_secrets():
    """Fetch provider API keys from secrets service"""
    try:
        # Implement actual gRPC call to secrets-service
        # For now, we'll use environment variable as fallback
        provider_keys = json.loads(os.getenv("PROVIDER_KEYS", "{}"))

        # Validate that all required keys are present
        required_providers = ["openai", "anthropic", "mistral", "groq"]
        for provider in required_providers:
            if provider not in provider_keys:
                logger.warning(f"Missing API key for provider: {provider}")
            elif not provider_keys[provider]:
                logger.warning(f"Empty API key for provider: {provider}")

        # Mask API keys in logs
        masked_keys = {k: "*****" if v else "" for k, v in provider_keys.items()}
        logger.info(f"Loaded provider keys: {masked_keys}")

        return provider_keys
    except Exception as e:
        logger.error(f"Failed to fetch provider keys from secrets service: {e}")
        return {}

PROVIDER_KEYS = get_provider_keys_from_secrets()

class Msg(BaseModel):
    role: str
    content: str

    @validator('role')
    def validate_role(cls, v):
        if v not in ['user', 'assistant', 'system']:
            raise ValueError('Role must be user, assistant, or system')
        return v

    @validator('content')
    def validate_content(cls, v):
        if not v or not isinstance(v, str):
            raise ValueError('Content must be a non-empty string')
        if len(v) > 10000:  # Reasonable limit
            raise ValueError('Content too long')
        return v

class Req(BaseModel):
    model: str
    messages: List[Msg]
    temperature: float = 0.7
    max_tokens: int = 1024

    @validator('model')
    def validate_model(cls, v):
        if not v or not isinstance(v, str):
            raise ValueError('Model must be a non-empty string')
        parts = v.split('/')
        if len(parts) != 2:
            raise ValueError('Model must be in format provider/model')
        provider = parts[0]
        if provider not in PROVIDER_KEYS:
            raise ValueError(f'Provider {provider} not configured')
        return v

    @validator('temperature')
    def validate_temperature(cls, v):
        if not (0.0 <= v <= 1.0):
            raise ValueError('Temperature must be between 0.0 and 1.0')
        return v

    @validator('max_tokens')
    def validate_max_tokens(cls, v):
        if not isinstance(v, int) or v <= 0 or v > 4096:
            raise ValueError('max_tokens must be a positive integer <= 4096')
        return v

@app.post("/v1/generate")
async def generate(req: Req):
    # Use litellm if available, otherwise fallback to echo
    try:
        import litellm
        from litellm import completion

        # Convert messages to litellm format
        messages = [{"role": m.role, "content": m.content} for m in req.messages]

        # Call litellm
        response = completion(
            model=req.model,
            messages=messages,
            temperature=req.temperature,
            max_tokens=req.max_tokens
        )

        # Extract response text and usage
        if isinstance(response, dict):
            text = response.get("choices", [{"text": "no response"}])[0].get("text", "no response")
            usage = response.get("usage", {"total_tokens": len(text) // 4 + 1})
            return {"text": text, "usage": usage}

    except ImportError:
        # Fallback to echo if litellm not available
        text = " ".join(m.content for m in req.messages)
        if not text:
            text = "empty"
        return {"text": f"proxy-echo: {text}", "usage": {"total_tokens": max(1,len(text)//4)} }

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=int(os.getenv("PORT", "8100")))
