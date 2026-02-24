# MIG/NATS Binding v0.1

- Status: Draft
- Last Updated: 2026-02-24
- Depends On: `rfc/MIG-0001.md`

## 1. Scope

This profile defines how MIG v0.1 core operations are projected onto NATS.

## 2. Transport Requirements

- NATS core is used for request/reply and low-latency pub/sub.
- JetStream SHOULD be used for durability and replay.
- TLS and authenticated NATS connections are REQUIRED.

## 3. Subject Conventions

- Discover: `mig.v0_1.<tenant>.discover`
- Invoke: `mig.v0_1.<tenant>.invoke.<capability>`
- Publish: `mig.v0_1.<tenant>.events.<topic>`
- Subscribe: `mig.v0_1.<tenant>.events.<topic>`
- Cancel: `mig.v0_1.<tenant>.control.cancel.<message_id>`
- Heartbeat: `mig.v0_1.<tenant>.control.heartbeat`

`<capability>` and `<topic>` path segments SHOULD use dot-safe names.

## 4. Envelope Encoding

Payloads are UTF-8 JSON with MIG v0.1 header + operation payload.

Example request payload:

```json
{
  "header": {
    "mig_version": "0.1",
    "message_id": "c8d45589-0baf-4a6e-a45a-8c6225de2f33",
    "timestamp": "2026-02-24T10:00:00Z",
    "tenant_id": "acme",
    "session_id": "s-123",
    "traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-00",
    "deadline_ms": 30000
  },
  "capability": "observatory.models.infer",
  "payload": {
    "input": "..."
  }
}
```

## 5. Header Mapping

MIG header fields SHOULD be duplicated into NATS headers for routing and observability:

- `mig_version`
- `message_id`
- `tenant_id`
- `session_id`
- `traceparent`
- `idempotency_key`

## 6. Request/Reply Mapping

- `HELLO`, `DISCOVER`, `INVOKE`, `CANCEL`, `HEARTBEAT` use request/reply semantics.
- Reply payload MUST include either success envelope or MIG error envelope.

## 7. Streaming Mapping

For stream invocation:

- A stream control message starts the stream and returns `stream_id`.
- Frames publish on `mig.v0_1.<tenant>.stream.<stream_id>`.
- Final frame sets `end_stream=true`.

## 8. Event Replay

When JetStream is enabled:

- Subscribers MAY provide `resume_cursor` mapped to sequence or stream position.
- Providers SHOULD expose replay support via capability QoS metadata.

## 9. Error Handling

Errors MUST follow MIG error schema with stable codes from `MIG-0001`.

## 10. Recommended JetStream Settings

- `max_ack_pending`: bounded per consumer
- explicit ack mode
- retention according to tenant policy
- dead-letter subject for poison messages
