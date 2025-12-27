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

# Get provider keys from secrets service
def get_provider_keys_from_secrets():
    """Fetch provider API keys from secrets service"""
    try:
        # Implement actual gRPC call to secrets-service
        # For now, we'll use environment variable as fallback
        # In production, this should be replaced with:
        # 1. gRPC client to secrets-service
        # 2. Fetch secrets using proper authentication
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

def call_litellm(provider_model, messages, temperature, max_tokens):
    """Call LiteLLM with proper validation and error handling"""
    try:
        # Validate provider_model format
        if not provider_model or not isinstance(provider_model, str):
            raise ValueError("Invalid provider_model format")

        parts = provider_model.split("/")
        if len(parts) < 2:
            raise ValueError("provider_model must be in format 'provider/model'")

        provider = parts[0]
        model_name = parts[1]

        # Validate provider
        if provider not in PROVIDER_KEYS:
            raise ValueError(f"Provider '{provider}' is not configured")

        # Validate API key
        api_key = PROVIDER_KEYS.get(provider)
        if not api_key:
            raise ValueError(f"API key for provider '{provider}' is not configured")

        # Validate messages
        if not messages or not isinstance(messages, list):
            raise ValueError("Messages must be a non-empty list")

        # Convert messages to litellm format with validation
        litellm_messages = []
        for msg in messages:
            if hasattr(msg, 'role') and hasattr(msg, 'content'):
                if not msg.role or not msg.content:
                    raise ValueError("Message role and content must not be empty")
                litellm_messages.append({"role": msg.role, "content": msg.content})
            else:
                # Fallback for simple string messages
                litellm_messages.append({"role": "user", "content": str(msg)})

        # Validate parameters
        if not (0.0 <= temperature <= 1.0):
            raise ValueError("Temperature must be between 0.0 and 1.0")

        if not isinstance(max_tokens, int) or max_tokens <= 0:
            raise ValueError("max_tokens must be a positive integer")

        # Call LiteLLM with validated parameters
        litellm.api_key = api_key
        response = completion(
            model=provider_model,
            messages=litellm_messages,
            temperature=temperature,
            max_tokens=max_tokens,
            stream=False
        )

        # Validate response
        if not isinstance(response, dict):
            raise ValueError("Invalid response format from LiteLLM")

        return response

    except ValueError as ve:
        logger.warning(f"Validation error in call_litellm: {ve}")
        return {"text": f"validation error: {str(ve)}", "usage": {"total_tokens": 0}}
    except Exception as e:
        logger.exception("LiteLLM call failed")
        return {"text": f"litellm error: {str(e)}", "usage": {"total_tokens": 0}}

class ModelServicer:
    # will be wrapped when protos are generated
    def Generate(self, request, context):
        """Generate response from model with proper validation"""
        try:
            # Validate request
            if not request:
                raise ValueError("Empty request received")

            # Validate and extract messages
            if not hasattr(request, "messages") or not request.messages:
                raise ValueError("Messages are required")

            msgs = list(request.messages)
            if not msgs:
                raise ValueError("Messages list cannot be empty")

            # Validate model
            if not request.model:
                raise ValueError("Model name is required")

            # Validate parameters
            temperature = request.temperature if hasattr(request, "temperature") else 0.7
            max_tokens = request.max_tokens if hasattr(request, "max_tokens") else 1024

            if not (0.0 <= temperature <= 1.0):
                raise ValueError("Temperature must be between 0.0 and 1.0")

            if not isinstance(max_tokens, int) or max_tokens <= 0:
                raise ValueError("max_tokens must be a positive integer")

            text = " ".join(msgs) if msgs else "empty"

            if LITELLM:
                prov = request.model or "local"
                try:
                    res = call_litellm(f"{prov}/{request.model}", msgs, temperature, max_tokens)
                    text = ""
                    if isinstance(res, dict):
                        if "choices" in res and len(res["choices"]) > 0:
                            for c in res["choices"]:
                                text += c.get("message", {}).get("content", "") or c.get("text", "")
                        else:
                            text = res.get("text", str(res))
                    else:
                        text = str(res)
                except Exception as e:
                    logger.exception("Error in Generate method")
                    text = f"error: {str(e)}"
            else:
                text = f"proxy-echo: {text}"

            # Create and return proper GenResponse
            tokens_used = max(1, len(text) // 4)  # Simple token estimation
            return model_pb2.GenResponse(
                request_id=request.request_id if hasattr(request, "request_id") else "",
                text=text,
                tokens_used=tokens_used
            )

        except ValueError as ve:
            logger.warning(f"Validation error in Generate: {ve}")
            return model_pb2.GenResponse(
                request_id="",
                text=f"validation error: {str(ve)}",
                tokens_used=0
            )
        except Exception as e:
            logger.exception("Error in Generate method")
            return model_pb2.GenResponse(
                request_id="",
                text=f"error: {str(e)}",
                tokens_used=0
            )

    def BatchGenerate(self, request, context):
        """Process multiple generation requests in a single batch with validation"""
        responses = []

        try:
            # Validate batch request
            if not request or not hasattr(request, "requests") or not request.requests:
                raise ValueError("Empty batch request received")

            for single_request in request.requests:
                try:
                    # Validate each individual request
                    if not single_request:
                        raise ValueError("Empty request in batch")

                    # Validate and extract messages
                    if not hasattr(single_request, "messages") or not single_request.messages:
                        raise ValueError("Messages are required")

                    msgs = list(single_request.messages)
                    if not msgs:
                        raise ValueError("Messages list cannot be empty")

                    # Validate model
                    if not single_request.model:
                        raise ValueError("Model name is required")

                    # Validate parameters
                    temperature = single_request.temperature if hasattr(single_request, "temperature") else 0.7
                    max_tokens = single_request.max_tokens if hasattr(single_request, "max_tokens") else 1024

                    if not (0.0 <= temperature <= 1.0):
                        raise ValueError("Temperature must be between 0.0 and 1.0")

                    if not isinstance(max_tokens, int) or max_tokens <= 0:
                        raise ValueError("max_tokens must be a positive integer")

                    text = " ".join(msgs) if msgs else "empty"

                    if LITELLM:
                        prov = single_request.model or "local"
                        try:
                            res = call_litellm(f"{prov}/{single_request.model}", msgs, temperature, max_tokens)
                            text = ""
                            if isinstance(res, dict):
                                if "choices" in res and len(res["choices"]) > 0:
                                    for c in res["choices"]:
                                        text += c.get("message", {}).get("content", "") or c.get("text", "")
                                else:
                                    text = res.get("text", str(res))
                            else:
                                text = str(res)
                        except Exception as e:
                            logger.exception("Error in batch request")
                            text = f"error: {str(e)}"
                    else:
                        text = f"proxy-echo: {text}"

                    # Create and return proper GenResponse for this request
                    tokens_used = max(1, len(text) // 4)  # Simple token estimation
                    response = model_pb2.GenResponse(
                        request_id=single_request.request_id if hasattr(single_request, "request_id") else "",
                        text=text,
                        tokens_used=tokens_used
                    )
                    responses.append(response)

                except ValueError as ve:
                    logger.warning(f"Validation error in batch request: {ve}")
                    # Add error response for this request
                    responses.append(model_pb2.GenResponse(
                        request_id=single_request.request_id if hasattr(single_request, "request_id") else "",
                        text=f"validation error: {str(ve)}",
                        tokens_used=0
                    ))

            return model_pb2.BatchGenResponse(responses=responses)

        except Exception as e:
            logger.exception("Error in BatchGenerate method")
            # Return empty response with error
            return model_pb2.BatchGenResponse(responses=[])

    def GenerateStream(self, request, context):
        """Streaming version of Generate that yields multiple responses with validation"""
        try:
            # Validate request
            if not request:
                raise ValueError("Empty request received")

            # Validate and extract messages
            if not hasattr(request, "messages") or not request.messages:
                raise ValueError("Messages are required")

            msgs = list(request.messages)
            if not msgs:
                raise ValueError("Messages list cannot be empty")

            # Validate model
            if not request.model:
                raise ValueError("Model name is required")

            # Validate parameters
            temperature = request.temperature if hasattr(request, "temperature") else 0.7
            max_tokens = request.max_tokens if hasattr(request, "max_tokens") else 1024

            if not (0.0 <= temperature <= 1.0):
                raise ValueError("Temperature must be between 0.0 and 1.0")

            if not isinstance(max_tokens, int) or max_tokens <= 0:
                raise ValueError("max_tokens must be a positive integer")

            text = " ".join(msgs) if msgs else "empty"

            # For streaming, we'll split the response into chunks
            if LITELLM:
                prov = request.model or "local"
                try:
                    res = call_litellm(f"{prov}/{request.model}", msgs, temperature, max_tokens)
                    if isinstance(res, dict):
                        if "choices" in res and len(res["choices"]) > 0:
                            # Yield each choice as a separate response
                            for c in res["choices"]:
                                chunk_text = c.get("message", {}).get("content", "") or c.get("text", "")
                                if chunk_text:
                                    tokens_used = max(1, len(chunk_text) // 4)
                                    yield model_pb2.GenResponse(
                                        request_id=request.request_id if hasattr(request, "request_id") else "",
                                        text=chunk_text,
                                        tokens_used=tokens_used
                                    )
                        else:
                            # Single response
                            text = res.get("text", str(res))
                            tokens_used = max(1, len(text) // 4)
                            yield model_pb2.GenResponse(
                                request_id=request.request_id if hasattr(request, "request_id") else "",
                                text=text,
                                tokens_used=tokens_used
                            )
                    else:
                        # Fallback for non-dict responses
                        text = str(res)
                        tokens_used = max(1, len(text) // 4)
                        yield model_pb2.GenResponse(
                            request_id=request.request_id if hasattr(request, "request_id") else "",
                            text=text,
                            tokens_used=tokens_used
                        )
                except Exception as e:
                    logger.exception("Error in GenerateStream method")
                    error_text = f"error: {str(e)}"
                    yield model_pb2.GenResponse(
                        request_id=request.request_id if hasattr(request, "request_id") else "",
                        text=error_text,
                        tokens_used=1
                    )
            else:
                # Fallback echo for non-litellm case
                tokens_used = max(1, len(text) // 4)
                yield model_pb2.GenResponse(
                    request_id=request.request_id if hasattr(request, "request_id") else "",
                    text=f"proxy-echo: {text}",
                    tokens_used=tokens_used
                )

        except ValueError as ve:
            logger.warning(f"Validation error in GenerateStream: {ve}")
            yield model_pb2.GenResponse(
                request_id="",
                text=f"validation error: {str(ve)}",
                tokens_used=0
            )
        except Exception as e:
            logger.exception("Error in GenerateStream method")
            yield model_pb2.GenResponse(
                request_id="",
                text=f"error: {str(e)}",
                tokens_used=0
            )

def get_server_credentials():
    cert_dir = os.path.join(os.path.dirname(__file__), "certs")
    with open(os.path.join(cert_dir, "model-proxy.pem"), "rb") as f:
        cert_chain = f.read()
    with open(os.path.join(cert_dir, "model-proxy-key.pem"), "rb") as f:
        private_key = f.read()
    with open(os.path.join(cert_dir, "ca.pem"), "rb") as f:
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
