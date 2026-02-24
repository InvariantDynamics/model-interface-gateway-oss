# MIG Roadmap

## v0.1 (Current)

- Core protocol operations (`HELLO`, `DISCOVER`, `INVOKE`, `PUBLISH`, `SUBSCRIBE`, `CANCEL`, `HEARTBEAT`)
- Binding profiles for gRPC, NATS, HTTP
- MCP interoperability mapping and adapter contract
- Conformance checklist and executable harness
- Standalone `migd` runtime with Admin, Pro, and Cloud API scaffolding

## v0.2 (Planned)

- Standard capability metadata extension model
- Replay cursor standardization across bindings
- Compression/chunking negotiation in `HELLO`
- Stream lifecycle signaling hardening
- Adapter manifest schema v0.2 adoption (`policy_tags`, `billing_tier`)

## v0.3 (Planned)

- Capability lifecycle events (deprecations and sunset windows)
- Signed descriptor bundles
- Formal compatibility certification suite
- Governance automation for MIG-EP proposals

## Product Rollout

1. OSS Core + Pro first (self-hosted adoption + design partners)
2. Cloud control plane beta on AWS
3. Cloud GA with multi-region reliability and compliance hardening
