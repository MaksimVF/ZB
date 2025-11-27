package handlers

import (
"encoding/json"
"io"
"net/http"
"llm-gateway-pro/services/gateway/internal/grpc"
)

func ChatCompletion(headClient *grpc.HeadClient) http.HandlerFunc {
return func(w http.ResponseWriter, r *http.Request) {
var req struct {
Model    string `json:"model"`
Messages []struct {
Role    string `json:"role"`
Content string `json:"content"`
} `json:"messages"`
Stream bool `json:"stream"`
}

json.NewDecoder(r.Body).Decode(&req)

if req.Stream {
// стриминг через SSE
w.Header().Set("Content-Type", "text/event-stream")
stream, _ := headClient.Stream(r.Context(), req.Model, req.Messages)
for chunk := range stream {
io.WriteString(w, "data: "+chunk+"\n\n")
if f, ok := w.(http.Flusher); ok {
f.Flush()
}
}
io.WriteString(w, "data: [DONE]\n\n")
} else {
resp, _ := headClient.Completion(r.Context(), req.Model, req.Messages)
json.NewEncoder(w).Encode(resp)
}
}
}
