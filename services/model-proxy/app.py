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
    # echo fallback
    text = " ".join(m.content for m in req.messages)
    if not text:
        text = "empty"
    return {"text": f"proxy-echo: {text}", "usage": {"total_tokens": max(1,len(text)//4)} }

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=int(os.getenv("PORT", "8100")))
