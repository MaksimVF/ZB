



package docs

import (
    "context"
    "net/http"
    "strings"

    "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
    "google.golang.org/grpc"
    "google.golang.org/protobuf/reflect/protoreflect"
    "google.golang.org/protobuf/reflect/protoregistry"
)

// API documentation constants
const (
    // ServiceDescription provides a description of the Chat service
    ServiceDescription = `
The Chat service provides access to language models for generating text responses.
It supports both synchronous and streaming responses.

Key features:
- Token-based authentication (when enabled)
- Support for multiple language models
- Configurable response parameters (temperature, max tokens)
- Streaming and non-streaming endpoints
- Comprehensive error handling and monitoring
`

    // ChatCompletionDescription describes the ChatCompletion endpoint
    ChatCompletionDescription = `
ChatCompletion provides a synchronous text generation endpoint.

Parameters:
- model: The language model to use (e.g., "gpt-4o")
- messages: Array of message objects with role and content
- temperature: Controls randomness (0.0-1.0)
- max_tokens: Maximum number of tokens to generate

Returns:
- request_id: Unique request identifier
- full_text: Generated text response
- model: Model used for generation
- provider: Backend provider
- tokens_used: Number of tokens consumed
`

    // ChatCompletionStreamDescription describes the streaming endpoint
    ChatCompletionStreamDescription = `
ChatCompletionStream provides asynchronous text generation with server-sent events.

Parameters:
- model: The language model to use
- messages: Array of message objects
- temperature: Controls randomness
- max_tokens: Maximum tokens to generate

Returns:
- Stream of ChatResponseChunk messages with:
  - request_id: Request identifier
  - chunk: Text chunk
  - is_final: Indicates final chunk
  - provider: Backend provider
  - tokens_used: Tokens consumed
`

    // ErrorCodesDescription documents the error codes
    ErrorCodesDescription = `
Error Codes:
- OK: Successful response
- INVALID_ARGUMENT: Invalid request parameters
- UNAUTHENTICATED: Authentication failed
- PERMISSION_DENIED: Insufficient permissions
- UNAVAILABLE: Service unavailable (circuit breaker open)
- INTERNAL: Internal server error
- UNKNOWN: Unexpected error
`
)

// DocumentationHandler provides API documentation
func DocumentationHandler() http.Handler {
    mux := http.NewServeMux()

    // Service documentation
    mux.HandleFunc("/docs", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/html")
        w.Write([]byte(`
<!DOCTYPE html>
<html>
<head>
    <title>Chat Service API Documentation</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; margin: 20px; }
        h1, h2, h3 { color: #333; }
        pre { background: #f4f4f4; padding: 10px; border: 1px solid #ddd; }
        code { font-family: monospace; }
    </style>
</head>
<body>
    <h1>Chat Service API Documentation</h1>
    <p>` + strings.ReplaceAll(ServiceDescription, "\n", "<br>") + `</p>

    <h2>Endpoints</h2>

    <h3>ChatCompletion</h3>
    <p>` + strings.ReplaceAll(ChatCompletionDescription, "\n", "<br>") + `</p>

    <h3>ChatCompletionStream</h3>
    <p>` + strings.ReplaceAll(ChatCompletionStreamDescription, "\n", "<br>") + `</p>

    <h3>Error Codes</h3>
    <p>` + strings.ReplaceAll(ErrorCodesDescription, "\n", "<br>") + `</p>
</body>
</html>
`))
    })

    return mux
}

// RegisterDocumentation registers the documentation service
func RegisterDocumentation(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) error {
    return mux.HandlePath("GET", "/docs", DocumentationHandler())
}

// GetProtoFiles returns the proto file descriptions
func GetProtoFiles() map[string]string {
    return map[string]string{
        "chat.proto": `
syntax = "proto3";

package gen;

service ChatService {
    rpc ChatCompletion(ChatRequest) returns (ChatResponse);
    rpc ChatCompletionStream(ChatRequest) returns (stream ChatResponseChunk);
}

message ChatRequest {
    string request_id = 1;
    string model = 2;
    repeated Message messages = 3;
    float temperature = 4;
    int32 max_tokens = 5;
}

message Message {
    string role = 1;
    string content = 2;
}

message ChatResponse {
    string request_id = 1;
    string full_text = 2;
    string model = 3;
    string provider = 4;
    int32 tokens_used = 5;
}

message ChatResponseChunk {
    string request_id = 1;
    string chunk = 2;
    bool is_final = 3;
    string provider = 4;
    int32 tokens_used = 5;
}
`,
        "model.proto": `
syntax = "proto3";

package model;

service ModelService {
    rpc Generate(GenRequest) returns (GenResponse);
    rpc GenerateStream(GenRequest) returns (stream GenResponse);
}

message GenRequest {
    string request_id = 1;
    string model = 2;
    repeated string messages = 3;
    float temperature = 4;
    int32 max_tokens = 5;
    bool stream = 6;
}

message GenResponse {
    string text = 1;
    int32 tokens_used = 2;
}
`,
    }
}

// GetServiceInfo returns information about available services
func GetServiceInfo() map[string]string {
    return map[string]string{
        "ChatService": "Provides chat completion functionality",
        "ModelService": "Internal model service for text generation",
    }
}



