# MIG Demo Recording Guide

This guide gives you a repeatable 2-4 minute demo flow for MIG Core.

Use this together with [`scripts/demo.sh`](../scripts/demo.sh).

## 1) Pre-Flight

- Enable Do Not Disturb
- Increase terminal font size
- Keep three panes visible:
  - Pane A: `migd` server logs
  - Pane B: SSE stream
  - Pane C: demo operator commands
- Open browser tab: [http://localhost:8080/ui](http://localhost:8080/ui)

## 2) Start Runtime

From repo root:

```bash
go run ./core/cmd/migd
```

If you want JWT mode:

```bash
MIGD_AUTH_MODE=jwt \
MIGD_JWT_HS256_SECRET=supersecret \
go run ./core/cmd/migd
```

Then export token for the demo helper:

```bash
export MIG_DEMO_TOKEN='<your-jwt>'
```

## 3) Quick Script Commands

```bash
./scripts/demo.sh doctor
./scripts/demo.sh hello
./scripts/demo.sh discover
./scripts/demo.sh invoke "demo narration"
./scripts/demo.sh publish done
./scripts/demo.sh connections
./scripts/demo.sh talk-track
```

Open long-lived SSE stream in a separate pane:

```bash
./scripts/demo.sh sse
```

Run full non-interactive sequence:

```bash
./scripts/demo.sh run
```

## 4) Recommended Recording Sequence

1. Start recording your screen (QuickTime: `File -> New Screen Recording`).
2. Show `/ui` loaded and mention it is built into OSS Core.
3. In Pane C run:
   - `./scripts/demo.sh doctor`
   - `./scripts/demo.sh hello`
   - `./scripts/demo.sh discover`
   - `./scripts/demo.sh invoke "recording sample"`
4. In Pane B run `./scripts/demo.sh sse` and keep it open.
5. Back in Pane C run:
   - `./scripts/demo.sh publish done`
   - `./scripts/demo.sh connections`
6. End on `/ui` showing active connections and counters updated.

## 5) 2-Minute Talk Track

Generate it directly:

```bash
./scripts/demo.sh talk-track
```

Core message to land:

- MIG Core is protocol-compliant and runnable as a standalone gateway.
- You can inspect live activity through `/admin/v0.1/connections` and `/ui`.
- The same runtime supports HTTP now, with optional gRPC and NATS bindings.

## 6) Customization

Use env vars to target different environments:

```bash
export MIG_DEMO_BASE_URL='http://localhost:8080'
export MIG_DEMO_TENANT='acme'
export MIG_DEMO_TOPIC='observatory.inference.completed'
export MIG_DEMO_CAPABILITY='observatory.models.infer'
```

## 7) Troubleshooting

- `curl: (7) Failed to connect`: `migd` is not running, wrong `MIG_DEMO_BASE_URL`, or wrong port.
- `401/403`: token or tenant mismatch in JWT mode.
- No SSE events: ensure topic in `publish` matches topic in `sse`.
- Empty connections: only long-lived streams appear; run `./scripts/demo.sh sse` first.
