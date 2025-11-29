package main

import (
"context"
"encoding/json"
"log"
"net"
"net/http"
"os"

"github.com/hashicorp/vault/api"
"google.golang.org/grpc"
"google.golang.org/grpc/credentials"
pb "llm-gateway-pro/services/secret-service/pb"
)

var vaultClient *api.Client

type server struct{ pb.UnimplementedSecretServiceServer }

func init() {
config := api.DefaultConfig()
config.Address = os.Getenv("VAULT_ADDR") // http://vault:8200
client, err := api.NewClient(config)
if err != nil {
log.Fatal(err)
}
client.SetToken(os.Getenv("VAULT_TOKEN")) // token with proper rights
vaultClient = client
}

// ===================== gRPC =====================
func (s *server) GetSecret(ctx context.Context, req *pb.GetSecretRequest) (*pb.GetSecretResponse, error) {
secret, err := vaultClient.Logical().Read("secret/data/" + req.Name)
if err != nil || secret == nil {
return nil, err
}
data := secret.Data["data"].(map[string]interface{})
value := data["value"].(string)
return &pb.GetSecretResponse{Value: value}, nil
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
secrets, _ := vaultClient.Logical().List("secret/metadata/llm")
json.NewEncoder(w).Encode(secrets)

case http.MethodPost:
var input struct {
Path  string `json:"path"`  // "llm/openai/api_key"
Value string `json:"value"`
}
json.NewDecoder(r.Body).Decode(&input)
_, err := vaultClient.Logical().Write("secret/data/"+input.Path, map[string]interface{}{
"data": map[string]interface{}{"value": input.Value},
})
if err != nil {
http.Error(w, err.Error(), 500)
return
}
json.NewEncoder(w).Encode(map[string]string{"status": "saved"})

case http.MethodDelete:
name := r.URL.Path[len("/admin/api/secrets/"):]
_, err := vaultClient.Logical().Delete("secret/data/" + name)
if err != nil {
http.Error(w, err.Error(), 500)
return
}
json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}
}

func main() {
init()

// gRPC (mTLS)
lis, _ := net.Listen("tcp", ":50053")
creds, _ := credentials.NewServerTLSFromFile("/certs/secret-service.pem", "/certs/secret-service-key.pem")
grpcServer := grpc.NewServer(grpc.Creds(creds))
pb.RegisterSecretServiceServer(grpcServer, &server{})
go grpcServer.Serve(lis)

// HTTP Admin API
http.HandleFunc("/admin/api/secrets", adminHandler)
http.HandleFunc("/admin/api/secrets/", adminHandler)
log.Println("secret-service: Vault + gRPC :50053 + HTTP :8082")
log.Fatal(http.ListenAndServe(":8082", nil))
}
