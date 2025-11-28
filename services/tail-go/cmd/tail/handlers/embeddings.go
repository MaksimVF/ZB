// services/gateway/handlers/embeddings.go
package handlers

import (
"bytes"
"crypto/sha256"
"encoding/hex"
"encoding/json"
"fmt"
"io"
"log"
"net/http"
"strings"
"time"

"github.com/go-redis/redis/v8"
"llm-gateway-pro/services/gateway/internal/secrets"
)

var rdb = redis.NewClient(&redis.Options{Addr: "redis:6379"})

type EmbeddingsRequest struct {
Model string      `json:"model"`
Input interface{} `json:"input"` // string | []string
}

type EmbeddingResponse struct {
Object string `json:"object"`
Data   []struct {
Index     int       `json:"index"`
Object    string    `json:"object"`
Embedding []float64 `json:"embedding"`
} `json:"data"`
Model string `json:"model"`
Usage struct {
PromptTokens int `json:"prompt_tokens"`
TotalTokens  int `json:"total_tokens"`
} `json:"usage"`
}

var embeddingProviders = map[string]struct {
Provider string
BaseURL  string
}{
"text-embedding-3-large": {"openai", "https://api.openai.com"},
"text-embedding-3-small": {"openai", "https://api.openai.com"},
"voyage-2":               {"voyage", "https://api.voyageai.com"},
"embed-multilingual-v3":  {"cohere", "https://api.cohere.com"},
"textembedding-gecko":    {"google", "https://generativelanguage.googleapis.com"},
}

func Embeddings(w http.ResponseWriter, r *http.Request) {
var req EmbeddingsRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
return
}

// Нормализация и хэширование входа
inputTexts := normalizeInput(req.Input)
cacheKeys := make([]string, len(inputTexts))
hashes := make([]string, len(inputTexts))

for i, text := range inputTexts {
h := sha256.Sum256([]byte(text))
hashes[i] = hex.EncodeToString(h[:])
cacheKeys[i] = fmt.Sprintf("emb:%s:%s", req.Model, hashes[i])
}

// Проверяем кэш
cachedResults := make([]*EmbeddingResponse, len(inputTexts))
missIndices := []int{}
missHashes := []string{}

for i, key := range cacheKeys {
if cached, err := rdb.Get(r.Context(), key).Result(); err == nil {
var resp EmbeddingResponse
if json.Unmarshal([]byte(cached), &resp) == nil {
cachedResults[i] = &resp
w.Header().Add("X-Cache", "HIT")
continue
}
}
missIndices = append(missIndices, i)
missHashes = append(missHashes, hashes[i])
w.Header().Add("X-Cache", "MISS")
}

// Если всё в кэше — сразу отдаём
if len(missIndices) == 0 {
final := buildBatchResponse(req.Model, inputTexts, cachedResults)
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(final)
return
}

// Запрос к провайдеру только для отсутствующих
missTexts := make([]string, len(missIndices))
for i, idx := range missIndices {
missTexts[i] = inputTexts[idx]
}

providerResp := requestEmbeddings(req.Model, missTexts)
if providerResp == nil {
http.Error(w, `{"error":"provider error"}`, http.StatusBadGateway)
return
}

// Сохраняем в кэш (30 дней)
for i, data := range providerResp.Data {
resp := EmbeddingResponse{
Object: "list",
Data:   []struct { Index int; Object string; Embedding []float64 }{data},
Model:  req.Model,
Usage:  providerResp.Usage,
}
raw, _ := json.Marshal(resp)
cacheKey := fmt.Sprintf("emb:%s:%s", req.Model, missHashes[i])
rdb.SetEX(r.Context(), cacheKey, raw, 30*24*time.Hour)
cachedResults[missIndices[i]] = &resp
}

final := buildBatchResponse(req.Model, inputTexts, cachedResults)
w.Header().Set("Content-Type", "application/json")
w.Header().Set("X-Cache-Hits", fmt.Sprintf("%d", len(inputTexts)-len(missIndices)))
w.Header().Set("X-Cache-Misses", fmt.Sprintf("%d", len(missIndices)))
json.NewEncoder(w).Encode(final)
}

// Вспомогательные функции
func normalizeInput(input interface{}) []string {
switch v := input.(type) {
case string:
return []string{strings.TrimSpace(v)}
case []string:
result := make([]string, len(v))
for i, s := range v {
result[i] = strings.TrimSpace(s)
}
return result
default:
return []string{fmt.Sprintf("%v", v)}
}
}

func requestEmbeddings(model string, texts []string) *EmbeddingResponse {
cfg := embeddingProviders[model]
apiKey, err := secrets.Get(fmt.Sprintf("llm/%s/api_key", cfg.Provider))
if err != nil {
log.Printf("Secret error: %v", err)
return nil
}

body := map[string]interface{}{
"model": model,
"input": texts,
}
jsonBody, _ := json.Marshal(body)

url := cfg.BaseURL + "/v1/embeddings"
if cfg.Provider == "google" {
url = cfg.BaseURL + "/v1beta/models/" + model + ":embedContent"
}

req, _ := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
req.Header.Set("Content-Type", "application/json")
switch cfg.Provider {
case "openai", "voyage", "cohere":
req.Header.Set("Authorization", "Bearer "+apiKey)
case "google":
req.URL.RawQuery = "key=" + apiKey
}

resp, err := http.DefaultClient.Do(req)
if err != nil || resp.StatusCode >= 400 {
log.Printf("Provider error: %v, status: %d", err, resp.StatusCode)
return nil
}
defer resp.Body.Close()

var result EmbeddingResponse
json.NewDecoder(resp.Body).Decode(&result)
return &result
}

func buildBatchResponse(model string, inputs []string, results []*EmbeddingResponse) EmbeddingResponse {
data := make([]struct {
Index     int       `json:"index"`
Object    string    `json:"object"`
Embedding []float64 `json:"embedding"`
}, len(inputs))

totalTokens := 0
for i, res := range results {
if res != nil && len(res.Data) > 0 {
data[i] = res.Data[0]
data[i].Index = i
totalTokens += res.Usage.TotalTokens
}
}

return EmbeddingResponse{
Object: "list",
Data:   data,
Model:  model,
Usage: struct {
PromptTokens int `json:"prompt_tokens"`
TotalTokens  int `json:"total_tokens"`
}{TotalTokens: totalTokens, PromptTokens: totalTokens},
}
}
