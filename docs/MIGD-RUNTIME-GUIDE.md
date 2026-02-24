# MIGD Runtime Guide

This guide covers runtime-focused MIG v0.1 deployment options for `migd`.

For first-time setup, use [QUICKSTART.md](QUICKSTART.md).  
For full operator documentation, use [USER-GUIDE.md](USER-GUIDE.md).

## OSS Console

- `GET /ui`: browser dashboard for runtime visibility and quick invoke testing.
- `GET /admin/v0.1/connections`: machine-readable active connection snapshot.

## Security Modes

### Open mode (default)

```bash
MIGD_AUTH_MODE=none go run ./core/cmd/migd
```

### JWT mode

```bash
MIGD_AUTH_MODE=jwt \
MIGD_JWT_HS256_SECRET=supersecret \
go run ./core/cmd/migd
```

In JWT mode:
- `Authorization: Bearer <token>` is required.
- Token must include `tenant_id` (or `tenant`) claim.
- Capability scope checks use `scope` or `scopes` claims.

## Observability

Enable metrics (default on):

```bash
MIGD_ENABLE_METRICS=true go run ./core/cmd/migd
```

Prometheus scrape endpoint:

- `GET /metrics`

## gRPC binding

Enable gRPC listener:

```bash
MIGD_GRPC_ADDR=:9090 go run ./core/cmd/migd
```

Services exposed:
- `Discovery` (`Hello`, `Discover`)
- `Invocation` (`Invoke`, `StreamInvoke`)
- `Events` (`Publish`, `Subscribe`)
- `Control` (`Cancel`, `Heartbeat`)

Smoke check:

```bash
go run ./conformance/harness/cmd/mig-grpc-smoke -addr localhost:9090 -tenant acme
```

## Optional durability hooks

### NATS event mirroring

```bash
MIGD_NATS_URL=nats://localhost:4222 go run ./core/cmd/migd
```

Published MIG events are mirrored to subjects:

- `mig.v0_1.<tenant>.events.<topic>`

### NATS request/reply binding

```bash
MIGD_NATS_URL=nats://localhost:4222 \\
MIGD_ENABLE_NATS_BINDING=true \\
go run ./core/cmd/migd
```

Request/reply subjects:
- `mig.v0_1.<tenant>.hello`
- `mig.v0_1.<tenant>.discover`
- `mig.v0_1.<tenant>.invoke.<capability>`
- `mig.v0_1.<tenant>.events.<topic>`
- `mig.v0_1.<tenant>.control.cancel.<message_id>`
- `mig.v0_1.<tenant>.control.heartbeat`

Smoke check:

```bash
go run ./conformance/harness/cmd/mig-nats-smoke -url nats://localhost:4222 -tenant acme
```

### Audit JSONL sink

```bash
MIGD_AUDIT_LOG_PATH=./migd-audit.jsonl go run ./core/cmd/migd
```

Each invoke audit record is appended as one JSON line.

## WebSocket stream invoke

Use endpoint:

- `GET /mig/v0.1/stream`

Frame contract:
- `kind=request` + `capability` + `payload` invokes capability.
- `kind=control` + `payload.action=cancel` sends cancellation.

Responses are emitted as `kind=response` or `kind=error` frames.
