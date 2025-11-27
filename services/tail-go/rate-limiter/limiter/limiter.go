// limiter/limiter.go
package limiter

import (
"context"
"encoding/json"
"net/http"
"llm-gateway-pro/services/rate-limiter/pb"
)

type Service struct{ pb.UnimplementedRateLimiterServer }

func (s *Service) Check(ctx context.Context, req *pb.CheckRequest) (*pb.CheckResponse, error) {
allowed := CheckTokens(req.ClientId, req.Path, req.Tokens)
return &pb.CheckResponse{Allowed: allowed}, nil
}

func AdminHandler(w http.ResponseWriter, r *http.Request) {
if r.Header.Get("X-Admin-Key") != "devkey123" {
http.Error(w, "forbidden", 403); return
}
// простая реализация
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
