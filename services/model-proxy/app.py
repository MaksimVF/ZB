from fastapi import FastAPI
from pydantic import BaseModel
import uvicorn
import os

# Minimal model-proxy: echo or simple litellm usage if available

app = FastAPI(title="Model Proxy")

class Msg(BaseModel):
    role: str
    content: str

class Req(BaseModel):
    model: str
    messages: list[Msg]
    temperature: float = 0.7
    max_tokens: int = 1024

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
