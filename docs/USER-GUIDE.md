# MIG User Guide (OSS Core `migd`)

This guide explains how to run, operate, and integrate the open-source MIG Core runtime.

If you want a fast setup first, use [QUICKSTART.md](QUICKSTART.md), then return here.

## 1) What `migd` Is

`migd` is a standalone gateway that implements MIG v0.1 core behavior and exposes:

- MIG protocol endpoints over HTTP
- Optional gRPC services
- Optional NATS integration
- Admin APIs for capabilities/schemas/conformance/connections
- Built-in OSS UI at `/ui`

Current runtime state is in-memory by default (capabilities, schemas, events, usage, quotas, audit records, connections).

## 2) Runtime Planes

`migd` maps to MIG operation planes:

- Discovery plane: `HELLO`, `DISCOVER`
- Invocation plane: `INVOKE`, WebSocket stream invoke, `CANCEL`
- Event plane: `PUBLISH`, `SUBSCRIBE` (SSE), replay cursor support
- Control plane: `HEARTBEAT`

Cross-cutting concerns available now:

- Auth mode: `none` or JWT (`HS256`)
- Tenant enforcement
- Scope-based capability visibility/authorization
- Prometheus metrics
- Optional audit JSONL sink
- Active connection tracking

## 3) Prerequisites

- Go `1.24+`
- `curl`
- Optional: `jq`
- Optional (for NATS features): running NATS server

Repo root:

```bash
cd model-interface-gateway-oss
```

## 4) Run Modes

### 4.1 Open mode (default)

```bash
go run ./core/cmd/migd
```

### 4.2 JWT mode

```bash
MIGD_AUTH_MODE=jwt \
MIGD_JWT_HS256_SECRET=supersecret \
go run ./core/cmd/migd
```

JWT mode requires:

- `Authorization: Bearer <token>`
- `tenant_id` or `tenant` claim in token
- Optional scopes via `scope` or `scopes`

## 5) Configuration Reference

| Variable | Default | Description |
|---|---|---|
| `MIGD_ADDR` | `:8080` | HTTP listen address |
| `MIGD_GRPC_ADDR` | empty | gRPC listen address; empty disables gRPC |
| `MIGD_AUTH_MODE` | `none` | `none` or `jwt` |
| `MIGD_JWT_HS256_SECRET` | empty | Required when `MIGD_AUTH_MODE=jwt` |
| `MIGD_REQUIRE_TENANT_HEADER` | `false` | Requires `X-Tenant-ID` even when token has tenant claim |
| `MIGD_ENABLE_METRICS` | `true` | Enables metrics middleware and `GET /metrics` |
| `MIGD_NATS_URL` | empty | Enables NATS connectivity for mirroring and binding features |
| `MIGD_ENABLE_NATS_BINDING` | `true` | Enables NATS request/reply binding when `MIGD_NATS_URL` is set |
| `MIGD_AUDIT_LOG_PATH` | empty | JSONL sink path for invoke audit records |

## 6) API Reference (Operational)

Protocol/OpenAPI source files:

- `openapi/mig.v0.1.yaml`
- `openapi/mig.admin.v0.1.yaml`
- `openapi/mig.pro.v0.1.yaml`
- `openapi/mig.cloud.v0.1.yaml`

### 6.1 MIG Core endpoints

- `POST /mig/v0.1/hello`
- `POST /mig/v0.1/discover`
- `POST /mig/v0.1/invoke/{capability}`
- `POST /mig/v0.1/publish/{topic}`
- `GET /mig/v0.1/subscribe/{topic}` (SSE)
- `POST /mig/v0.1/cancel/{message_id}`
- `POST /mig/v0.1/heartbeat`
- `GET /mig/v0.1/stream` (WebSocket upgrade)

### 6.2 Admin endpoints

- `POST /admin/v0.1/capabilities`
- `GET /admin/v0.1/capabilities`
- `POST /admin/v0.1/schemas`
- `GET /admin/v0.1/health/conformance`
- `GET /admin/v0.1/connections`

### 6.3 Pro extension endpoints (scaffolded in this runtime)

- `POST /pro/v0.1/policies/validate`
- `POST /pro/v0.1/quotas`
- `GET /pro/v0.1/audit/export`

### 6.4 Cloud extension endpoints (scaffolded in this runtime)

- `POST /cloud/v0.1/orgs`
- `POST /cloud/v0.1/tenants`
- `POST /cloud/v0.1/gateways`
- `GET /cloud/v0.1/usage`

## 7) Core Workflows

### 7.1 HELLO

```bash
curl -sS -X POST http://localhost:8080/mig/v0.1/hello \
  -H 'Content-Type: application/json' \
  -H 'X-Tenant-ID: acme' \
  -d '{
    "header": {"tenant_id": "acme"},
    "supported_versions": ["0.1"],
    "requested_bindings": ["http"]
  }'
```

### 7.2 DISCOVER

```bash
curl -sS -X POST http://localhost:8080/mig/v0.1/discover \
  -H 'Content-Type: application/json' \
  -H 'X-Tenant-ID: acme' \
  -d '{"header": {"tenant_id": "acme"}}'
```

In JWT mode, `DISCOVER` is scope-filtered. Capabilities requiring scopes not present in token are omitted.

### 7.3 INVOKE

```bash
curl -sS -X POST http://localhost:8080/mig/v0.1/invoke/observatory.models.infer \
  -H 'Content-Type: application/json' \
  -H 'X-Tenant-ID: acme' \
  -d '{
    "header": {
      "tenant_id": "acme",
      "idempotency_key": "idem-1",
      "deadline_ms": 30000
    },
    "payload": {"input": "hello"}
  }'
```

Important behavior:

- `header.idempotency_key` deduplicates repeated calls per tenant + capability + key
- `header.deadline_ms` controls request timeout
- In JWT mode, invoke requires at least one matching capability scope

### 7.4 CANCEL

```bash
curl -sS -X POST http://localhost:8080/mig/v0.1/cancel/msg-123 \
  -H 'Content-Type: application/json' \
  -H 'X-Tenant-ID: acme' \
  -d '{
    "header": {"tenant_id": "acme"},
    "target_message_id": "msg-123",
    "reason": "operator cancel"
  }'
```

### 7.5 PUBLISH + SUBSCRIBE (SSE)

Subscriber:

```bash
curl -N http://localhost:8080/mig/v0.1/subscribe/observatory.inference.completed \
  -H 'X-Tenant-ID: acme'
```

Publisher:

```bash
curl -sS -X POST http://localhost:8080/mig/v0.1/publish/observatory.inference.completed \
  -H 'Content-Type: application/json' \
  -H 'X-Tenant-ID: acme' \
  -d '{
    "header": {"tenant_id": "acme"},
    "payload": {"status": "ok"}
  }'
```

Replay behavior:

- `resume_cursor` query param is a non-negative integer offset into retained events for the topic

## 8) OSS UI (`/ui`)

Open:

- [http://localhost:8080/ui](http://localhost:8080/ui)

UI capabilities:

- Shows active connection summary and details
- Shows invocation usage and capability count
- Shows conformance health
- Sends HELLO, DISCOVER, INVOKE directly from browser
- Supports tenant selection and optional bearer token input

Auth behavior:

- `/ui` page is always served
- In JWT mode, API calls from UI fail until you provide a valid token in the token field

## 9) Connection Viewer (`/admin/v0.1/connections`)

Use this to inspect active long-lived runtime connections.

Filters:

- `tenant_id`
- `kind`
- `protocol`

Example:

```bash
curl -sS 'http://localhost:8080/admin/v0.1/connections?tenant_id=acme&protocol=http'
```

Response shape:

```json
{
  "generated_at": "2026-02-24T21:10:13Z",
  "summary": {
    "total": 1,
    "by_protocol": {"http": 1},
    "by_kind": {"sse_subscribe": 1},
    "by_tenant": {"acme": 1},
    "nats_binding_active": false
  },
  "connections": [
    {
      "id": "conn-abc123",
      "protocol": "http",
      "kind": "sse_subscribe",
      "tenant_id": "acme",
      "actor": "anonymous",
      "remote_addr": "127.0.0.1:60942",
      "started_at": "2026-02-24T21:10:11Z",
      "meta": {"topic": "observatory.inference.completed", "resume_cursor": ""}
    }
  ]
}
```

Kinds you can typically see:

- `sse_subscribe`
- `ws_stream`
- `stream_invoke` (gRPC bidi stream)
- `event_subscribe` (gRPC server stream)

Unary HTTP calls are short-lived and do not appear as active connections.

## 10) Admin Operations

### 10.1 Add a capability

```bash
curl -sS -X POST http://localhost:8080/admin/v0.1/capabilities \
  -H 'Content-Type: application/json' \
  -d '{
    "descriptor": {
      "id": "acme.tools.summarize",
      "version": "1.0.0",
      "modes": ["unary"],
      "input_schema_uri": "schema://acme/summarize/input/v1",
      "output_schema_uri": "schema://acme/summarize/output/v1",
      "auth_scopes": ["capability:summarize"],
      "event_topics": ["acme.summarize.completed"]
    }
  }'
```

### 10.2 Add a schema

```bash
curl -sS -X POST http://localhost:8080/admin/v0.1/schemas \
  -H 'Content-Type: application/json' \
  -d '{
    "uri": "schema://acme/summarize/input/v1",
    "schema": {
      "type": "object",
      "properties": {"text": {"type": "string"}},
      "required": ["text"]
    }
  }'
```

### 10.3 List capabilities

```bash
curl -sS http://localhost:8080/admin/v0.1/capabilities
```

## 11) Pro and Cloud API Scaffolds

These are currently reference implementations for product-surface planning and integration.

Pro examples:

```bash
curl -sS -X POST http://localhost:8080/pro/v0.1/policies/validate \
  -H 'Content-Type: application/json' \
  -d '{"tenant_id":"acme","capability":"observatory.models.infer","action":"invoke"}'
```

```bash
curl -sS -X POST http://localhost:8080/pro/v0.1/quotas \
  -H 'Content-Type: application/json' \
  -d '{"tenant_id":"acme","max_invocations":100}'
```

```bash
curl -sS 'http://localhost:8080/pro/v0.1/audit/export?tenant_id=acme'
```

Cloud examples:

```bash
curl -sS -X POST http://localhost:8080/cloud/v0.1/orgs \
  -H 'Content-Type: application/json' \
  -d '{"name":"Acme"}'
```

```bash
curl -sS -X POST http://localhost:8080/cloud/v0.1/tenants \
  -H 'Content-Type: application/json' \
  -d '{"org_id":"<org-id>","name":"Acme Prod"}'
```

```bash
curl -sS -X POST http://localhost:8080/cloud/v0.1/gateways \
  -H 'Content-Type: application/json' \
  -d '{"tenant_id":"<tenant-id>","region":"us-east-1","binding":"http"}'
```

```bash
curl -sS http://localhost:8080/cloud/v0.1/usage
```

## 12) gRPC Binding

Enable gRPC server:

```bash
MIGD_GRPC_ADDR=:9090 go run ./core/cmd/migd
```

Smoke test:

```bash
go run ./conformance/harness/cmd/mig-grpc-smoke -addr localhost:9090 -tenant acme
```

Auth metadata for JWT mode:

- `authorization: Bearer <token>`
- `x-tenant-id: <tenant>`

## 13) NATS Integration

### 13.1 Event mirroring

Enable with `MIGD_NATS_URL`.

Published MIG events are mirrored to:

- `mig.v0_1.<tenant>.events.<topic>`

### 13.2 NATS request/reply binding

Enable both:

```bash
MIGD_NATS_URL=nats://localhost:4222 \
MIGD_ENABLE_NATS_BINDING=true \
go run ./core/cmd/migd
```

Request subjects:

- `mig.v0_1.<tenant>.hello`
- `mig.v0_1.<tenant>.discover`
- `mig.v0_1.<tenant>.invoke.<capability>`
- `mig.v0_1.<tenant>.events.<topic>`
- `mig.v0_1.<tenant>.control.cancel.<message_id>`
- `mig.v0_1.<tenant>.control.heartbeat`

Smoke test:

```bash
go run ./conformance/harness/cmd/mig-nats-smoke -url nats://localhost:4222 -tenant acme
```

## 14) Observability and Audit

Metrics endpoint:

- `GET /metrics`

Primary metrics:

- `mig_gateway_http_requests_total`
- `mig_gateway_http_request_duration_seconds`
- `mig_gateway_errors_total`
- `mig_gateway_active_streams`

Audit JSONL sink:

```bash
MIGD_AUDIT_LOG_PATH=./migd-audit.jsonl go run ./core/cmd/migd
```

Each successful invoke appends one JSON line with actor, tenant, capability, outcome, timestamp, and message ID.

## 15) Validation and Test Commands

Run all unit tests:

```bash
go test ./...
```

Run HTTP conformance smoke checks:

```bash
go run ./conformance/harness/cmd/mig-conformance -base-url http://localhost:8080 -tenant acme
```

## 16) Known Current Limitations

- Runtime state is in-memory (not durable across restarts)
- No built-in distributed coordination/HA in OSS runtime
- `/ui` is intentionally lightweight and does not replace external observability tooling
- Pro/Cloud endpoints are scaffolded reference behavior, not full enterprise control-plane implementation

## 17) Related Docs

- [Demo Guide](DEMO-GUIDE.md)
- [Quick Start](QUICKSTART.md)
- [Runtime Guide](MIGD-RUNTIME-GUIDE.md)
- [Compatibility Matrix](COMPATIBILITY-MATRIX.md)
- [Conformance Checklist](../conformance/CONFORMANCE-CHECKLIST.md)
