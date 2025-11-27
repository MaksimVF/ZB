LLM Platform Skeleton

Structure:
- proto/: proto definitions
- services/head-go/: Go head service scaffold (needs gen from protoc)
- services/tail-go/: Go tail REST->gRPC proxy scaffold (needs gen)
- services/model-proxy/: simple Python model proxy (FastAPI)
- ui/: minimal React app (Vite)
- docker-compose.yml, Makefile

Steps:
1. Generate protobuf go code:
   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
   export PATH="$PATH:$(go env GOPATH)/bin"
   protoc -I proto --go_out=./services/head-go/gen --go-grpc_out=./services/head-go/gen proto/*.proto
   protoc -I proto --go_out=./services/tail-go/gen --go-grpc_out=./services/tail-go/gen proto/*.proto

2. Build images:
   make build

3. Start stack:
   make up

Notes:
- The Go services expect generated pb files in services/*/gen/.
- The model-proxy is a minimal echo server; replace with litellm integration if desired.
- Add mTLS and real provider implementations before production.
