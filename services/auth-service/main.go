// services/auth-service/main.go
package main

import (
"context"
"crypto/rand"
"encoding/base64"
"encoding/json"
"fmt"
"log"
"net/http"
"time"

"github.com/dgrijalva/jwt-go"
"github.com/go-redis/redis/v8"
"github.com/gorilla/mux"
"golang.org/x/crypto/bcrypt"
"gopkg.in/gomail.v2"
"google.golang.org/grpc"
pb "llm-gateway-pro/services/auth-service/pb"
"gorm.io/driver/postgres"
"gorm.io/gorm"
)

var (
db    *gorm.DB
rdb   = redis.NewClient(&redis.Options{Addr: "redis:6379"})
secret = []byte("your-super-secret-jwt-key-2025")
)

type User struct {
ID        string    `gorm:"primaryKey" json:"id"`
Email     string    `gorm:"unique" json:"email"`
Password  string    `json:"-"`
Role      string    `json:"role"` // user, admin, superadmin
Balance   float64   `json:"balance_usd"`
TOTP      string    `json:"-"` // зашифрованный секрет
CreatedAt time.Time `json:"created_at"`
}

type APIKey struct {
ID      string `gorm:"primaryKey"`
UserID  string
Key     string `gorm:"unique"`
Prefix  string
Name    string
Active  bool
Created time.Time
}

func main() {
dsn := "host=postgres user=postgres password=postgres dbname=authdb port=5432 sslmode=disable"
var err error
db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
if err != nil { log.Fatal(err) }
db.AutoMigrate(&User{}, &APIKey{})

r := mux.NewRouter()
r.HandleFunc("/register", Register).Methods("POST")
r.HandleFunc("/login", Login).Methods("POST")
r.HandleFunc("/me", AuthMiddleware(Me)).Methods("GET")
r.HandleFunc("/api-keys", AuthMiddleware(ListAPIKeys)).Methods("GET")
r.HandleFunc("/api-keys", AuthMiddleware(CreateAPIKey)).Methods("POST")
r.HandleFunc("/balance", AuthMiddleware(GetBalance)).Methods("GET")

// gRPC для gateway
go func() {
lis, _ := net.Listen("tcp", ":50051")
s := grpc.NewServer()
pb.RegisterAuthServiceServer(s, &server{})
s.Serve(lis)
}()

log.Println("Auth service: HTTP :8081 | gRPC :50051")
http.ListenAndServe(":8081", r)
}

// === HTTP API ===
func Register(w http.ResponseWriter, r *http.Request) {
var req struct { Email, Password string }
json.NewDecoder(r.Body).Decode(&req)

hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
user := User{
ID:       uuid.New().String(),
Email:    req.Email,
Password: string(hash),
Role:     "user",
Balance:  10.0, // стартовый бонус
}
db.Create(&user)

// Генерируем первый API-ключ
createAPIKeyForUser(user.ID, "Default key")

json.NewEncoder(w).Encode(map[string]string{"status": "ok", "user_id": user.ID})
}

func Login(w http.ResponseWriter, r *http.Request) {
var req struct { Email, Password string }
json.NewDecoder(r.Body).Decode(&req)

var user User
db.Where("email = ?", req.Email).First(&user)
if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
http.Error(w, "invalid", 401); return
}

token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
"user_id": user.ID,
"email":   user.Email,
"role":    user.Role,
"exp":     time.Now().Add(30 * 24 * time.Hour).Unix(),
})
signed, _ := token.SignedString(secret)

json.NewEncoder(w).Encode(map[string]interface{}{
"token": signed,
"user":  user,
"api_keys": getUserAPIKeys(user.ID),
})
}

func Me(w http.ResponseWriter, r *http.Request) {
user := r.Context().Value("user").(User)
json.NewEncoder(w).Encode(user)
}

func ListAPIKeys(w http.ResponseWriter, r *http.Request) {
user := r.Context().Value("user").(User)
json.NewEncoder(w).Encode(getUserAPIKeys(user.ID))
}

func CreateAPIKey(w http.ResponseWriter, r *http.Request) {
user := r.Context().Value("user").(User)
var req struct { Name string }
json.NewDecoder(r.Body).Decode(&req)
createAPIKeyForUser(user.ID, req.Name)
json.NewEncoder(w).Encode(map[string]string{"status": "created"})
}

func createAPIKeyForUser(userID, name string) {
prefix := "tvo_"
raw := make([]byte, 32)
rand.Read(raw)
key := prefix + base64.URLEncoding.EncodeToString(raw)[:32]

db.Create(&APIKey{
ID:      uuid.New().String(),
UserID:  userID,
Key:     key,
Prefix:  prefix,
Name:    name,
Active:  true,
Created: time.Now(),
})
}

func getUserAPIKeys(userID string) []map[string]interface{} {
var keys []APIKey
db.Where("user_id = ?", userID).Find(&keys)
res := []map[string]interface{}{}
for _, k := range keys {
res = append(res, map[string]interface{}{
"id":      k.ID,
"name":    k.Name,
"key":     k.Key,
"prefix":  k.Prefix,
"created": k.Created,
})
}
return res
}

// === gRPC для gateway ===
type server struct{ pb.UnimplementedAuthServiceServer }

func (s *server) ValidateAPIKey(ctx context.Context, req *pb.ValidateRequest) (*pb.ValidateResponse, error) {
key := req.ApiKey
if !strings.HasPrefix(key, "tvo_") {
return &pb.ValidateResponse{Valid: false}, nil
}

var apiKey APIKey
if db.Where("key = ?", key).First(&apiKey).Error != nil {
return &pb.ValidateResponse{Valid: false}, nil
}

var user User
db.First(&user, "id = ?", apiKey.UserID)

return &pb.ValidateResponse{
Valid:   true,
UserId:  user.ID,
Role:    user.Role,
Balance: user.Balance,
}, nil
}

// === Middleware ===
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
return func(w http.ResponseWriter, r *http.Request) {
tokenStr := r.Header.Get("Authorization")
if strings.HasPrefix(tokenStr, "Bearer ") {
tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")
}

token, _ := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
return secret, nil
})

if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
var user User
db.First(&user, "id = ?", claims["user_id"])
ctx := context.WithValue(r.Context(), "user", user)
next.ServeHTTP(w, r.WithContext(ctx))
return
}
http.Error(w, "unauthorized", 401)
}
}
