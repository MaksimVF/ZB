package main

import (
"context"
"crypto/tls"
"fmt"
"log"
"net/http"
"os"
"os/signal"
"sync"
"syscall"
"time"

"github.com/go-redis/redis/v8"
"google.golang.org/grpc"
"google.golang.org/grpc/credentials"

pb "llm-gateway-pro/services/secret-service/pb" // <-- твой proto
"llm-gateway-pro/services/gateway/handlers"
	"llm-gateway-pro/services/tail-go/cmd/tail/middleware"
)

// Глобальные клиенты
var (
secretClient pb.SecretServiceClient
secretConn   *grpc.ClientConnInterface
redisClient  *redis.Client
secretsCache sync.Map // имя → plaintext (кешируем на 30 сек)
)

func main() {
// === 1. Подключаемся к Redis ===
redisClient = redis.NewClient(&redis.Options{
Addr: "redis:6379",
})

// === 2. Подключаемся к secret-service (gRPC + mTLS) ===
var err error
secretConn, err = grpc.Dial(
"secret-service:50053",
grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
InsecureSkipVerify: false, // в прод — true только если self-signed и доверяешь
})),
)
if err != nil {
log.Fatalf("Не удалось подключиться к secret-service: %v", err)
}
defer secretConn.Close()

secretClient = pb.NewSecretServiceClient(secretConn)

// === 3. Фоновая задача: обновление секретов при изменении ===
go watchSecretsUpdates()

// === 4. HTTP → HTTPS сервер (OpenAI-совместимый API) ===
mux := http.NewServeMux()

// Публичные эндпоинты
mux.HandleFunc("POST /v1/chat/completions", middleware.RateLimiter(handlers.ChatCompletion))
mux.HandleFunc("POST /v1/completions", middleware.RateLimiter(handlers.ChatCompletion))
mux.HandleFunc("POST /v1/batch", middleware.RateLimiter(handlers.BatchSubmit))
mux.HandleFunc("POST /v1/embeddings", middleware.RateLimiter(handlers.Embeddings))
mux.HandleFunc("POST /v1/agentic", middleware.RateLimiter(handlers.AgenticHandler))

/*
// Проверяем авторизацию
conn, _ := grpc.Dial("auth-service:50051", grpc.WithInsecure())
client := pb.NewAuthServiceClient(conn)
resp, _ := client.ValidateAPIKey(ctx, &pb.ValidateRequest{ApiKey: apiKey})

if !resp.Valid { http.Error(w, "invalid key", 401); return }
userID = resp.UserId
balance = resp.Balance
*/

// Health check
mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
w.WriteHeader(http.StatusOK)
fmt.Fprint(w, "OK")
})

srv := &http.Server{
Addr:    ":8443",
Handler: mux,
TLSConfig: &tls.Config{
MinVersion: tls.VersionTLS12,
},
}

// Graceful shutdown
go func() {
log.Println("Gateway запущен на https://0.0.0.0:8443")
if err := srv.ListenAndServeTLS(
"/certs/gateway.pem",
"/certs/gateway-key.pem",
); err != nil && err != http.ErrServerClosed {
log.Fatalf("HTTPS сервер упал: %v", err)
}
}()

// Ожидание сигнала завершения
c := make(chan os.Signal, 1)
signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
<-c

log.Println("Останавливаем gateway...")
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
srv.Shutdown(ctx)
log.Println("Gateway остановлен")
}

// ======================== СЕКРЕТЫ ИЗ VAULT ========================

// Получить секрет из Vault (с кэшем 30 сек)
func getSecret(name string) (string, error) {
// Сначала проверяем кэш
if val, ok := secretsCache.Load(name); ok {
if cached, ok := val.(struct {
value string
exp   time.Time
}); ok && time.Now().Before(cached.exp) {
return cached.value, nil
}
}

// Запрашиваем у secret-service
ctx, cancelCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

resp, err := secretClient.GetSecret(ctx, &pb.GetSecretRequest{Name: name})
if err != nil {
return "", fmt.Errorf("ошибка получения секрета %s: %w", name, err)
}

// Кэшируем на 30 секунд
secretsCache.Store(name, struct {
value string
exp   time.Time
}{value: resp.Value, exp: time.Now().Add(30 * time.Second)})

return resp.Value, nil
}

// Фоновая подписка на обновления секретов
func watchSecretsUpdates() {
	retryDelay := 5 * time.Second
	maxRetryDelay := 60 * time.Second

	for {
		log.Println("Connecting to Redis for secret updates...")
		pubsub := redisClient.Subscribe(context.Background(), "secrets:updated")

		// Wait for subscription confirmation
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := pubsub.Receive(ctx); err != nil {
			log.Printf("Failed to subscribe to Redis: %v. Retrying in %v...", err, retryDelay)
			time.Sleep(retryDelay)
			retryDelay = time.Duration(float64(retryDelay) * 1.5) // exponential backoff
			if retryDelay > maxRetryDelay {
				retryDelay = maxRetryDelay
			}
			continue
		}

		// Reset retry delay on successful connection
		retryDelay = 5 * time.Second

		log.Println("Successfully subscribed to Redis secret updates")

		// Process messages
		for {
			msg, err := pubsub.ReceiveMessage()
			if err != nil {
				log.Printf("Redis subscription error: %v. Reconnecting...", err)
				break
			}
			log.Printf("Секрет обновлён: %s — очищаем кэш", msg.Payload)
			secretsCache.Delete(msg.Payload)
		}

		// Clean up and reconnect
		pubsub.Close()
		time.Sleep(retryDelay)
	}
}
