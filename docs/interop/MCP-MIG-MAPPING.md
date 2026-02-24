# MCP <-> MIG Interoperability Mapping (v0.1)

- Status: Draft
- Last Updated: 2026-02-24
- Depends On: `rfc/MIG-0001.md`

## 1. Scope

This document defines a practical interoperability contract between:

- MCP clients/servers (JSON-RPC based)
- MIG providers/consumers (transport-neutral core protocol)

It covers discovery, invocation, streaming/progress, events, cancellation, errors, and context propagation.

## 2. Adapter Modes

### 2.1 MIG Provider Exposed as MCP Server

An adapter accepts MCP JSON-RPC requests and translates them into MIG core operations.

### 2.2 MCP Server Exposed as MIG Provider

An adapter accepts MIG operations and calls MCP methods on an upstream MCP server.

Both directions MAY be implemented by one bidirectional gateway.

## 3. Core Mapping Principles

- MIG semantics are canonical for capability identity, error model, and telemetry.
- MCP method names are mapped to MIG operations through deterministic rules.
- Adapter behavior MUST preserve auth context, tenant context, and trace context.
- Adapter SHOULD preserve streaming semantics where the source protocol supports it.

## 4. Handshake and Session Mapping

| MCP | MIG | Notes |
| --- | --- | --- |
| `initialize` request | `HELLO` | `supported_versions` and requested features derived from MCP client capabilities |
| `initialize` response | `HELLO` response (+ optional `DISCOVER`) | MCP server capabilities are synthesized from MIG capabilities |
| `notifications/initialized` | Session activation | MAY trigger warm discovery cache and heartbeat scheduling |
| n/a | `HEARTBEAT` | Exposed internally; optional MCP-level keepalive notification |

### 4.1 Version Negotiation

- Adapter MUST reject incompatible major versions.
- Adapter SHOULD advertise supported MIG versions in MCP initialization metadata.

## 5. Capability Discovery Mapping

### 5.1 Tools

| MCP Method | MIG Operation | Mapping |
| --- | --- | --- |
| `tools/list` | `DISCOVER` | Filter MIG capabilities where adapter surface is `tool` |
| Tool schema | Capability schema URIs | Adapter resolves/embeds schema into MCP tool input schema |

### 5.2 Resources

| MCP Method | MIG Operation | Mapping |
| --- | --- | --- |
| `resources/list` | `DISCOVER` | Filter surface `resource` |
| `resources/read` | `INVOKE` | Invoke mapped read capability with `uri` payload |

### 5.3 Prompts

| MCP Method | MIG Operation | Mapping |
| --- | --- | --- |
| `prompts/list` | `DISCOVER` | Filter surface `prompt` |
| `prompts/get` | `INVOKE` | Invoke mapped prompt capability with arguments |

## 6. Invocation Mapping

| MCP Method | MIG Operation | Notes |
| --- | --- | --- |
| `tools/call` | `INVOKE` | Tool name maps to capability ID via adapter manifest |
| JSON-RPC request id | `message_id` | Adapter SHOULD keep reversible correlation map |
| Tool arguments | `payload` | Pass-through object |
| JSON-RPC cancel semantics | `CANCEL` | Cancellation target is correlated MIG message ID |

### 6.1 Streaming and Progress

MCP may communicate progress and partial results through notifications.

- MIG stream frames of type `response` with `end_stream=false` map to MCP progress notifications.
- Terminal MIG stream frame maps to final `tools/call` result.
- MIG stream errors map to JSON-RPC error response.

## 7. Events Mapping

| Source | Target | Mapping |
| --- | --- | --- |
| MIG `SUBSCRIBE` event stream | MCP notifications | Adapter emits namespaced notification, e.g. `notifications/mig/event` |
| MCP event-like notifications | MIG `PUBLISH` | Optional; only when configured |

### 7.1 Topic Naming

MIG topics SHOULD remain namespaced (`domain.system.event`). MCP notifications SHOULD include full topic in payload to avoid collisions.

## 8. Error Translation

### 8.1 MIG -> MCP (JSON-RPC)

| MIG Error Code | JSON-RPC Code | Notes |
| --- | --- | --- |
| `MIG_INVALID_REQUEST` | `-32600` | Invalid request |
| `MIG_NOT_FOUND` | `-32601` | Method/capability not found |
| `MIG_UNAUTHORIZED` | `-32001` | Server-defined auth error |
| `MIG_FORBIDDEN` | `-32003` | Permission denied |
| `MIG_TIMEOUT` | `-32008` | Timeout |
| `MIG_RATE_LIMITED` | `-32009` | Throttled |
| `MIG_BACKPRESSURE` | `-32010` | Capacity pressure |
| `MIG_UNAVAILABLE` | `-32011` | Temporary unavailability |
| `MIG_INTERNAL` | `-32603` | Internal error |
| `MIG_VERSION_MISMATCH` | `-32012` | Protocol mismatch |
| `MIG_UNSUPPORTED_CAPABILITY` | `-32601` | Unsupported capability |

### 8.2 MCP -> MIG

Adapters MUST preserve JSON-RPC `error.code` and `error.data` in MIG `details`.

Recommended mapping:

- `-32600` -> `MIG_INVALID_REQUEST`
- `-32601` -> `MIG_NOT_FOUND`
- `-32602` -> `MIG_INVALID_REQUEST`
- `-32603` -> `MIG_INTERNAL`
- `-32000` to `-32099` -> `MIG_UNAVAILABLE` unless a stronger mapping exists

## 9. Context and Metadata Propagation

### 9.1 Required Context

Adapter MUST carry:

- `tenant_id`
- `session_id` (if available)
- `traceparent` (if available)
- authenticated subject identity

### 9.2 Recommended Metadata Keys

- `meta.mcp.client_name`
- `meta.mcp.client_version`
- `meta.mcp.request_id`
- `meta.mcp.method`
- `meta.mig.binding`

## 10. Security Mapping

- MCP transport authentication context MUST be mapped to MIG auth principal.
- MIG auth scopes MUST be checked before exposing a capability via MCP discovery.
- Adapter MUST NOT leak capabilities across tenants.

## 11. Adapter Manifest

Adapters SHOULD use an explicit manifest to avoid implicit or brittle name translation.

Example:

```yaml
version: 0.1
mappings:
  tools:
    - mcp_name: observatory_infer
      mig_capability: observatory.models.infer
      mode: unary
  resources:
    - mcp_uri_pattern: observatory://artifact/{id}
      mig_capability: observatory.artifacts.read
      mode: unary
  prompts:
    - mcp_name: summarize_incident
      mig_capability: observatory.prompts.incident_summary
      mode: unary
policy:
  allow_streaming_tools: true
  emit_event_notifications: true
```

Manifest schema v0.2 MAY additionally include:

- `tools[*].policy_tags`: policy classification hints for adapter-side checks.
- `tools[*].billing_tier`: product meter grouping hint (for example `core`, `pro`).
- top-level `billing.meter` and `billing.usage_scope` fields.

## 12. Normative Adapter Behavior

An interoperable adapter implementation MUST:

- Implement `initialize`, `tools/list`, and `tools/call` mapping.
- Provide deterministic capability name mapping.
- Preserve correlation IDs across translations.
- Preserve retryability semantics through error mapping metadata.
- Support `CANCEL` mapping for in-flight invocations.

An implementation SHOULD:

- Support resources and prompts mapping.
- Support stream/progress translation.
- Support event bridging.

## 13. Minimal Compliance Matrix

| Feature | Required for Interop v0.1 |
| --- | --- |
| Handshake (`initialize` <-> `HELLO`) | Yes |
| Tools discovery/invoke mapping | Yes |
| Error translation | Yes |
| Correlation ID preservation | Yes |
| Cancel mapping | Yes |
| Resources mapping | Recommended |
| Prompts mapping | Recommended |
| Event bridge | Recommended |
| Streaming bridge | Recommended |

## 14. Migration Strategy

1. Deploy adapter in tool-only mode (`tools/list`, `tools/call`).
2. Add resource/prompt mappings through manifest updates.
3. Enable streaming and event bridge for high-volume workflows.
4. Move clients from MCP-native endpoints to MIG-native bindings over time.

## 15. Open Issues for v0.2

- Standard metadata schema for MCP surface typing in MIG descriptors.
- Shared replay cursor format for event bridges.
- Standard cancellation notification format for MCP streams.
