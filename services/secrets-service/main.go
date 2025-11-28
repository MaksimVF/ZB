package main

import (
"context"
"crypto/aes"
"crypto/cipher"
"crypto/rand"
"encoding/base64"
"encoding/json"
"log"
"net"
"net/http"
"os"
"sync"

"github.com/go-redis/redis/v8"
"google.golang.org/grpc"
"google.golang.org/grpc/credentials"
pb "llm-gateway-pro/services/secret-service/pb"
)

var (
rdb        *redis.Client
aesgcm     cipher.AEAD
encryptOnce sync.Once
masterKey   []byte
)

type server struct{ pb.UnimplementedSecretServiceServer }

func initEncryption() {
key := os.Getenv("ENCRYPTION_MASTER_KEY")
if key == "" {
log.Fatal("ENCRYPTION_MASTER_KEY required")
}
var err error
masterKey, err = base64.StdEncoding.DecodeString(key)
if err != nil || len(masterKey) != 32 {
log.Fatal("ENCRYPTION_MASTER_KEY must be 32-byte base64")
}
block, _ := aes.NewCipher(masterKey)
aesgcm, _ = cipher.NewGCM(block)
}

// ===================== gRPC =====================
func (s *server) GetSecret(ctx context.Context, req *pb.GetSecretRequest) (*pb.GetSecretResponse, error) {
encrypted, err := rdb.HGet(ctx, "secrets", req.Name).Result()
if err != nil {
return nil, err
}
plaintext, err := decrypt(encrypted)
if err != nil {
return nil, err
}
return &pb.GetSecretResponse{Value: plaintext}, nil
}

// ===================== HTTP Admin API =====================
func adminHandler(w http.ResponseWriter, r *http.Request) {
if r.Header.Get("X-Admin-Key") != os.Getenv("ADMIN_KEY") {
http.Error(w, "forbidden", 403)
return
}
w.Header().Set("Access-Control-Allow-Origin", "*")
w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE")
w.Header().Set("Access-Control-Allow-Headers", "Content-Type,X-Admin-Key")

if r.Method == http.MethodOptions {
return
}

switch r.Method {
case http.MethodGet:
secrets, _ := rdb.HGetAll(r.Context(), "secrets").Result()
visible := map[string]string{}
for k, v := range secrets {
if dec, err := decrypt(v); err == nil && len(dec) > 8 {
visible[k] = "sk-... " + dec[len(dec)-8:]
} else {
visible[k] = "invalid"
}
}
json.NewEncoder(w).Encode(visible)

case http.MethodPost:
var input struct {
Name  string `json:"name"`  // "openai.api_key"
Value string `json:"value"`
}
json.NewDecoder(r.Body).Decode(&input)
encrypted := encrypt(input.Value)
rdb.HSet(r.Context(), "secrets", input.Name, encrypted)
rdb.Publish(r.Context(), "secrets:updated", input.Name)
json.NewEncoder(w).Encode(map[string]string{"status": "saved"})

case http.MethodDelete:
name := r.URL.Path[len("/admin/api/secrets/"):]
rdb.HDel(r.Context(), "secrets", name)
json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}
}

func encrypt(plain string) string {
encryptOnce.Do(initEncryption)
nonce := make([]byte, aesgcm.NonceSize())
rand.Read(nonce)
ciphertext := aesgcm.Seal(nonce, nonce, []byte(plain), nil)
return base64.StdEncoding.EncodeToString(ciphertext)
}

func decrypt(encrypted string) (string, error) {
encryptOnce.Do(initEncryption)
data, _ := base64.StdEncoding.DecodeString(encrypted)
nonceSize := aesgcm.NonceSize()
nonce, ciphertext := data[:nonceSize], data[nonceSize:]
plain, err := aesgcm.Open(nil, nonce, ciphertext, nil)
return string(plain), err
}

func main() {
rdb = redis.NewClient(&redis.Options{Addr: "redis:6379"})
initEncryption()

// gRPC сервер (mTLS)
lis, _ := net.Listen("tcp", ":50053")
creds, _ := credentials.NewServerTLSFromFile("/certs/secret-service.pem", "/certs/secret-service-key.pem")
grpcServer := grpc.NewServer(grpc.Creds(creds))
pb.RegisterSecretServiceServer(grpcServer, &server{})
go grpcServer.Serve(lis)

// HTTP Admin API
http.HandleFunc("/admin/api/secrets", adminHandler)
http.HandleFunc("/admin/api/secrets/", adminHandler)
log.Println("secret-service: gRPC :50053 (mTLS), HTTP :8082")
log.Fatal(http.ListenAndServe(":8082", nil))
}
