# MIG (Model Interface Gateway)

MIG is a transport-neutral interface standard for model and agent interoperability.

This repository includes a runnable standalone gateway (`migd`) and one product lineage:

- `MIG Core` (OSS, Apache-2.0)
- `MIG Pro` (self-hosted commercial overlay)
- `MIG Cloud` (managed control plane overlay)

## Start Here

- [Quick Start](docs/QUICKSTART.md)
- [User Guide](docs/USER-GUIDE.md)
- [Demo Guide](docs/DEMO-GUIDE.md)
- [Runtime Guide](docs/MIGD-RUNTIME-GUIDE.md)
- [MIG v0.1 RFC](rfc/MIG-0001.md)
- [MIG HTTP OpenAPI](openapi/mig.v0.1.yaml)
- [Admin OpenAPI](openapi/mig.admin.v0.1.yaml)

## What Works Today (OSS Core)

- MIG v0.1 HTTP endpoints (`HELLO`, `DISCOVER`, `INVOKE`, `PUBLISH`, `SUBSCRIBE`, `CANCEL`, `HEARTBEAT`)
- WebSocket stream endpoint: `GET /mig/v0.1/stream`
- Built-in OSS UI: `GET /ui`
- Active connection viewer API: `GET /admin/v0.1/connections`
- Admin capability/schema APIs
- Optional gRPC binding
- Optional NATS mirroring and request/reply binding
- Prometheus metrics and JSONL audit sink
- MCP adapter and conformance smoke runners

## 3-Minute Run

### 1) Start `migd`

```bash
go run ./core/cmd/migd
```

Defaults:

- HTTP address: `:8080`
- Auth mode: `none`
- Metrics: enabled at `GET /metrics`

### 2) Send a test invoke

```bash
curl -sS -X POST http://localhost:8080/mig/v0.1/invoke/observatory.models.infer \
  -H 'Content-Type: application/json' \
  -H 'X-Tenant-ID: acme' \
  -d '{
    "header": {"tenant_id": "acme"},
    "payload": {"input": "hello from readme"}
  }'
```

### 3) Open the OSS UI

[http://localhost:8080/ui](http://localhost:8080/ui)

The UI shows active connections, invocation/capability counters, conformance status, and quick HELLO/DISCOVER/INVOKE actions.

## Runtime Configuration

- `MIGD_ADDR` (default `:8080`)
- `MIGD_GRPC_ADDR` (optional, disabled when empty)
- `MIGD_AUTH_MODE` (`none` or `jwt`, default `none`)
- `MIGD_JWT_HS256_SECRET` (required when `MIGD_AUTH_MODE=jwt`)
- `MIGD_REQUIRE_TENANT_HEADER` (`true|false`, default `false`)
- `MIGD_ENABLE_METRICS` (`true|false`, default `true`)
- `MIGD_NATS_URL` (optional)
- `MIGD_ENABLE_NATS_BINDING` (`true|false`, default `true`; requires `MIGD_NATS_URL`)
- `MIGD_AUDIT_LOG_PATH` (optional JSONL path)

## API Surfaces

MIG Core:

- `POST /mig/v0.1/hello`
- `POST /mig/v0.1/discover`
- `POST /mig/v0.1/invoke/{capability}`
- `POST /mig/v0.1/publish/{topic}`
- `GET /mig/v0.1/subscribe/{topic}` (SSE)
- `POST /mig/v0.1/cancel/{message_id}`
- `POST /mig/v0.1/heartbeat`
- `GET /mig/v0.1/stream` (WebSocket)

Admin:

- `POST /admin/v0.1/capabilities`
- `GET /admin/v0.1/capabilities`
- `POST /admin/v0.1/schemas`
- `GET /admin/v0.1/health/conformance`
- `GET /admin/v0.1/connections`

Pro extension (scaffolded):

- `POST /pro/v0.1/policies/validate`
- `POST /pro/v0.1/quotas`
- `GET /pro/v0.1/audit/export`

Cloud extension (scaffolded):

- `POST /cloud/v0.1/orgs`
- `POST /cloud/v0.1/tenants`
- `POST /cloud/v0.1/gateways`
- `GET /cloud/v0.1/usage`

## Repository Layout

- `core/`: Go runtime for `migd`
- `pro/`: Pro overlay docs and API contract
- `cloud/`: Cloud overlay docs and API contract
- `adapters/mcp/`: MCP-to-MIG adapter
- `conformance/harness/`: executable conformance checks
- `sdk/go/`: Go SDK reference client
- `sdk/python/`, `sdk/typescript/`: SDK scaffolds
- `openapi/`: Core/Admin/Pro/Cloud API specs
- `proto/mig/v0_1/`: protobuf and gRPC definitions

## Build and Test

Run unit tests:

```bash
go test ./...
```

Run HTTP conformance smoke checks against a running gateway:

```bash
go run ./conformance/harness/cmd/mig-conformance -base-url http://localhost:8080 -tenant acme
```

Run gRPC smoke checks:

```bash
go run ./conformance/harness/cmd/mig-grpc-smoke -addr localhost:9090 -tenant acme
```

Run NATS smoke checks:

```bash
go run ./conformance/harness/cmd/mig-nats-smoke -url nats://localhost:4222 -tenant acme
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Apache-2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
