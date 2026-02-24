# MIG Core Runtime (`migd`)

`migd` is the standalone MIG gateway runtime implemented in Go.

## Responsibilities

- MIG v0.1 protocol handling (`HELLO`, `DISCOVER`, `INVOKE`, `PUBLISH`, `SUBSCRIBE`, `CANCEL`, `HEARTBEAT`)
- gRPC service bindings for Discovery, Invocation, Events, and Control
- WebSocket stream invoke endpoint (`GET /mig/v0.1/stream`)
- Admin API for capability and schema management
- Pro and Cloud extension API scaffolding
- JWT auth middleware and tenant enforcement
- Prometheus metrics endpoint (`GET /metrics`)
- Optional NATS event mirroring and audit JSONL sink
- Optional NATS request/reply protocol binding
- Admin connection viewer API (`GET /admin/v0.1/connections`)
- Built-in OSS dashboard UI (`GET /ui`)
- In-memory reference stores for capability, event, and usage state

## Run

```bash
go run ./core/cmd/migd
```

## Common runtime env vars

- `MIGD_AUTH_MODE=none|jwt`
- `MIGD_JWT_HS256_SECRET=<secret>` (required in JWT mode)
- `MIGD_GRPC_ADDR=:9090` (optional gRPC listener)
- `MIGD_ENABLE_METRICS=true|false`
- `MIGD_NATS_URL=nats://localhost:4222`
- `MIGD_ENABLE_NATS_BINDING=true|false`
- `MIGD_AUDIT_LOG_PATH=./migd-audit.jsonl`

## Current State

This build is a reference implementation for protocol and product-surface execution.
Persistence, distributed coordination, and production hardening remain roadmap items.
