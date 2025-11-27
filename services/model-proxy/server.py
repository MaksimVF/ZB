# server.py
import os, json, logging
from concurrent import futures
import grpc

# После генерации protobuf разместятся model_pb2.py и model_pb2_grpc.py
try:
    import model_pb2, model_pb2_grpc
except Exception:
    model_pb2 = None
    model_pb2_grpc = None

# Try liteLLM
try:
    import litellm
    from litellm import completion
    LITELLM = True
except Exception:
    LITELLM = False

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("model-proxy")

PROVIDER_KEYS = json.loads(os.getenv("PROVIDER_KEYS", "{}"))

def call_litellm(provider_model, messages, temperature, max_tokens):
    litellm.api_key = PROVIDER_KEYS.get(provider_model.split("/")[0])
    return completion(model=provider_model, messages=[{"role":"user","content":" ".join(messages)}],
                      temperature=temperature, max_tokens=max_tokens, stream=False)

class ModelServicer(model_pb2_grpc.ModelServiceServicer):
    def Generate(self, request, context):
        messages = list(request.messages)
        if LITELLM:
            try:
                provider_model = f"{request.model}/{request.model}" if "/" not in request.model else request.model
                res = call_litellm(provider_model, messages, request.temperature, request.max_tokens)
                text = ""
                tokens = 0
                if isinstance(res, dict):
                    if "choices" in res and len(res["choices"])>0:
                        for c in res["choices"]:
                            text += c.get("message",{}).get("content","") or c.get("text","")
                    else:
                        text = res.get("text", str(res))
                    tokens = int(res.get("usage", {}).get("total_tokens", 0) or 0)
                else:
                    text = str(res)
                return model_pb2.GenResponse(request_id=request.request_id, text=text, tokens_used=tokens)
            except Exception as e:
                logger.exception("litellm error")
                return model_pb2.GenResponse(request_id=request.request_id, text=f"error: {e}", tokens_used=0)
        # fallback echo
        text = "proxy-echo: " + " ".join(messages or [""])
        tokens = max(1, len(text)//4)
        return model_pb2.GenResponse(request_id=request.request_id, text=text, tokens_used=tokens)

    def GenerateStream(self, request, context):
        # simple stream wrapper: yield single GenResponse chunk
        resp = self.Generate(request, context)
        yield resp

def serve():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=8))
    model_pb2_grpc.add_ModelServiceServicer_to_server(ModelServicer(), server)
    addr = os.getenv("MODEL_ADDR", "[::]:50061")
    server.add_insecure_port(addr)
    logger.info("Model-proxy listening %s", addr)
    server.start()
    server.wait_for_termination()

if __name__ == "__main__":
    serve()
