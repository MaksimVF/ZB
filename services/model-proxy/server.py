# server.py
import os
import json
import logging
from concurrent import futures
import ssl
import grpc
# generated modules expected: model_pb2, model_pb2_grpc
try:
    import model_pb2, model_pb2_grpc
except Exception:
    # placeholders are fine for skeleton; generate protos to use
    model_pb2 = None
    model_pb2_grpc = None

# try optional import
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
    provider = provider_model.split("/")[0]
    try:
        litellm.api_key = PROVIDER_KEYS.get(provider)
        return completion(model=provider_model, messages=[{"role":"user","content":" ".join(messages)}], temperature=temperature, max_tokens=max_tokens, stream=False)
    except Exception as e:
        logger.exception("litellm call failed")
        return {"text": "litellm error: "+str(e), "usage": {"total_tokens": 0}}

class ModelServicer:
    # will be wrapped when protos are generated
    def Generate(self, request, context):
        msgs = list(request.messages) if request and hasattr(request, "messages") else []
        text = " ".join(msgs) if msgs else "empty"
        if LITELLM:
            prov = request.model or "local"
            try:
                res = call_litellm(f"{prov}/{request.model}", msgs, request.temperature, request.max_tokens)
                text = ""
                if isinstance(res, dict):
                    if "choices" in res and len(res["choices"])>0:
                        for c in res["choices"]:
                            text += c.get("message",{}).get("content","") or c.get("text","")
                    else:
                        text = res.get("text", str(res))
                else:
                    text = str(res)
            except Exception as e:
                logger.exception("error")
                text = "error: "+str(e)

        # Create and return proper GenResponse
        tokens_used = max(1, len(text) // 4)  # Simple token estimation
        return model_pb2.GenResponse(
            request_id=request.request_id if request and hasattr(request, "request_id") else "",
            text=text,
            tokens_used=tokens_used
        )

    def GenerateStream(self, request, context):
        """Streaming version of Generate that yields multiple responses"""
        msgs = list(request.messages) if request and hasattr(request, "messages") else []
        text = " ".join(msgs) if msgs else "empty"

        # For streaming, we'll split the response into chunks
        if LITELLM:
            prov = request.model or "local"
            try:
                res = call_litellm(f"{prov}/{request.model}", msgs, request.temperature, request.max_tokens)
                if isinstance(res, dict):
                    if "choices" in res and len(res["choices"])>0:
                        # Yield each choice as a separate response
                        for c in res["choices"]:
                            chunk_text = c.get("message",{}).get("content","") or c.get("text","")
                            if chunk_text:
                                tokens_used = max(1, len(chunk_text) // 4)
                                yield model_pb2.GenResponse(
                                    request_id=request.request_id if request and hasattr(request, "request_id") else "",
                                    text=chunk_text,
                                    tokens_used=tokens_used
                                )
                    else:
                        # Single response
                        text = res.get("text", str(res))
                        tokens_used = max(1, len(text) // 4)
                        yield model_pb2.GenResponse(
                            request_id=request.request_id if request and hasattr(request, "request_id") else "",
                            text=text,
                            tokens_used=tokens_used
                        )
                else:
                    # Fallback for non-dict responses
                    text = str(res)
                    tokens_used = max(1, len(text) // 4)
                    yield model_pb2.GenResponse(
                        request_id=request.request_id if request and hasattr(request, "request_id") else "",
                        text=text,
                        tokens_used=tokens_used
                    )
            except Exception as e:
                logger.exception("error")
                error_text = "error: "+str(e)
                yield model_pb2.GenResponse(
                    request_id=request.request_id if request and hasattr(request, "request_id") else "",
                    text=error_text,
                    tokens_used=1
                )
        else:
            # Fallback echo for non-litellm case
            tokens_used = max(1, len(text) // 4)
            yield model_pb2.GenResponse(
                request_id=request.request_id if request and hasattr(request, "request_id") else "",
                text=f"proxy-echo: {text}",
                tokens_used=tokens_used
            )

def get_server_credentials():
    with open("/workspace/ZB/certs/model-proxy.pem", "rb") as f:
        cert_chain = f.read()
    with open("/workspace/ZB/certs/model-proxy-key.pem", "rb") as f:
        private_key = f.read()
    with open("/workspace/ZB/certs/ca.pem", "rb") as f:
        ca_cert = f.read()

    return grpc.ssl_server_credentials(
        ((private_key, cert_chain),),
        root_certificates=ca_cert,
        require_client_auth=True  # Обязательная взаимная аутентификация
    )

def serve():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    model_pb2_grpc.add_ModelServiceServicer_to_server(ModelServicer(), server)

    port = os.getenv("GRPC_PORT", "50061")
    server_credentials = get_server_credentials()
    server.add_secure_port(f"[::]:{port}", server_credentials)

    logger.info(f"model-proxy mTLS gRPC server starting on :{port}")
    server.start()
    server.wait_for_termination()

if __name__ == "__main__":
    serve()
