// services/gateway/internal/secrets/helper.go
package secrets

import (
"context"
"fmt"
"sync"
"time"

pb "llm-gateway-pro/services/secret-service/pb"
"google.golang.org/grpc"
)

var (
client pb.SecretServiceClient
once   sync.Once
cache  = make(map[string]cachedSecret)
mu     sync.RWMutex
)

type cachedSecret struct {
value string
exp   time.Time
}

func initClient() {
once.Do(func() {
conn, err := grpc.Dial("secret-service:50053",
grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
grpc.WithBlock(),
)
if err != nil {
panic(fmt.Sprintf("Не удалось подключиться к secret-service: %v", err))
}
client = pb.NewSecretServiceClient(conn)
})
}

func Get(name string) (string, error) {
initClient()

// Проверяем кэш
mu.RLock()
if c, ok := cache[name]; ok && time.Now().Before(c.exp) {
mu.RUnlock()
return c.value, nil
}
mu.RUnlock()

// Запрос в secret-service
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

resp, err := client.GetSecret(ctx, &pb.GetSecretRequest{Name: name})
if err != nil {
return "", err
}

// Кэшируем на 30 сек
mu.Lock()
cache[name] = cachedSecret{value: resp.Value, exp: time.Now().Add(30 * time.Second)}
mu.Unlock()

return resp.Value, nil
}
