# MIG Quick Start (OSS Core)

This quick start gets you from zero to a working MIG gateway with:

- successful `HELLO` / `DISCOVER` / `INVOKE`
- OSS console at `/ui`
- live connection visibility from `/admin/v0.1/connections`

Estimated time: 10 minutes.

## Prerequisites

- Go `1.24+`
- `curl`
- Optional: `jq` for prettier JSON output

From the repo root:

```bash
cd /Users/zacnickens/Projects/model-interface-gateway
```

## 1) Start `migd`

```bash
go run ./core/cmd/migd
```

Default HTTP address is `http://localhost:8080`.

## 2) Verify conformance health

```bash
curl -sS http://localhost:8080/admin/v0.1/health/conformance
```

Expected shape:

```json
{"core":true,"streaming":true,"evented":true,"full":true}
```

## 3) Run core MIG calls

### HELLO

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

### DISCOVER

```bash
curl -sS -X POST http://localhost:8080/mig/v0.1/discover \
  -H 'Content-Type: application/json' \
  -H 'X-Tenant-ID: acme' \
  -d '{
    "header": {"tenant_id": "acme"}
  }'
```

### INVOKE

```bash
curl -sS -X POST http://localhost:8080/mig/v0.1/invoke/observatory.models.infer \
  -H 'Content-Type: application/json' \
  -H 'X-Tenant-ID: acme' \
  -d '{
    "header": {"tenant_id": "acme"},
    "payload": {"input": "hello MIG"}
  }'
```

## 4) Open the OSS UI

Open:

- [http://localhost:8080/ui](http://localhost:8080/ui)

What you should see:

- Active connection count
- Invocation/capability counters
- Conformance status
- Connections table
- Quick HELLO/DISCOVER/INVOKE action panel

## 5) Validate the connection viewer

Open a second terminal and create a long-lived SSE subscription:

```bash
curl -N http://localhost:8080/mig/v0.1/subscribe/observatory.inference.completed \
  -H 'X-Tenant-ID: acme'
```

In a third terminal, query active connections:

```bash
curl -sS 'http://localhost:8080/admin/v0.1/connections?tenant_id=acme'
```

Expected: one connection entry with `kind` like `sse_subscribe`.

Publish an event to the same topic:

```bash
curl -sS -X POST http://localhost:8080/mig/v0.1/publish/observatory.inference.completed \
  -H 'Content-Type: application/json' \
  -H 'X-Tenant-ID: acme' \
  -d '{
    "header": {"tenant_id": "acme"},
    "payload": {"status": "done", "job_id": "job-1"}
  }'
```

The SSE terminal should receive the event immediately.

## 6) Run conformance smoke checks

HTTP checks:

```bash
go run ./conformance/harness/cmd/mig-conformance \
  -base-url http://localhost:8080 \
  -tenant acme
```

## 7) Optional: enable gRPC and run gRPC smoke

Start gateway with gRPC:

```bash
MIGD_GRPC_ADDR=:9090 go run ./core/cmd/migd
```

Run smoke check:

```bash
go run ./conformance/harness/cmd/mig-grpc-smoke \
  -addr localhost:9090 \
  -tenant acme
```

## 8) Optional: run MCP adapter

```bash
go run ./adapters/mcp/cmd/mcp-mig-adapter \
  -mig-url http://localhost:8080 \
  -tenant-id acme \
  -manifest examples/mcp-mig-adapter.v0.2.manifest.yaml
```

Adapter defaults to `:8090`.

## 9) Optional: JWT auth mode

Start gateway in JWT mode:

```bash
MIGD_AUTH_MODE=jwt \
MIGD_JWT_HS256_SECRET=supersecret \
go run ./core/cmd/migd
```

Then include both:

- `Authorization: Bearer <token>`
- `X-Tenant-ID: <tenant>`

Token requirements:

- HS256 signature with the same secret
- `tenant_id` or `tenant` claim
- optional `scope`/`scopes` claims for capability filtering

The `/ui` page itself still loads, and you can paste the token into the UI toolbar so API calls succeed.

## Troubleshooting

- `tenant_id is required`: include `header.tenant_id` in body and/or `X-Tenant-ID`.
- `missing bearer token` or `invalid bearer token`: check `MIGD_AUTH_MODE`, secret, and header.
- `capability not found`: use `observatory.models.infer` or register a capability via admin API.
- Empty connection list: only long-lived streams show up (`sse_subscribe`, `ws_stream`, gRPC stream calls).

## Next Read

- [Demo Guide](DEMO-GUIDE.md)
- [User Guide](USER-GUIDE.md)
- [Runtime Guide](MIGD-RUNTIME-GUIDE.md)
