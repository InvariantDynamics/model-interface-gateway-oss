# MIG v0.1 Conformance Checklist

Use this checklist to validate implementations against `rfc/MIG-0001.md`.

## Core Profile

- [ ] `HELLO` negotiation implemented with deterministic version selection
- [ ] `DISCOVER` implemented with auth-scoped visibility
- [ ] Unary `INVOKE` implemented
- [ ] `CANCEL` implemented for in-flight invocations
- [ ] Standard MIG error model (`code`, `message`, `retryable`, `details`)
- [ ] `MessageHeader` fields present on all operations

## Streaming Profile

- [ ] `StreamInvoke` implemented
- [ ] `CANCEL` terminates in-flight stream work
- [ ] Backpressure signals are surfaced
- [ ] Deadlines enforced for stream operations

## Evented Profile

- [ ] `PUBLISH` implemented
- [ ] `SUBSCRIBE` implemented
- [ ] Event ordering documented
- [ ] Replay behavior documented and tested

## Full Profile

- [ ] Trace propagation supported (`traceparent`)
- [ ] Metrics by capability and error code
- [ ] Audit log includes actor, tenant, capability, outcome, timestamp
- [ ] Heartbeat/liveness behavior documented

## Binding-Specific Checks

### gRPC

- [ ] `proto/mig/v0_1/mig.proto` compiles
- [ ] Services map one-to-one to required MIG operations

### NATS

- [ ] Subject conventions use `mig.v0_1.<tenant>...`
- [ ] Request/reply invocation tested
- [ ] Durable consumer and replay behavior validated (JetStream)

### HTTP

- [ ] All required routes implemented under `/mig/v0.1/*`
- [ ] SSE endpoint returns event envelopes
- [ ] WebSocket stream endpoint behavior documented

## MCP Interop Checks

- [ ] `initialize` <-> `HELLO` mapping works
- [ ] `tools/list` <-> `DISCOVER` mapping works
- [ ] `tools/call` <-> `INVOKE` mapping works
- [ ] Error mapping follows `docs/interop/MCP-MIG-MAPPING.md`
- [ ] Correlation IDs preserved end-to-end
- [ ] Cancel propagation from MCP to MIG is verified
