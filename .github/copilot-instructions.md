<!-- Copilot / AI Agent instructions for the ZB repository -->
# Copilot instructions — ZB LLM Platform

Purpose
- Quickly orient an AI coding agent to be productive in this repository.
- Focus on the architecture, key workflows, conventions, and concrete commands.

Big picture (core components)
- `proto/`: all Protocol Buffer definitions used across services.
- `services/head-go/`: head service (Go) — request routing toward `model-proxy`.
- `services/tail-go/`: tail/main business logic service (Go).
- `services/gateway/`: gateway for agent-facing APIs and provider management.
- `services/model-proxy/`: model proxy (provider integrations; Python in this repo).
- `services/secrets-service/`: secrets management (Go) — gRPC service used by other services.
- `ui/admin-dashboard/`: admin UI (React) for managing secrets and monitoring.
- `deploy/`, `docker-compose.yml`, `Makefile`: deployment and orchestration assets.

Request flows (explicit, two canonical flows)
- Client → Tail → Head → Model Proxy → LLM Provider
- Client → Gateway → Head → Model Proxy → LLM Provider

Key workflows & commands (concrete examples)
- Build all container images (root):
  - `make build`  (runs `docker compose build`)
- Start full stack locally (root):
  - `make up`  (runs `docker compose up`)
- Generate protobuf code (examples used in README):
  - Install protoc plugins:
    - `go install google.golang.org/protobuf/cmd/protoc-gen-go@latest`
    - `go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest`
    - `export PATH="$PATH:$(go env GOPATH)/bin"`
  - Generate for a service (example):
    - `protoc -I proto --go_out=./services/head-go/gen --go-grpc_out=./services/head-go/gen proto/*.proto`
  - Pattern: generate into each service's `gen` folder (e.g. `services/tail-go/gen`).
- Run a single Go service locally:
  - `cd services/<service>` then `go run .` or `go build && ./<binary>` depending on service layout.
- Tests:
  - Many services include Go tests (e.g. `services/auth-service/main_test.go`). Run `go test ./services/…` per package.

Conventions and repository-specific patterns
- Protobufs: always regenerate when proto files change; generated code lives in `services/<svc>/gen`.
- gRPC + mTLS: services communicate over gRPC and expect mTLS and secret retrieval via `secrets-service`.
- Secrets: no hardcoded keys — secrets are stored/managed via `services/secrets-service` and the Admin UI.
- Docker-first local dev: Compose + service Dockerfiles are the canonical local/run environment. Prefer `make build` / `make up` for an integrated environment.
- Service startup order matters: ensure cache/redis → `model-proxy` → `head` → `tail` to avoid race conditions.

Integration points and external dependencies
- LLM providers: reached through `services/model-proxy` (adapter pattern — keep provider logic contained there).
- Vault/Secret store: `services/secrets-service` is the single source of truth for provider API keys.
- Redis/cache and rate-limiter: shared infra; check `docker-compose.yml` and `deploy/` playbooks for exact names.
- Observability: Prometheus metrics exposed on `/metrics`; dashboards in `observability/` and `observability/dashboards/`.

Files to inspect when making changes
- `README.md` — high-level architecture and quick-start steps.
- `SERVICE_ARCHITECTURE.md` — detailed component interaction (read before cross-service edits).
- `docker-compose.yml` & `Makefile` — how services are assembled locally.
- `deploy/` — Ansible automation; check `deploy/site.yml` and `group_vars/all.yml` for production values.
- `proto/` — authoritative protobuf definitions; any change requires regen and service updates.
- `services/*/Dockerfile` and `services/*/go.mod` — service build/runtime expectations.

Behavior guidance for Copilot-style edits
- When touching protos: regenerate target service `gen` folder, run `go test` for affected services.
- Prefer small, service-scoped changes; respect service boundaries (don’t inline provider logic into `head` or `tail`).
- For changes involving secrets or connection parameters, update `deploy/group_vars/all.yml` and `deploy/README.md` notes.
- Use Docker Compose for integration verification rather than assuming a local environment parity.

Examples to copy/paste
- Generate proto for `tail-go`:
  - `protoc -I proto --go_out=./services/tail-go/gen --go-grpc_out=./services/tail-go/gen proto/*.proto`
- Start full dev stack and view admin UI:
  - `make build && make up`
  - Admin UI: `http://localhost:3000` (secrets + monitoring)

If something is missing or ambiguous
- Ask for which service, environment (dev/staging/prod), or the specific file(s) you will change.
- If credentials or certs are required, point to `certs/` and `deploy/` for generation and Ansible-managed secrets.

Keep this file short — it is the agent's cheat-sheet. Update it when cross-service contracts or deployment steps change.
